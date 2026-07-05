package bake

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

// The two external rendering steps are interfaces so the orchestrator can be unit
// tested with fakes; the real implementations SHELL OUT (I8: core never renders
// strokes — the pixels come from poppler for rasters and web/bake for overlays).

// Rasterizer turns a PDF into one PNG per page (page rasters, I12).
type Rasterizer interface {
	Rasterize(ctx context.Context, pdf []byte) ([][]byte, error)
}

// renderedOverlay is one transparent per-layer overlay the worker produced.
type renderedOverlay struct {
	Page        int
	LayerID     string
	Order       int32
	Mandatory   bool
	RoleTag     string
	ContentHash string
	PNG         []byte
}

// OverlayRenderer renders the transparent per-layer annotation overlays for a
// request via @troubastack/ink (the ONE renderer, I8) — never in Go.
type OverlayRenderer interface {
	Render(ctx context.Context, req cliRequest) ([]renderedOverlay, error)
}

// ---- poppler pdftoppm rasterizer ----------------------------------------

type popplerRasterizer struct {
	bin string // pdftoppm binary (TROUBA_PDFTOPPM)
	dpi int
}

func (r popplerRasterizer) Rasterize(ctx context.Context, pdf []byte) ([][]byte, error) {
	dir, err := os.MkdirTemp("", "trouba-raster-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		return nil, err
	}
	prefix := filepath.Join(dir, "page")
	// pdftoppm -png -r <dpi> in.pdf page  →  page-1.png, page-2.png, … (zero-padded).
	cmd := exec.CommandContext(ctx, r.bin, "-png", "-r", strconv.Itoa(r.dpi), in, prefix)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// A missing binary or a bad PDF fails the BAKE with a clear message — never
		// the server (the caller maps this to a 5xx/4xx, the process keeps running).
		return nil, fmt.Errorf("pdftoppm (%s): %w: %s", r.bin, err, strings.TrimSpace(stderr.String()))
	}

	entries, err := filepath.Glob(prefix + "-*.png")
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("pdftoppm produced no pages")
	}
	// Sort by the numeric page suffix (page-2 before page-10, robust to padding).
	sort.Slice(entries, func(i, j int) bool { return pageNum(entries[i]) < pageNum(entries[j]) })
	pages := make([][]byte, 0, len(entries))
	for _, e := range entries {
		b, rerr := os.ReadFile(e)
		if rerr != nil {
			return nil, rerr
		}
		pages = append(pages, b)
	}
	return pages, nil
}

// pageNum extracts N from a "…-N.png" pdftoppm output name (0 if unparseable).
func pageNum(path string) int {
	base := strings.TrimSuffix(filepath.Base(path), ".png")
	if i := strings.LastIndex(base, "-"); i >= 0 {
		if n, err := strconv.Atoi(base[i+1:]); err == nil {
			return n
		}
	}
	return 0
}

// ---- web/bake (Node) overlay renderer -----------------------------------

type nodeOverlayRenderer struct {
	node string // node binary (TROUBA_NODE)
	cli  string // path to web/bake/dist/cli.js (TROUBA_BAKE_CLI)
}

// The manifest the troubabake CLI writes (web/bake/src/cli.ts).
type cliManifest struct {
	Pages []struct {
		Index    int `json:"index"`
		Overlays []struct {
			LayerID     string `json:"layerId"`
			File        string `json:"file"`
			Order       int32  `json:"order"`
			Mandatory   bool   `json:"mandatory"`
			RoleTag     string `json:"roleTag"`
			ContentHash string `json:"contentHash"`
		} `json:"overlays"`
	} `json:"pages"`
}

func (r nodeOverlayRenderer) Render(ctx context.Context, req cliRequest) ([]renderedOverlay, error) {
	dir, err := os.MkdirTemp("", "trouba-overlay-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)

	reqPath := filepath.Join(dir, "request.json")
	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(reqPath, reqBytes, 0o600); err != nil {
		return nil, err
	}
	outDir := filepath.Join(dir, "out")
	// node dist/cli.js --in request.json --out <outDir>  (stdout stays clean; logs→stderr).
	cmd := exec.CommandContext(ctx, r.node, r.cli, "--in", reqPath, "--out", outDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("web/bake worker (%s %s): %w: %s", r.node, r.cli, err, strings.TrimSpace(stderr.String()))
	}

	manifestBytes, err := os.ReadFile(filepath.Join(outDir, "index.json"))
	if err != nil {
		return nil, fmt.Errorf("web/bake worker wrote no index.json: %w", err)
	}
	var man cliManifest
	if err := json.Unmarshal(manifestBytes, &man); err != nil {
		return nil, fmt.Errorf("web/bake index.json malformed: %w", err)
	}
	var out []renderedOverlay
	for _, page := range man.Pages {
		for _, ov := range page.Overlays {
			png, rerr := os.ReadFile(filepath.Join(outDir, ov.File))
			if rerr != nil {
				return nil, fmt.Errorf("web/bake overlay %s missing: %w", ov.File, rerr)
			}
			out = append(out, renderedOverlay{
				Page:        page.Index,
				LayerID:     ov.LayerID,
				Order:       ov.Order,
				Mandatory:   ov.Mandatory,
				RoleTag:     ov.RoleTag,
				ContentHash: ov.ContentHash,
				PNG:         png,
			})
		}
	}
	return out, nil
}

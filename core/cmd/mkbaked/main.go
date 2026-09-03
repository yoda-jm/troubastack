// Command mkbaked regenerates a "current baker" bundle fixture by running the REAL web/bake pipeline
// (poppler + the Node worker) in-process on a synthetic concert, then unpacking it into -out.
//
// It is A61's companion to mkbundle, and deliberately different: mkbundle is a synthetic, deterministic
// generator (`make fixtures`); mkbaked runs the ACTUAL baker, so its output tracks what a real bake
// emits today — the compatibility arm that catches bake-format drift. It writes ONLY to -out (a NEW
// fixture dir) and NEVER touches the frozen `fixtures/baked/`.
//
// It skips cleanly — a message and exit 0, nothing written — when the bake toolchain is absent (node,
// pdftoppm, or the web/bake cli.js), which is the case a contributor without the toolchain hits; a
// half-written fixture would be worse than none. The CLI path is resolved exactly as the server
// resolves it (config.ResolveBakeCLI, T128), honouring TROUBA_BAKE_CLI/TROUBA_NODE/TROUBA_PDFTOPPM, so
// there is no second hard-coded path (a second hard-coded path is how the gig server broke).
//
// A real bake is NOT byte-reproducible (UUIDs, bakedAt, raster), so a refresh is a deliberate non-empty
// diff — stated next to the target in fixtures/README.md. Content is entirely synthetic; no band data
// ever goes into a committed fixture.
//
//	cd core && go run ./cmd/mkbaked -out ../app/shared/src/commonTest/resources/fixtures/baked-current
package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-pdf/fpdf"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/config"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

func main() {
	out := flag.String("out", "", "unpacked fixture dir to (over)write with a freshly-baked bundle")
	flag.Parse()
	if *out == "" {
		log.Fatal("mkbaked: -out <dir> is required")
	}

	// Resolve the bake toolchain exactly as the server does (T128), honouring the env overrides so the
	// skip path is testable with TROUBA_BAKE_CLI=/nonexistent. Never hard-code a second CLI path.
	cwd, _ := os.Getwd()
	exe, _ := os.Executable()
	configured := os.Getenv("TROUBA_BAKE_CLI")
	if configured == "" {
		configured = config.DefaultBakeCLI
	}
	cli := config.ResolveBakeCLI(configured, filepath.Dir(exe), cwd, fileExists)
	node := envOr("TROUBA_NODE", "node")
	ppm := envOr("TROUBA_PDFTOPPM", "pdftoppm")

	if missing := missingTool(node, ppm, cli); missing != "" {
		fmt.Printf("mkbaked: %s — skipping; %s left untouched.\n"+
			"  Install poppler-utils + Node and build web/bake, or set TROUBA_BAKE_CLI/TROUBA_NODE/TROUBA_PDFTOPPM.\n",
			missing, *out)
		return // exit 0: this is the expected no-toolchain path, not a failure
	}

	// A synthetic concert: one song with a REAL one-page PDF (pdftoppm must rasterise it — the unit
	// tests' "%PDF-1.4 fixture" stub cannot be), plus one drawn annotation layer so the real baker emits
	// an overlay blob (a plain text-chart bakes zero overlays — the very structural fact baked/ guards).
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u := must(svc.Register("templateadmin", "Template Admin", "password123", ""))
	band := must(svc.CreateBand(u, "Template Band"))
	song := must(svc.CreateSong(u, band.ID, "Sound Check", ""))
	if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", onePagePDF()); err != nil {
		log.Fatalf("mkbaked: upload pdf: %v", err)
	}
	apply(eng, song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", Name: "Notes", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
		AuthorID: u.ID,
	})
	apply(eng, song.ID, domain.Mutation{
		Kind: domain.KindCreate, UUID: "o1", AuthorID: u.ID,
		Object: &domain.Object{
			UUID: "o1", LayerID: "L1", Type: domain.TypeRect, Page: 0, Version: 1,
			Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
			Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
		},
	})
	sl := must(svc.CreateSetlist(u, band.ID, "Template Concert", "", "", ""))
	if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
		log.Fatalf("mkbaked: add setlist item: %v", err)
	}

	// Bake through the REAL pipeline into a temp dir, then copy the unpacked bundle to -out.
	bakesDir, err := os.MkdirTemp("", "mkbaked-*")
	if err != nil {
		log.Fatalf("mkbaked: tempdir: %v", err)
	}
	defer os.RemoveAll(bakesDir)
	b := bake.New(svc, eng, bake.Config{BakesDir: bakesDir, Pdftoppm: ppm, Node: node, BakeCLI: cli})
	bundle, _, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil, "")
	if err != nil {
		log.Fatalf("mkbaked: bake failed: %v", err)
	}
	src, err := bundleDir(filepath.Join(bakesDir, bundle.ConcertID))
	if err != nil {
		log.Fatalf("mkbaked: locate bundle dir: %v", err)
	}
	if err := replaceDir(*out, src); err != nil {
		log.Fatalf("mkbaked: write %s: %v", *out, err)
	}
	fmt.Printf("mkbaked: wrote %s from a real bake (%d song, rev %d). NOT byte-reproducible — a refresh is a deliberate diff.\n",
		*out, len(bundle.Songs), bundle.ConcertRev)
}

// missingTool returns a human message for the first missing tool, or "" if all are present. node and
// pdftoppm are resolved on PATH; the CLI is checked as a regular file (its @napi-rs/canvas binding is
// exercised at bake time, and a bake failure there is a hard error, not a silent skip).
func missingTool(node, ppm, cli string) string {
	if _, err := exec.LookPath(node); err != nil {
		return "node not found (" + node + ")"
	}
	if _, err := exec.LookPath(ppm); err != nil {
		return "pdftoppm not found (" + ppm + ")"
	}
	if fi, err := os.Stat(cli); err != nil || fi.IsDir() {
		return "web/bake worker not found (" + cli + ")"
	}
	return ""
}

// bundleDir finds the published <rev>/ directory under concertDir (the one that is not a .tmp staging
// dir), rather than guessing the rev-number naming.
func bundleDir(concertDir string) (string, error) {
	entries, err := os.ReadDir(concertDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() && !strings.HasSuffix(e.Name(), ".tmp") {
			return filepath.Join(concertDir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no published bundle dir under %s", concertDir)
}

func onePagePDF() []byte {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 24)
	pdf.Cell(0, 20, "Sound Check")
	pdf.Ln(24)
	pdf.SetFont("Helvetica", "", 12)
	pdf.MultiCell(0, 8, "Synthetic chart for the current-baker fixture (A61). No band data.", "", "L", false)
	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		log.Fatalf("mkbaked: build pdf: %v", err)
	}
	return buf.Bytes()
}

func replaceDir(dst, src string) error {
	if err := os.RemoveAll(dst); err != nil {
		return err
	}
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(p, target)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	w, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer w.Close()
	_, err = io.Copy(w, in)
	return err
}

func apply(eng *engine.Engine, songID string, m domain.Mutation) {
	if _, err := eng.Apply(songID, m); err != nil {
		log.Fatalf("mkbaked: apply mutation: %v", err)
	}
}

func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func must[T any](v T, err error) T {
	if err != nil {
		log.Fatalf("mkbaked: %v", err)
	}
	return v
}

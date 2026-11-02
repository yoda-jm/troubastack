package bake

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
)

// T199 — the page-aspect source for the deskew view transform. Rotating a crop window about its centre is done
// in PHYSICAL units, so it needs each page's true aspect (W/H). That aspect must be the SAME number at bake
// time and at crop-save dry-run time, or the dryRun's hidden-marks list and what the bake actually drops would
// disagree. The one honest shared source is the source PDF's MediaBox (the box pdftoppm renders), read from
// pdfinfo. It is independent of the crop and of the raster cache, and it is exact (points, no px rounding). The
// cropped raster's own dimensions are NOT usable: their ratio only approximates the page's (it rounds) and it
// moves with the crop.

// pageAsper reads every page's rendered aspect from a PDF. popplerRasterizer implements it with pdfinfo; a test
// can inject a fake via bake.Config.Aspecter.
type pageAsper interface {
	pageAspects(ctx context.Context, pdf []byte) ([]float64, error)
}

// FixedAspecter is a TEST HELPER (exported only so cross-package tests — e.g. the httpapi crop-deskew
// end-to-end — can inject it through bake.Config.Aspecter, since pageAsper is unexported). It returns a fixed
// per-page aspect regardless of the PDF, letting a test drive the deskew transform with a DISTINCTIVE aspect
// and prove both the bake drop and the crop dry-run take their aspect from this one source. Not used in
// production (Config.Aspecter is nil there, so the real pdfinfo-backed popplerRasterizer is used).
type FixedAspecter struct{ Aspects []float64 }

func (f FixedAspecter) pageAspects(context.Context, []byte) ([]float64, error) {
	if len(f.Aspects) == 0 {
		return nil, fmt.Errorf("FixedAspecter: no aspects configured")
	}
	return f.Aspects, nil
}

var (
	// pdfinfo -box prints, per page, a MediaBox line (bare on a single page, "Page N MediaBox:" in a range)
	// and a rotation line ("Page rot:" single, "Page N rot:" in a range). An optional page number is captured
	// so both shapes parse; the box is the MEDIABOX, never "Page size" (which is the CropBox pdftoppm ignores).
	mediaBoxRe = regexp.MustCompile(`(?m)^(?:Page\s+(\d+)\s+)?MediaBox:\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)\s+(-?[\d.]+)`)
	pageRotRe  = regexp.MustCompile(`(?m)^Page\s+(?:(\d+)\s+)?rot:\s+(-?\d+)`)
)

// pageAspects returns the rendered aspect (W/H) of every page, 0-based by page index. W,H come from the page's
// MediaBox (x2−x1, y2−y1) and are SWAPPED for a /Rotate of 90 or 270, matching what pdftoppm renders. One
// pdfinfo exec reads the whole file. It returns an error — never a wrong silent default — if pdfinfo is not
// configured or the output can't be parsed; the caller surfaces that (B1: no silent degrade).
func (pr popplerRasterizer) pageAspects(ctx context.Context, pdf []byte) ([]float64, error) {
	if pr.infoBin == "" {
		return nil, fmt.Errorf("deskew needs the pdfinfo binary, which is not configured")
	}
	dir, err := os.MkdirTemp("", "trouba-aspect-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	in := filepath.Join(dir, "in.pdf")
	if err := os.WriteFile(in, pdf, 0o600); err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, pr.infoBin, "-f", "1", "-l", "100000", "-box", in)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("pdfinfo -box: %w", err)
	}
	return parsePageAspects(out.String())
}

// parsePageAspects turns a `pdfinfo -box` listing into one aspect per page. An absent page number (the
// single-page shape) is page 1. Pure, so it is unit-tested directly.
func parsePageAspects(listing string) ([]float64, error) {
	type box struct{ w, h float64 }
	boxes := map[int]box{}
	for i, m := range mediaBoxRe.FindAllStringSubmatch(listing, -1) {
		page := i + 1
		if m[1] != "" {
			page, _ = strconv.Atoi(m[1])
		}
		x1, _ := strconv.ParseFloat(m[2], 64)
		y1, _ := strconv.ParseFloat(m[3], 64)
		x2, _ := strconv.ParseFloat(m[4], 64)
		y2, _ := strconv.ParseFloat(m[5], 64)
		boxes[page] = box{w: x2 - x1, h: y2 - y1}
	}
	if len(boxes) == 0 {
		return nil, fmt.Errorf("pdfinfo -box: no MediaBox lines found")
	}
	rots := map[int]int{}
	for i, m := range pageRotRe.FindAllStringSubmatch(listing, -1) {
		page := i + 1
		if m[1] != "" {
			page, _ = strconv.Atoi(m[1])
		}
		r, _ := strconv.Atoi(m[2])
		rots[page] = ((r % 360) + 360) % 360
	}
	maxPage := 0
	for p := range boxes {
		if p > maxPage {
			maxPage = p
		}
	}
	out := make([]float64, maxPage)
	for p := 1; p <= maxPage; p++ {
		b, ok := boxes[p]
		if !ok {
			return nil, fmt.Errorf("pdfinfo -box: no MediaBox for page %d", p)
		}
		w, h := b.w, b.h
		if rots[p] == 90 || rots[p] == 270 {
			w, h = h, w
		}
		if w <= 0 || h <= 0 {
			return nil, fmt.Errorf("pdfinfo -box: page %d has a non-positive MediaBox %gx%g", p, w, h)
		}
		out[p-1] = w / h
	}
	return out, nil
}

// aspects returns the per-page aspects for a PDF, memoized by content hash so the one pdfinfo read serves the
// bake drop, the spill warning and repeated bakes of the same file. A Baker with no aspecter (a bare-literal
// test baker) returns an error rather than a guess.
func (b *Baker) aspects(ctx context.Context, pdf []byte) ([]float64, error) {
	if b.aspecter == nil {
		return nil, fmt.Errorf("deskew page-aspect source not configured")
	}
	key := Sha256Hex(pdf)
	b.aspectMu.Lock()
	if b.aspectMemo == nil {
		b.aspectMemo = map[string][]float64{}
	} else if a, ok := b.aspectMemo[key]; ok {
		b.aspectMu.Unlock()
		return a, nil
	}
	b.aspectMu.Unlock()
	a, err := b.aspecter.pageAspects(ctx, pdf)
	if err != nil {
		return nil, err
	}
	b.aspectMu.Lock()
	b.aspectMemo[key] = a
	b.aspectMu.Unlock()
	return a, nil
}

// PageAspect returns one page's rendered aspect (W/H) for the crop-save dry run; page is 0-based. An error
// (pdfinfo unavailable/unparseable, or page out of range) is surfaced to Studio, never a silent default (B1).
func (b *Baker) PageAspect(ctx context.Context, pdf []byte, page int) (float64, error) {
	a, err := b.aspects(ctx, pdf)
	if err != nil {
		return 0, err
	}
	if page < 0 || page >= len(a) {
		return 0, fmt.Errorf("deskew: page %d is out of range (%d page(s))", page, len(a))
	}
	return a[page], nil
}

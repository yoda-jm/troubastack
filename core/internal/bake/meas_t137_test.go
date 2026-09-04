package bake

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestMeasureT137 bakes a REAL differing-selection band (Canon parts, real poppler @150 DPI) and reports
// the .tstage delta: today's single-file bundle vs the T137 union pool. Run: T137_MEASURE=1 go test -run
// TestMeasureT137 -v ./internal/bake/. Not a CI test (needs poppler + real PDFs; measurement only).
func TestMeasureT137(t *testing.T) {
	if os.Getenv("T137_MEASURE") == "" {
		t.Skip("set T137_MEASURE=1 to run the measurement")
	}
	charts := "../../../docs/demo-charts"
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(charts, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return b
	}

	e := newT137Env(t)
	score := e.upload(t, "canon-score.pdf", read("canon-score.pdf"))  // default reader (6 pages)
	v2 := e.upload(t, "canon-violin2.pdf", read("canon-violin2.pdf")) // 2 pages
	cello := e.upload(t, "canon-cello.pdf", read("canon-cello.pdf"))  // 1 page
	_ = score

	dir := t.TempDir()
	b := &Baker{svc: e.svc, eng: e.eng, raster: popplerRasterizer{bin: "/usr/bin/pdftoppm", dpi: 150}, overlays: fakeOverlays{png: []byte("ov")}, bakesDir: dir, now: func() int64 { return 1700000000 }}

	tstageSize := func() int64 {
		matches, _ := filepath.Glob(filepath.Join(dir, "*", "*.tstage"))
		var newest string
		var newestMod int64
		for _, m := range matches {
			fi, err := os.Stat(m)
			if err != nil {
				continue
			}
			if fi.ModTime().UnixNano() >= newestMod {
				newestMod, newest = fi.ModTime().UnixNano(), m
			}
		}
		fi, err := os.Stat(newest)
		if err != nil {
			t.Fatalf("stat tstage: %v", err)
		}
		return fi.Size()
	}

	// Baseline: nobody diverges → today's single-file bundle (score only).
	baseBundle, _, err := b.Bake(context.Background(), e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("baseline bake: %v", err)
	}
	baseSize := tstageSize()
	basePages := len(baseBundle.Songs[0].Pages)

	// Differing: two members pick different parts; the default reader (admin) stays on the score.
	e.selects(t, e.marie, v2.ID)
	e.selects(t, e.leo, cello.ID)

	divBundle, _, err := b.Bake(context.Background(), e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("differing bake: %v", err)
	}
	divSize := tstageSize()
	divPages := len(divBundle.Songs[0].Pages)
	mps := len(divBundle.Songs[0].MemberPages)

	t.Logf("T137 bundle-size measurement (Canon, real poppler @150 DPI):")
	t.Logf("  baseline (score only):      %d pages, %d bytes .tstage", basePages, baseSize)
	t.Logf("  differing (score+violin2+cello): %d pages, %d bytes .tstage, %d member_pages entries", divPages, divSize, mps)
	t.Logf("  DELTA: +%d pages, +%d bytes (%.1f%%)", divPages-basePages, divSize-baseSize, 100*float64(divSize-baseSize)/float64(baseSize))
}

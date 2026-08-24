package bake

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"path/filepath"
	"regexp"
	"strconv"
	"testing"
)

// solidPNG returns a w×h PNG filled with c (opaque) — a controllable raster/overlay.
func solidPNG(t *testing.T, w, h int, c color.Color) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, c)
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

var pageLeafRE = regexp.MustCompile(`/Type\s*/Page\b`)

// TestConcertPDF_ValidAndPageCount: the export is a parseable PDF whose leaf page
// count equals the baked page count, and re-printing the same bundle is
// byte-identical (deterministic modulo the pinned timestamps).
func TestConcertPDF_ValidAndPageCount(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	white := solidPNG(t, 40, 56, color.White)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 3, png: white},
		overlays: fakeOverlays{png: white},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
		t.Fatalf("bake: %v", err)
	}

	pdf, err := b.ConcertPDF(setlistID, "", u.ID)
	if err != nil {
		t.Fatalf("ConcertPDF: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("output is not a PDF (no %%PDF- header)")
	}
	if !bytes.Contains(pdf, []byte("%%EOF")) {
		t.Fatalf("PDF missing %%EOF trailer")
	}
	// The seed setlist has one song; the fake rasterizer gave it 3 pages.
	if got := len(pageLeafRE.FindAll(pdf, -1)); got != 3 {
		t.Fatalf("PDF leaf page count = %d, want 3 (== baked pages)", got)
	}

	pdf2, err := b.ConcertPDF(setlistID, "", u.ID)
	if err != nil {
		t.Fatalf("ConcertPDF re-run: %v", err)
	}
	if !bytes.Equal(pdf, pdf2) {
		t.Fatalf("ConcertPDF is not deterministic: two runs differ (%d vs %d bytes)", len(pdf), len(pdf2))
	}
}

// TestComposePage_OverlayCompositingAndFilter is the compositing guard, red-first:
// a page with a red personal overlay composites RED for its owner but stays WHITE
// for a different viewer (the overlay is filtered out) — proving both that
// overlays actually paint AND that LayerVisible gates them. The seed's L1 layer is
// personal, owned by u.
func TestComposePage_OverlayCompositingAndFilter(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	white := solidPNG(t, 40, 56, color.White)
	red := solidPNG(t, 40, 56, color.RGBA{R: 0xe1, G: 0x1d, B: 0x48, A: 0xff})
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: white},
		overlays: fakeOverlays{png: red}, // the one overlay (seed layer L1) is solid red
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	cb, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	rev, ok := b.latestRev(cb.ConcertID)
	if !ok {
		t.Fatal("no published rev after bake")
	}
	revDir := filepath.Join(b.bakesDir, cb.ConcertID, strconv.FormatUint(rev, 10))
	page := cb.Songs[0].Pages[0]

	isRedAt := func(img image.Image) bool {
		r, g, bl, _ := img.At(20, 28).RGBA()
		return r>>8 > 0x80 && g>>8 < 0x60 && bl>>8 < 0x80
	}

	// Owner sees the red overlay.
	mine, err := b.composePage(revDir, page, "", u.ID)
	if err != nil {
		t.Fatalf("composePage (owner): %v", err)
	}
	if !isRedAt(mine) {
		t.Fatalf("owner's composite should show the red overlay pixel")
	}

	// A different viewer does not — the personal layer is filtered, page stays white.
	other, err := b.composePage(revDir, page, "", "someone-else")
	if err != nil {
		t.Fatalf("composePage (other): %v", err)
	}
	if isRedAt(other) {
		t.Fatalf("a non-owner must NOT see the personal overlay (LayerVisible filter failed)")
	}
}

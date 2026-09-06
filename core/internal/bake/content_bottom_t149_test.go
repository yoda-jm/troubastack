package bake

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// rasterInkTo builds an OPAQUE WHITE page PNG (h tall) with black pixels filling rows [0, inkRows) — a page
// whose text ends at inkRows/h. No ink below that.
func rasterInkTo(t *testing.T, w, h, inkRows int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if y < inkRows {
				img.Set(x, y, color.NRGBA{0, 0, 0, 255}) // black glyph
			} else {
				img.Set(x, y, color.NRGBA{255, 255, 255, 255}) // white page
			}
		}
	}
	return encodePNG(t, img)
}

// overlayInkBand builds a TRANSPARENT overlay PNG (h tall) with opaque red pixels in rows [y0, y1) — a mark
// whose lowest ink is at y1/h. Everything else transparent.
func overlayInkBand(t *testing.T, w, h, y0, y1 int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h)) // zero value = fully transparent
	for y := y0; y < y1; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.NRGBA{225, 29, 72, 255}) // opaque mark
		}
	}
	return encodePNG(t, img)
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestContentBottomPermille_RasterOnly(t *testing.T) {
	// Text ends a fifth of the way down a 1000px page → ~200 permille.
	got := contentBottomPermille(rasterInkTo(t, 100, 1000, 200), nil)
	if got < 195 || got > 205 {
		t.Fatalf("raster ink to 20%% → %d permille, want ~200", got)
	}
	// A page filled to the last row → ~1000.
	if full := contentBottomPermille(rasterInkTo(t, 100, 1000, 1000), nil); full < 995 {
		t.Fatalf("full page → %d permille, want ~1000", full)
	}
	// A blank page → 0 (⇒ presenter shows the full page; nothing to trim).
	blank := contentBottomPermille(rasterInkTo(t, 100, 1000, 0), nil)
	if blank != 0 {
		t.Fatalf("blank page → %d permille, want 0", blank)
	}
}

// TestContentBottomPermille_MarkBelowText is the case that PROTECTS a musician (VLL's real data has a mark
// at Y 0.328–0.424 on a page whose text ends at 0.051): the page must stay open to the OVERLAY's extent, not
// the text's. Teeth: computing from the raster (text) alone gives ~50 and would crop the mark — assert the
// result reflects the overlay (~420) and strictly exceeds the raster-only value.
func TestContentBottomPermille_MarkBelowText(t *testing.T) {
	raster := rasterInkTo(t, 100, 1000, 51)           // text ends at 5.1%
	overlay := overlayInkBand(t, 100, 1000, 328, 424) // mark spans 32.8%–42.4%

	rasterOnly := contentBottomPermille(raster, nil)
	if rasterOnly < 45 || rasterOnly > 60 {
		t.Fatalf("raster-only → %d permille, want ~51 (the fixture)", rasterOnly)
	}
	got := contentBottomPermille(raster, [][]byte{overlay})
	if got < 415 || got > 430 {
		t.Fatalf("raster+overlay → %d permille, want ~424 (the mark's bottom)", got)
	}
	if got <= rasterOnly {
		t.Fatalf("content bottom (%d) did not extend past the text (%d) to cover the mark — the mark would be cropped", got, rasterOnly)
	}
}

func TestContentBottomPermille_Undecodable(t *testing.T) {
	// Best-effort: garbage bytes contribute nothing (0 ⇒ full page), never a panic.
	if got := contentBottomPermille([]byte("not a png"), [][]byte{[]byte("nope")}); got != 0 {
		t.Fatalf("undecodable input → %d, want 0", got)
	}
}

// TestBake_ContentBottomReachesBundle_T149 proves the per-page measurement is computed and written into the
// actual bundle.json (the app reads the file, not the Go struct) — the T143 done-when discipline.
func TestBake_ContentBottomReachesBundle_T149(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	raster := rasterInkTo(t, 100, 1000, 200) // this song's page: text ends at ~20%
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: raster},
		overlays: fakeOverlays{png: tinyPNG(t, 100, 1000)}, // transparent ⇒ contributes no ink
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
		t.Fatalf("bake: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(b.bakesDir, setlistID, "1", "bundle.json"))
	if err != nil {
		t.Fatalf("read bundle.json: %v", err)
	}
	var parsed ConcertBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("bundle.json parse: %v", err)
	}
	if len(parsed.Songs) == 0 || len(parsed.Songs[0].Pages) == 0 {
		t.Fatalf("bundle has no song pages: %+v", parsed.Songs)
	}
	got := parsed.Songs[0].Pages[0].ContentBottomPermille
	if got < 195 || got > 205 {
		t.Fatalf("bundle page contentBottomPermille = %d, want ~200 (raster ink to 20%%)", got)
	}
	if !strings.Contains(string(data), `"contentBottomPermille"`) {
		t.Errorf("bundle.json must carry the contentBottomPermille key, got:\n%s", string(data))
	}
}

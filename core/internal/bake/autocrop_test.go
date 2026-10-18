package bake

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

// A synthetic page: white background, black (ink) rectangles drawn into it. No real scan is committed — the
// spec's noise vectors are generated here so the guards can be proven red-first against a naive bbox.
func page(w, h int, rects ...image.Rectangle) []byte {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = 0xff // opaque white
	}
	for _, r := range rects {
		for y := r.Min.Y; y < r.Max.Y; y++ {
			for x := r.Min.X; x < r.Max.X; x++ {
				img.Set(x, y, color.RGBA{0, 0, 0, 0xff})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

const pw, ph = 200, 280 // a portrait test page

// The content block used across the vectors: centred, 0.3..0.7 on both axes.
var contentBlock = image.Rect(60, 84, 140, 196)

func approx(t *testing.T, got, want, tol float64, what string) {
	t.Helper()
	if math.Abs(got-want) > tol {
		t.Errorf("%s = %.4f, want ~%.4f (±%.2f)", what, got, want, tol)
	}
}

// TestAutodetect_CentredContent: a centred block yields a square crop (policy A) margined around it.
func TestAutodetect_CentredContent(t *testing.T) {
	r, ok := detectContentRect(page(pw, ph, contentBlock))
	if !ok {
		t.Fatal("centred content must yield a crop")
	}
	if math.Abs(r.w-r.h) > 1e-9 {
		t.Fatalf("policy A: crop must be a square in fraction space, got w=%.4f h=%.4f", r.w, r.h)
	}
	// content 0.3..0.7 + 0.02 margin => 0.28..0.72, side 0.44, centred.
	approx(t, r.w, 0.44, 0.02, "side")
	approx(t, r.x, 0.28, 0.02, "x")
	approx(t, r.y, 0.28, 0.02, "y")
}

// TestAutodetect_EdgeBandRejected: a black band down the left edge pollutes a NAIVE bbox (its left goes to
// x=0) but the guarded detector rejects the band and crops to the content only. Red-first: the naive box is
// asserted wrong in the same test.
func TestAutodetect_EdgeBandRejected(t *testing.T) {
	band := image.Rect(0, 0, 3, ph) // full-height band on the left edge
	raster := page(pw, ph, contentBlock, band)

	g, err := decodePNG(raster)
	if err != nil {
		t.Fatal(err)
	}
	nx0, _, _, _, nok := naiveInkBoxPx(g)
	if !nok || nx0 != 0 {
		t.Fatalf("naive bbox should be polluted by the edge band (left at x=0), got x0=%d ok=%v", nx0, nok)
	}

	r, ok := detectContentRect(raster)
	if !ok {
		t.Fatal("content beside an edge band must still yield a crop")
	}
	// The band is gone: the crop matches the centred-content crop, NOT one pulled to the left edge.
	approx(t, r.x, 0.28, 0.02, "x (band must not pull the crop to the edge)")
	approx(t, r.w, 0.44, 0.02, "side")
}

// TestAutodetect_SpeckRejected: a tiny dust speck in a far corner stretches a NAIVE bbox but is below the
// min-component area, so the guarded detector ignores it.
func TestAutodetect_SpeckRejected(t *testing.T) {
	speck := image.Rect(190, 10, 192, 12) // 2x2 px, far top-right
	raster := page(pw, ph, contentBlock, speck)

	g, _ := decodePNG(raster)
	_, ny0, nx1, _, _ := naiveInkBoxPx(g)
	if nx1 < 190 || ny0 > 12 {
		t.Fatalf("naive bbox should reach the speck corner, got x1=%d y0=%d", nx1, ny0)
	}

	r, ok := detectContentRect(raster)
	if !ok {
		t.Fatal("content with a far speck must still yield a crop")
	}
	// Speck ignored → crop matches the centred content, its right edge well left of the speck.
	approx(t, r.x+r.w, 0.72, 0.02, "right edge (speck excluded)")
	approx(t, r.x, 0.28, 0.02, "x")
}

// TestAutodetect_BlankPage: an all-white page has nothing to crop.
func TestAutodetect_BlankPage(t *testing.T) {
	if _, ok := detectContentRect(page(pw, ph)); ok {
		t.Fatal("a blank page must not yield a crop")
	}
}

// TestAutodetect_FullBleed: content covering the whole page yields no crop (> autoMaxCoverFrac).
func TestAutodetect_FullBleed(t *testing.T) {
	if _, ok := detectContentRect(page(pw, ph, image.Rect(2, 2, pw-2, ph-2))); ok {
		t.Fatal("a full-bleed page must not yield a crop")
	}
}

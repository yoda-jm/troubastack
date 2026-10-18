package bake

import (
	"image"
	"image/color"
	"testing"
)

func isDark(c color.Color) bool {
	r, g, b, _ := c.RGBA()
	return r < 0x8000 && g < 0x8000 && b < 0x8000
}

// TestComposeLayers_ScalesOverlayToRaster pins the T198 fix: an overlay whose size differs from the raster is
// scaled onto the raster, so a mark lands where it belongs — not drawn at its own (smaller) bounds. The
// overlay is half the raster's size with a black block at ITS centre; after compositing, the block must be at
// the RASTER's centre (2× out), not at the overlay's. Red before the scaling fix (the block stayed top-left).
func TestComposeLayers_ScalesOverlayToRaster(t *testing.T) {
	const rw, rh = 200, 280
	raster := image.NewRGBA(image.Rect(0, 0, rw, rh))
	for i := range raster.Pix {
		raster.Pix[i] = 0xff // opaque white
	}
	// A transparent overlay at HALF the raster size, with an opaque black block at its centre (~50,70).
	ow, oh := rw/2, rh/2
	overlay := image.NewRGBA(image.Rect(0, 0, ow, oh))
	for y := oh/2 - 3; y <= oh/2+3; y++ {
		for x := ow/2 - 3; x <= ow/2+3; x++ {
			overlay.Set(x, y, color.RGBA{0, 0, 0, 0xff})
		}
	}

	out, err := composeLayers(raster, []composeLayer{{order: 0, load: func() (image.Image, error) { return overlay, nil }}})
	if err != nil {
		t.Fatal(err)
	}
	// The block scaled to the raster's centre (~100,140).
	if !isDark(out.At(rw/2, rh/2)) {
		t.Errorf("overlay mark not scaled to the raster centre (%d,%d) — composeLayers must resize it", rw/2, rh/2)
	}
	// It is NOT left at the overlay's own centre (~50,70) — that is the pre-fix bug.
	if isDark(out.At(ow/2, oh/2)) {
		t.Errorf("overlay mark drawn at its OWN bounds (%d,%d) — not scaled (pre-fix behaviour)", ow/2, oh/2)
	}
}

// TestComposeLayers_SameSizeUnchanged: a same-size overlay is composited 1:1 (no resample, no shift).
func TestComposeLayers_SameSizeUnchanged(t *testing.T) {
	const w, h = 120, 160
	raster := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := range raster.Pix {
		raster.Pix[i] = 0xff
	}
	overlay := image.NewRGBA(image.Rect(0, 0, w, h))
	overlay.Set(30, 40, color.RGBA{0, 0, 0, 0xff})
	out, err := composeLayers(raster, []composeLayer{{order: 0, load: func() (image.Image, error) { return overlay, nil }}})
	if err != nil {
		t.Fatal(err)
	}
	if !isDark(out.At(30, 40)) {
		t.Error("same-size overlay must composite 1:1 at its own coordinates")
	}
}

// TestResampleBilinear_Centre: a 2× upscale keeps a centred mark centred (half-pixel convention).
func TestResampleBilinear_Centre(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 10, 10))
	src.Set(5, 5, color.RGBA{0, 0, 0, 0xff})
	dst := resampleBilinear(src, 20, 20)
	// The dark sample near src (5,5) should land near dst (10,10) (2×), not at a corner.
	if !isDark(dst.At(10, 10)) && !isDark(dst.At(11, 11)) && !isDark(dst.At(9, 9)) {
		t.Errorf("2x upscale did not keep the centred mark near the centre: %v", dst.At(10, 10))
	}
}

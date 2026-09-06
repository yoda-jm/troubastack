package bake

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png" // register the PNG decoder used for page rasters + overlays
	"math"
)

// T149 — measure how far down a baked page's drawn content reaches, so Stage can trim a scroll page's blank
// tail instead of scrolling through empty paper. The measurement is a pure ink-extent scan of the same
// bytes the bundle already ships: the page raster (dark ink on white) and each overlay (opaque ink on
// transparent). It never alters a raster and never changes a coordinate — it only reports an extent.

// contentBottomPermille returns the lowest point reached by drawn content on a page, in PERMILLE of page
// height (0..1000), as the MAX over the page raster's dark ink and every overlay's opaque ink. A mark BELOW
// the text therefore keeps the page open far enough to show it (never cropping an annotation — the case
// VLL's own data contains). 0 when nothing decodes (⇒ the presenter shows the full page). The value is a
// raw content extent; the presenter adds its own breathing margin below it.
func contentBottomPermille(rasterPNG []byte, overlayPNGs [][]byte) int32 {
	best := 0
	if p, ok := inkBottomPermille(rasterPNG, isDarkInk); ok && p > best {
		best = p
	}
	for _, o := range overlayPNGs {
		if p, ok := inkBottomPermille(o, isOpaqueInk); ok && p > best {
			best = p
		}
	}
	return int32(best)
}

// inkBottomPermille decodes a PNG and returns the bottom edge of its lowest "ink" row, in permille of image
// height. It scans bottom-up and stops at the first ink row, so a page with content near the top costs only
// the blank tail. ok is false when the bytes do not decode (best-effort: the caller then contributes
// nothing for that image); a decoded image with no ink returns (0, true).
func inkBottomPermille(png []byte, isInk func(color.Color) bool) (int, bool) {
	if len(png) == 0 {
		return 0, false
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		return 0, false
	}
	b := img.Bounds()
	h := b.Dy()
	if h <= 0 {
		return 0, false
	}
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		for x := b.Min.X; x < b.Max.X; x++ {
			if isInk(img.At(x, y)) {
				rowsFromTop := y - b.Min.Y + 1 // bottom edge of this row, 1-indexed from the top
				return int(math.Round(float64(rowsFromTop) / float64(h) * 1000)), true
			}
		}
	}
	return 0, true // decoded, but blank
}

// isDarkInk marks a page-raster pixel as ink when it is opaque and not near-white — poppler renders the PDF
// as dark glyphs on an opaque white page, so anything meaningfully darker than white is drawn content
// (anti-aliased edges included).
func isDarkInk(c color.Color) bool {
	r, g, bl, a := c.RGBA() // each 0..0xffff
	if a < 0x8000 {
		return false
	}
	const nearWhite = 0xF500 // ~245/255 — tolerate JPEG/anti-alias noise, catch real glyph pixels
	return r < nearWhite || g < nearWhite || bl < nearWhite
}

// isOpaqueInk marks an overlay pixel as ink when it is meaningfully non-transparent — overlays are drawn on
// a transparent page, so any opaque pixel is a mark.
func isOpaqueInk(c color.Color) bool {
	_, _, _, a := c.RGBA() // 0..0xffff
	return a > 0x2000      // ~>12% opaque: ignore near-transparent anti-alias fringe
}

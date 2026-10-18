package bake

import (
	"bytes"
	"image"
	_ "image/png" // page rasters are PNG
	"math"
)

// Autodetect (T198): find a page's content and propose a crop rect, on the SAME page raster the bake already
// produces (pure Go, no new binary). A naive ink bounding box fails on real scans two ways — a dark band
// along a page edge (scanner bed / table edge / fold shadow) and an isolated dust speck — so detection works
// on CONNECTED COMPONENTS of ink and rejects both before taking the box. The box is then margined, grown to
// the page aspect ratio (policy A: a pure zoom, so the rect is a square in page-fraction space), and clamped.

// Tunables (named so they can be specced as bands, not adjectives). All fractions are of the page.
const (
	// A connected ink component smaller than this fraction of the page area is a speck (dust / JPEG noise),
	// not content. ~0.0004% of the page — a handful of pixels at bake DPI, well below any real glyph.
	autoMinComponentFrac = 0.000004
	// A connected ink component that TOUCHES a page edge AND spans at least this fraction of a page dimension
	// is a scanner/table/fold edge band, not content.
	autoEdgeBandSpanFrac = 0.9
	// Breathing margin added on every side of the content box before the AR grow.
	autoMarginFrac = 0.02
	// If the proposed crop would cover at least this fraction of the page, there is nothing worth cropping.
	autoMaxCoverFrac = 0.95
)

// detectContentRect proposes an autodetected crop for a page raster, in page fractions, as a square (policy
// A). ok is false when the page is blank, when nothing survives the noise guards, or when the content fills
// the page (≥ autoMaxCoverFrac) — in all of which the page bakes uncropped.
func detectContentRect(rasterPNG []byte) (cropRect, bool) {
	img, err := decodePNG(rasterPNG)
	if err != nil {
		return cropRect{}, false
	}
	w, h := img.w, img.h
	if w <= 0 || h <= 0 {
		return cropRect{}, false
	}
	x0, y0, x1, y1, ok := contentBoxPx(img)
	if !ok {
		return cropRect{}, false
	}
	return rectFromContentPx(x0, y0, x1, y1, w, h)
}

// rectFromContentPx turns a content pixel box into a margined, AR-grown, clamped square crop in fractions.
// Shared with the naive path in tests via contentBoxPx / naiveInkBoxPx so the two can be compared.
func rectFromContentPx(x0, y0, x1, y1, w, h int) (cropRect, bool) {
	// Content box in fractions, plus the margin.
	fx0 := float64(x0)/float64(w) - autoMarginFrac
	fy0 := float64(y0)/float64(h) - autoMarginFrac
	fx1 := float64(x1)/float64(w) + autoMarginFrac
	fy1 := float64(y1)/float64(h) + autoMarginFrac
	// Grow to the page aspect ratio => a square in fraction space (a page-shaped rect in pixels), centred on
	// the content.
	cx := (fx0 + fx1) / 2
	cy := (fy0 + fy1) / 2
	side := math.Max(fx1-fx0, fy1-fy0)
	if side >= autoMaxCoverFrac {
		return cropRect{}, false // covers (nearly) the whole page — don't crop
	}
	rx := cx - side/2
	ry := cy - side/2
	// Clamp inside the page without shrinking the side (shift the square in).
	if rx < 0 {
		rx = 0
	}
	if ry < 0 {
		ry = 0
	}
	if rx+side > 1 {
		rx = 1 - side
	}
	if ry+side > 1 {
		ry = 1 - side
	}
	return cropRect{x: rx, y: ry, w: side, h: side}, true
}

// grid is a decoded page raster as an ink bitmap: ink[y*w+x] is true where isDarkInk fired.
type grid struct {
	w, h int
	ink  []bool
}

func decodePNG(png []byte) (grid, error) {
	if len(png) == 0 {
		return grid{}, image.ErrFormat
	}
	img, _, err := image.Decode(bytes.NewReader(png))
	if err != nil {
		return grid{}, err
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	ink := make([]bool, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if isDarkInk(img.At(b.Min.X+x, b.Min.Y+y)) {
				ink[y*w+x] = true
			}
		}
	}
	return grid{w: w, h: h, ink: ink}, nil
}

// naiveInkBoxPx is the bounding box of ALL ink pixels — the version that fails on specks and edge bands. It
// exists so a test can prove the guards actually change the result (red-first against the naive box).
func naiveInkBoxPx(g grid) (x0, y0, x1, y1 int, ok bool) {
	x0, y0, x1, y1 = g.w, g.h, -1, -1
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if !g.ink[y*g.w+x] {
				continue
			}
			if x < x0 {
				x0 = x
			}
			if x > x1 {
				x1 = x
			}
			if y < y0 {
				y0 = y
			}
			if y > y1 {
				y1 = y
			}
		}
	}
	if x1 < 0 {
		return 0, 0, 0, 0, false
	}
	return x0, y0, x1 + 1, y1 + 1, true // half-open box
}

// contentBoxPx is the guarded content box: the union of the bounding boxes of ink COMPONENTS that are
// neither specks (too small) nor edge bands (border-touching and spanning most of a page dimension). ok is
// false when nothing survives.
func contentBoxPx(g grid) (x0, y0, x1, y1 int, ok bool) {
	minArea := int(autoMinComponentFrac * float64(g.w) * float64(g.h))
	if minArea < 6 {
		minArea = 6
	}
	spanW := int(autoEdgeBandSpanFrac * float64(g.w))
	spanH := int(autoEdgeBandSpanFrac * float64(g.h))

	labels := make([]int32, g.w*g.h) // 0 = unvisited
	var stack []int
	x0, y0, x1, y1 = g.w, g.h, -1, -1
	var cur int32
	for start := 0; start < len(g.ink); start++ {
		if !g.ink[start] || labels[start] != 0 {
			continue
		}
		cur++
		labels[start] = cur
		stack = stack[:0]
		stack = append(stack, start)
		// Component bbox + whether it touches a page border.
		cx0, cy0, cx1, cy1 := g.w, g.h, -1, -1
		area := 0
		touchesEdge := false
		for len(stack) > 0 {
			p := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			px := p % g.w
			py := p / g.w
			area++
			if px < cx0 {
				cx0 = px
			}
			if px > cx1 {
				cx1 = px
			}
			if py < cy0 {
				cy0 = py
			}
			if py > cy1 {
				cy1 = py
			}
			if px == 0 || py == 0 || px == g.w-1 || py == g.h-1 {
				touchesEdge = true
			}
			// 8-connectivity.
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					if dx == 0 && dy == 0 {
						continue
					}
					nx, ny := px+dx, py+dy
					if nx < 0 || ny < 0 || nx >= g.w || ny >= g.h {
						continue
					}
					n := ny*g.w + nx
					if g.ink[n] && labels[n] == 0 {
						labels[n] = cur
						stack = append(stack, n)
					}
				}
			}
		}
		if area < minArea {
			continue // speck
		}
		if touchesEdge && ((cx1-cx0+1) >= spanW || (cy1-cy0+1) >= spanH) {
			continue // edge band
		}
		if cx0 < x0 {
			x0 = cx0
		}
		if cy0 < y0 {
			y0 = cy0
		}
		if cx1 > x1 {
			x1 = cx1
		}
		if cy1 > y1 {
			y1 = cy1
		}
	}
	if x1 < 0 {
		return 0, 0, 0, 0, false
	}
	return x0, y0, x1 + 1, y1 + 1, true
}

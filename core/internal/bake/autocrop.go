package bake

import (
	"bytes"
	"context"
	"image"
	_ "image/png" // page rasters are PNG
	"math"
	"strconv"

	"troubastack/core/internal/app"
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
	// An edge band (scanner bed / table edge / fold shadow) is a component that TOUCHES a page border and is
	// THIN perpendicular to that border — at most this fraction of the perpendicular page dimension. Bounding
	// by thickness (not by how far it SPANS along the edge) is Fable's real-data fix: a band FUSED to the
	// music makes a THICK component, so it is kept and the content is not dropped with the band.
	//
	// Thin-only BY MEASUREMENT: there is deliberately no minimum LENGTH along the edge. The worry that a short
	// thin edge fragment (content) would be wrongly dropped was disproved on 1166 real pages — 580 proposals,
	// 0 cuts, and 18 pages came out TIGHTER (a short scan fragment at the edge that the span rule had stretched
	// the crop to is now dropped, keeping the music + handwriting inside). So no length threshold is applied.
	autoEdgeBandThickFrac = 0.05
	// Breathing margin added on every side of the content box before the AR grow.
	autoMarginFrac = 0.02
	// If the proposed crop would cover at least this fraction of the page, there is nothing worth cropping.
	autoMaxCoverFrac = 0.95
	// Floor for a proposed side, matching the save's minCropSide (0.2): a detected crop smaller than this is
	// grown to it, so an autodetect proposal always passes ValidateCrop.
	autoMinSideFrac = 0.2
)

// DetectCropForPages (T198 4b) proposes autodetected crops for an uploaded PDF, READ-ONLY: it rasterizes the
// PDF at base DPI through the SAME path a bake uses (b.raster.Rasterize, so detection sees exactly what would
// bake, and the render cache is shared) and runs detectContentRect per page. `want` selects pages (0-based);
// when `all` is true every page is considered and `want` is ignored. The result maps a stringified page index
// to its proposed rect (Source "auto", square per policy A); a page with no worthwhile crop is absent. A
// proposal carries a straightening angle only when `deskew` is set (the caller opts in); otherwise every
// proposal is upright (angle 0), T198's behaviour. It writes nothing — no bake, no sidecar, no file mutation.
func (b *Baker) DetectCropForPages(ctx context.Context, pdf []byte, want map[int]bool, all, deskew bool) (map[string]app.CropRect, error) {
	rasters, err := b.raster.Rasterize(ctx, pdf) // base DPI, uncropped (no crop in context)
	if err != nil {
		return nil, err
	}
	out := map[string]app.CropRect{}
	for i, raster := range rasters {
		if !all && !want[i] {
			continue
		}
		g, derr := decodePNG(raster)
		if derr != nil {
			continue
		}
		// T199: propose a deskew angle ONLY when the caller opts in (deskew). Studio's existing Auto button
		// merges the proposed rect whole and saves it, and there is no angle UI until S4 — so a default detect
		// must stay angle-0 (T198 behaviour), or the Auto button would silently produce rotated crops that
		// Studio still draws upright. When opted in, a confident angle is measured in the bake's leveled frame
		// (deskewContentRect) so the window lines up with how the bake reads the rect; if the page aspect can't
		// be read, the angle is dropped and the page proposes an upright T198 box.
		if deskew {
			angle, confident := detectPageAngle(g)
			if confident && angle != 0 {
				if asp, aerr := b.PageAspect(ctx, pdf, i); aerr == nil {
					if r, ok := deskewContentRect(g, angle, asp); ok {
						out[strconv.Itoa(i)] = app.CropRect{X: r.x, Y: r.y, W: r.w, H: r.h, Angle: r.angle, Source: "auto"}
					}
					continue
				}
			}
		}
		if r, ok := contentRectOfGrid(g); ok {
			out[strconv.Itoa(i)] = app.CropRect{X: r.x, Y: r.y, W: r.w, H: r.h, Source: "auto"}
		}
	}
	return out, nil
}

// detectContentRect proposes an autodetected crop for a page raster, in page fractions, as a square (policy
// A). ok is false when the page is blank, when nothing survives the noise guards, or when the content fills
// the page (≥ autoMaxCoverFrac) — in all of which the page bakes uncropped.
func detectContentRect(rasterPNG []byte) (cropRect, bool) {
	img, err := decodePNG(rasterPNG)
	if err != nil {
		return cropRect{}, false
	}
	return contentRectOfGrid(img)
}

// contentRectOfGrid is detectContentRect on an already-decoded ink grid — so the T199 autodetect can measure
// the content box on the STRAIGHTENED grid (spec: the bbox is measured after deskew) without re-encoding.
func contentRectOfGrid(g grid) (cropRect, bool) {
	if g.w <= 0 || g.h <= 0 {
		return cropRect{}, false
	}
	x0, y0, x1, y1, ok := contentBoxPx(g)
	if !ok {
		return cropRect{}, false
	}
	return rectFromContentPx(x0, y0, x1, y1, g.w, g.h)
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
	if side < autoMinSideFrac {
		side = autoMinSideFrac // tiny content: floor to the save's minimum so the proposal always validates
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

// contentComponents runs the connected-component walk and returns a per-pixel label array (0 = no ink) plus
// the set of labels that are real CONTENT — neither specks (too small) nor edge bands (border-touching and
// thin perpendicular to that border). Shared by contentBoxPx (the AABB) and deskewContentRect (the leveled
// box), so both see exactly the same surviving ink.
func contentComponents(g grid) (labels []int32, survivors map[int32]bool) {
	minArea := int(autoMinComponentFrac * float64(g.w) * float64(g.h))
	if minArea < 6 {
		minArea = 6
	}
	thickW := int(autoEdgeBandThickFrac * float64(g.w)) // max width of a vertical (left/right) edge band
	thickH := int(autoEdgeBandThickFrac * float64(g.h)) // max height of a horizontal (top/bottom) edge band

	labels = make([]int32, g.w*g.h) // 0 = unvisited
	survivors = map[int32]bool{}
	var stack []int
	var cur int32
	for start := 0; start < len(g.ink); start++ {
		if !g.ink[start] || labels[start] != 0 {
			continue
		}
		cur++
		labels[start] = cur
		stack = stack[:0]
		stack = append(stack, start)
		// Component bbox + which page borders it touches.
		cx0, cy0, cx1, cy1 := g.w, g.h, -1, -1
		area := 0
		touchesLeftRight := false
		touchesTopBottom := false
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
			if px == 0 || px == g.w-1 {
				touchesLeftRight = true
			}
			if py == 0 || py == g.h-1 {
				touchesTopBottom = true
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
		// Edge band: touches a border AND is THIN perpendicular to it. A vertical band hugs the left/right
		// edge and is narrow; a horizontal band hugs the top/bottom edge and is short. A component thicker
		// than the band bound (e.g. music fused to a band) is NOT a band and is kept.
		bw := cx1 - cx0 + 1
		bh := cy1 - cy0 + 1
		if (touchesLeftRight && bw <= thickW) || (touchesTopBottom && bh <= thickH) {
			continue // edge band
		}
		survivors[cur] = true
	}
	return labels, survivors
}

// contentBoxPx is the guarded content box: the pixel bounding box of the surviving (content) components. ok is
// false when nothing survives.
func contentBoxPx(g grid) (x0, y0, x1, y1 int, ok bool) {
	labels, survivors := contentComponents(g)
	x0, y0, x1, y1 = g.w, g.h, -1, -1
	for p, lab := range labels {
		if lab == 0 || !survivors[lab] {
			continue
		}
		px := p % g.w
		py := p / g.w
		if px < x0 {
			x0 = px
		}
		if px > x1 {
			x1 = px
		}
		if py < y0 {
			y0 = py
		}
		if py > y1 {
			y1 = py
		}
	}
	if x1 < 0 {
		return 0, 0, 0, 0, false
	}
	return x0, y0, x1 + 1, y1 + 1, true
}

// deskewContentRect builds the crop window for a DESKEWED page. The bake reads a crop as a source-frame box
// rotated about its OWN centre (app.PageViewTransform), so a box measured on a page straightened about some
// OTHER centre does not line up — off-centre tilted content falls outside the window. Instead we measure the
// surviving content directly in PageViewTransform's leveled (u,v) frame (u,v are the page point's coordinates
// with the tilt removed), take that box, and map its centre back to the source frame. Every surviving ink
// pixel then lies inside the window as the bake maps it, with the tilt gone. aspect is the page's rendered W/H
// (the same source the bake uses). At angle 0 this is exactly rectFromContentPx. ok is false on no content or
// a near-full-page box.
func deskewContentRect(g grid, angleDeg, aspect float64) (cropRect, bool) {
	if g.w <= 0 || g.h <= 0 || aspect <= 0 {
		return cropRect{}, false
	}
	labels, survivors := contentComponents(g)
	if len(survivors) == 0 {
		return cropRect{}, false
	}
	th := angleDeg * math.Pi / 180
	sin, cos := math.Sin(th), math.Cos(th)
	gw, gh := float64(g.w), float64(g.h)
	uMin, vMin := math.Inf(1), math.Inf(1)
	uMax, vMax := math.Inf(-1), math.Inf(-1)
	for p, lab := range labels {
		if lab == 0 || !survivors[lab] {
			continue
		}
		px := (float64(p%g.w) + 0.5) / gw // pixel centre, page fractions
		py := (float64(p/g.w) + 0.5) / gh
		u := cos*px + sin*py/aspect  // PageViewTransform's leveled coordinates, before the centre offset
		v := -sin*px*aspect + cos*py // (the forward map; its inverse has det 1, recovered below)
		if u < uMin {
			uMin = u
		}
		if u > uMax {
			uMax = u
		}
		if v < vMin {
			vMin = v
		}
		if v > vMax {
			vMax = v
		}
	}
	// Margin + policy-A square, in the leveled frame (a square in u,v is what the bake renders upright).
	side := math.Max(uMax-uMin, vMax-vMin) + 2*autoMarginFrac
	if side >= autoMaxCoverFrac {
		return cropRect{}, false
	}
	if side < autoMinSideFrac {
		side = autoMinSideFrac
	}
	// Map the leveled box centre back to the source page frame (inverse of the (u,v) map; det = 1).
	uc, vc := (uMin+uMax)/2, (vMin+vMax)/2
	cx := cos*uc - sin*vc/aspect
	cy := sin*aspect*uc + cos*vc
	rx, ry := cx-side/2, cy-side/2
	// Clamp the unrotated box into the page without shrinking it (same as rectFromContentPx).
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
	return cropRect{x: rx, y: ry, w: side, h: side, angle: angleDeg}, true
}

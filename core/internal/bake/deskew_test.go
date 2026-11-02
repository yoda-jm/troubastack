package bake

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// T199 deskew quality — these tests exercise the rotation resampler (rotateWindow) directly on synthetic
// pages, with no pdftoppm, so they are fast and deterministic. The spec's acceptance is that a straight EDGE
// in the source stays as sharp after straightening as in an unrotated crop, and that the two WRONG resamplers
// each fail — on DIFFERENT metrics, because one metric cannot catch both (measured: the spec's old "rows above
// 50% thickness" bound separated NEITHER wrong resampler for a thin line, so §Quality was amended to this):
//   - BILINEAR blurs, so it fails the SHARPNESS bound (a straightened step edge's 20→80% transition is ~0.85px
//     vs bicubic's ~0.46px); a nearest-neighbour staircase keeps a sharp edge and passes sharpness.
//   - NEAREST-NEIGHBOUR staircases, so it fails the STRAIGHTNESS bound (a straightened edge's per-row position
//     jumps by whole pixels, ~0.23px RMS); bilinear's smooth ramp passes straightness (~0.002px).
// bicubic (the production rotateWindow) passes BOTH. The two wrong resamplers below live only in the test, to
// prove each bound discriminates; the thin-line thickness check survives only as a bicubic ACCEPTANCE check.

// synthCrookedLine draws a thin black line through the image centre at angleDeg (y-down image frame), on a
// white background, anti-aliased by coverage — WITHOUT any rotation resampler, so it is an independent
// fixture. halfThick is the line's half-thickness in px (1 ⇒ a ~2px line).
func synthCrookedLine(size int, angleDeg, halfThick float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	th := angleDeg * math.Pi / 180
	// Unit normal to the line (the line direction is (cos,sin); the normal is (-sin,cos)).
	nx, ny := -math.Sin(th), math.Cos(th)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dist := (float64(x)+0.5-c)*nx + (float64(y)+0.5-c)*ny
			cov := halfThick + 0.5 - math.Abs(dist) // 1px anti-alias ramp at the edges
			if cov < 0 {
				cov = 0
			}
			if cov > 1 {
				cov = 1
			}
			v := uint8(math.Round(255 * (1 - cov))) // black line (0) on white (255)
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

// synthCrookedEdge draws a vertical black/white STEP EDGE through the centre at angleDeg from vertical, on a
// white left / black right page, anti-aliased by coverage — the cleanest sharpness fixture (the spec: "a
// straight edge in the source must stay as sharp as in an unrotated crop"). No rotation resampler is used.
func synthCrookedEdge(size int, angleDeg float64) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	c := float64(size) / 2
	th := angleDeg * math.Pi / 180
	// A vertical edge tilted by th: signed horizontal distance uses the edge normal (cos, sin).
	nx, ny := math.Cos(th), math.Sin(th)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			d := (float64(x)+0.5-c)*nx + (float64(y)+0.5-c)*ny // + = right of the edge (black)
			cov := d + 0.5                                     // 1px anti-alias ramp across the edge
			if cov < 0 {
				cov = 0
			}
			if cov > 1 {
				cov = 1
			}
			v := uint8(math.Round(255 * (1 - cov))) // white (255) left, black (0) right
			img.Set(x, y, color.RGBA{v, v, v, 255})
		}
	}
	return img
}

// edgeTransitionWidth measures sharpness: at each central ROW, the horizontal distance between the 80%-white
// and 80%-ink crossings of the straightened edge, averaged. A blurring (bilinear) resampler widens it; a
// nearest-neighbour or bicubic keeps it near the 1px anti-alias ramp. Rows with no clean crossing are skipped.
func edgeTransitionWidth(img *image.RGBA, y0, y1 int) float64 {
	w := img.Bounds().Dx()
	var sum float64
	var n int
	for y := y0; y < y1; y++ {
		lo, hi := -1.0, -1.0
		for x := 1; x < w; x++ {
			l := lum(img, x, y)
			if hi < 0 && l <= 204 { // first 80%-white→ descending past 20% ink
				hi = float64(x)
			}
			if l <= 51 { // reached 80% ink
				lo = float64(x)
				break
			}
		}
		if hi >= 0 && lo >= hi {
			sum += lo - hi
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// edgeStraightness is the RMS residual of the straightened edge's x-position per row (the 50% crossing) about
// its mean — a nearest-neighbour staircase jumps by whole px (large), a smooth kernel stays near-vertical.
func edgeStraightness(img *image.RGBA, y0, y1 int) float64 {
	w := img.Bounds().Dx()
	var xs []float64
	for y := y0; y < y1; y++ {
		for x := 1; x < w; x++ {
			if lum(img, x, y) <= 128 {
				// sub-pixel 50% crossing between x-1 and x
				l0, l1 := lum(img, x-1, y), lum(img, x, y)
				frac := 0.0
				if l0 != l1 {
					frac = (l0 - 128) / (l0 - l1)
				}
				xs = append(xs, float64(x-1)+frac)
				break
			}
		}
	}
	if len(xs) == 0 {
		return 0
	}
	var mean float64
	for _, v := range xs {
		mean += v
	}
	mean /= float64(len(xs))
	var ss float64
	for _, v := range xs {
		ss += (v - mean) * (v - mean)
	}
	return math.Sqrt(ss / float64(len(xs)))
}

// sampleKind selects the test-local resampler used to straighten the fixture.
type sampleKind int

const (
	kindBicubic sampleKind = iota // the production rotateWindow
	kindBilinear
	kindNearest
)

// straighten rotates src by angleDeg about (cx,cy) into an outW×outH upright image, with the chosen sampler.
// kindBicubic defers to the production rotateWindow; the other two mirror its inverse mapping AND its 8-bit
// conversion (clamp8) exactly, so the ONLY difference measured is the interpolation kernel.
func straighten(src image.Image, outW, outH int, cx, cy, angleDeg float64, kind sampleKind) *image.RGBA {
	if kind == kindBicubic {
		return rotateWindow(src, outW, outH, cx, cy, angleDeg)
	}
	dst := image.NewRGBA(image.Rect(0, 0, outW, outH))
	b := src.Bounds()
	sw, sh := b.Dx(), b.Dy()
	th := angleDeg * math.Pi / 180
	sin, cos := math.Sin(th), math.Cos(th)
	ocx, ocy := float64(outW)/2, float64(outH)/2
	at := func(sx, sy int) (float64, float64, float64, float64) {
		if sx < 0 || sx >= sw || sy < 0 || sy >= sh {
			return 65535, 65535, 65535, 65535
		}
		r, g, bl, a := src.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
		return float64(r), float64(g), float64(bl), float64(a)
	}
	for oy := 0; oy < outH; oy++ {
		dy := float64(oy) + 0.5 - ocy
		for ox := 0; ox < outW; ox++ {
			dx := float64(ox) + 0.5 - ocx
			fx := cx + (cos*dx - sin*dy) - 0.5
			fy := cy + (sin*dx + cos*dy) - 0.5
			var r, g, bl, a float64
			if kind == kindNearest {
				r, g, bl, a = at(int(math.Round(fx)), int(math.Round(fy)))
			} else { // bilinear
				x0, y0 := int(math.Floor(fx)), int(math.Floor(fy))
				wx, wy := fx-float64(x0), fy-float64(y0)
				r00, g00, b00, a00 := at(x0, y0)
				r10, g10, b10, a10 := at(x0+1, y0)
				r01, g01, b01, a01 := at(x0, y0+1)
				r11, g11, b11, a11 := at(x0+1, y0+1)
				lerp := func(c00, c10, c01, c11 float64) float64 {
					return (c00*(1-wx)+c10*wx)*(1-wy) + (c01*(1-wx)+c11*wx)*wy
				}
				r, g, bl, a = lerp(r00, r10, r01, r11), lerp(g00, g10, g01, g11), lerp(b00, b10, b01, b11), lerp(a00, a10, a01, a11)
			}
			i := dst.PixOffset(ox, oy)
			dst.Pix[i+0] = clamp8(r)
			dst.Pix[i+1] = clamp8(g)
			dst.Pix[i+2] = clamp8(bl)
			dst.Pix[i+3] = clamp8(a)
		}
	}
	return dst
}

// lum returns a pixel's luminance (0=black, 255=white); the fixtures are grey, so R==G==B. Used by every metric.
func lum(img *image.RGBA, x, y int) float64 {
	i := img.PixOffset(x, y)
	return float64(img.Pix[i]) // grey fixture: R==G==B
}

// thicknessAtCols is the MAX over the central columns of the count of ink rows (lum < 128) — the straightened
// line's vertical thickness. A blurring resampler inflates it.
func thicknessAtCols(img *image.RGBA, x0, x1 int) int {
	h := img.Bounds().Dy()
	worst := 0
	for x := x0; x < x1; x++ {
		n := 0
		for y := 0; y < h; y++ {
			if lum(img, x, y) < 128 {
				n++
			}
		}
		if n > worst {
			worst = n
		}
	}
	return worst
}

// straightnessResidual fits a horizontal line to the ink centroid row per column and returns the RMS residual
// (px). A staircasing (nearest-neighbour) resampler makes the centroid jump by whole pixels → large residual;
// a smooth kernel keeps it near flat. Columns with no ink are skipped.
func straightnessResidual(img *image.RGBA, x0, x1 int) float64 {
	h := img.Bounds().Dy()
	var cols []float64
	for x := x0; x < x1; x++ {
		var num, den float64
		for y := 0; y < h; y++ {
			ink := (255 - lum(img, x, y)) / 255
			num += ink * float64(y)
			den += ink
		}
		if den > 0 {
			cols = append(cols, num/den)
		}
	}
	if len(cols) == 0 {
		return 0
	}
	var mean float64
	for _, c := range cols {
		mean += c
	}
	mean /= float64(len(cols))
	var ss float64
	for _, c := range cols {
		ss += (c - mean) * (c - mean)
	}
	return math.Sqrt(ss / float64(len(cols)))
}

// singlePeak reports whether the row ink-sum profile over the central columns has exactly one local maximum
// run — a blurred or doubled line shows one peak; a staircase/aliased line can split into several.
func singlePeak(img *image.RGBA, x0, x1 int) bool {
	h := img.Bounds().Dy()
	prof := make([]float64, h)
	for y := 0; y < h; y++ {
		for x := x0; x < x1; x++ {
			prof[y] += 255 - lum(img, x, y)
		}
	}
	peaks := 0
	for y := 1; y < h-1; y++ {
		if prof[y] > prof[y-1] && prof[y] >= prof[y+1] && prof[y] > 0 {
			peaks++
		}
	}
	return peaks == 1
}

// The two discriminating bounds, placed in the gaps the measurement opened (angle 3°):
//
//	sharpness (edge transition width): bicubic ≈0.46px, nearest ≈0.56px, BILINEAR ≈0.85px (blurs) → ≤0.70 red-lines
//	  bilinear only; straightness: bicubic/bilinear ≈0.002px, NEAREST ≈0.23px (staircase) → ≤0.05 red-lines nearest
//	  only. bicubic passes both, which is the acceptance bar; each wrong resampler fails exactly ONE metric, so
//	  neither bound is vacuous and neither double-counts.
const (
	maxEdgeTransitionPx = 0.70
	maxEdgeStraightPx   = 0.05
)

// TestDeskewEdgeSharpnessAndStraightness proves the two bounds discriminate (B3): a straightened STEP EDGE
// stays sharp (bicubic) — BILINEAR blurs and fails sharpness while staying straight; NEAREST staircases and
// fails straightness while staying sharp; bicubic passes both. The `want` matrix pins which metric catches
// which, so a sabotage that swaps the production resampler for either wrong one reds exactly one assertion.
func TestDeskewEdgeSharpnessAndStraightness(t *testing.T) {
	const size, out, angle = 400, 280, 3.0
	cx, cy := float64(size)/2, float64(size)/2
	y0, y1 := out/2-80, out/2+80
	crooked := synthCrookedEdge(size, angle)

	type want struct{ sharp, straight bool }
	names := map[sampleKind]string{kindBicubic: "bicubic", kindBilinear: "bilinear", kindNearest: "nearest"}
	cases := map[sampleKind]want{
		kindBicubic:  {sharp: true, straight: true},  // production: sharp AND straight
		kindBilinear: {sharp: false, straight: true}, // blurs → fails sharpness ONLY
		kindNearest:  {sharp: true, straight: false}, // staircases → fails straightness ONLY
	}
	for kind, w := range cases {
		img := straighten(crooked, out, out, cx, cy, angle, kind)
		tw := edgeTransitionWidth(img, y0, y1)
		st := edgeStraightness(img, y0, y1)
		t.Logf("%-9s transitionWidth=%.3f (≤%.2f) straightness=%.3f (≤%.2f)", names[kind], tw, maxEdgeTransitionPx, st, maxEdgeStraightPx)
		if (tw <= maxEdgeTransitionPx) != w.sharp {
			t.Errorf("%s sharpness: transitionWidth=%.3f, bound ≤%.2f, want pass=%v", names[kind], tw, maxEdgeTransitionPx, w.sharp)
		}
		if (st <= maxEdgeStraightPx) != w.straight {
			t.Errorf("%s straightness: residual=%.3f, bound ≤%.2f, want pass=%v", names[kind], st, maxEdgeStraightPx, w.straight)
		}
	}
}

// TestDeskewThinLineAcceptance is a bicubic ACCEPTANCE check: a thin black line deskewed by its own
// crookedness keeps a vertical thickness within unrotated+1px and a single-peak row profile. It does NOT
// discriminate — measured, ALL THREE resamplers (bicubic, bilinear, nearest) pass it for a thin line, which
// is exactly why §Quality moved the discriminating bar to edge sharpness (see the sharpness test above). Kept
// only to pin that the production kernel still meets the spec's original thin-line shape.
func TestDeskewThinLineAcceptance(t *testing.T) {
	const size, out, angle, halfThick = 400, 280, 3.0, 1.0
	cx, cy := float64(size)/2, float64(size)/2
	x0, x1 := out/2-80, out/2+80
	t0 := thicknessAtCols(rotateWindow(synthCrookedLine(size, 0, halfThick), out, out, cx, cy, 0), x0, x1)
	got := rotateWindow(synthCrookedLine(size, angle, halfThick), out, out, cx, cy, angle)
	if th := thicknessAtCols(got, x0, x1); th > t0+1 {
		t.Errorf("straightened thin-line thickness %d exceeds unrotated %d + 1", th, t0)
	}
	if !singlePeak(got, x0, x1) {
		t.Error("straightened thin-line row profile must have a single peak")
	}
}

// TestDeskewSignConvention pins the angle's sign (spec): a window rotated +θ to match a line crooked by +θ
// yields a LEVEL line. Deskewing the same +θ line by −θ instead doubles the tilt, so its per-column centroid
// slopes steeply — many times the residual of the correct direction. Reds if rotateWindow's rotation sense flips.
func TestDeskewSignConvention(t *testing.T) {
	const size, out, angle, halfThick = 400, 280, 4.0, 1.0
	cx, cy := float64(size)/2, float64(size)/2
	x0, x1 := out/2-80, out/2+80
	crooked := synthCrookedLine(size, angle, halfThick)
	level := straightnessResidual(rotateWindow(crooked, out, out, cx, cy, angle), x0, x1)  // +θ straightens it
	worse := straightnessResidual(rotateWindow(crooked, out, out, cx, cy, -angle), x0, x1) // −θ doubles the tilt
	if level > 0.1 {
		t.Errorf("deskew by +θ must level a +θ line, residual=%.3f", level)
	}
	if worse < 3*level+0.5 {
		t.Errorf("deskew by −θ must leave the line clearly MORE sloped than +θ (−θ=%.3f vs +θ=%.3f)", worse, level)
	}
}

// TestDeskewWindowSpills covers B1b's geometry: the rotated-corner-inside-the-page test that decides whether
// the bake had to clamp. A centred window at a mild angle stays inside; a window flush to an edge spills once
// rotated; angle 0 never spills. The PHYSICAL-units rotation (page aspect) matters: the SAME window spills at
// one aspect but not another — a sabotage that rotates in normalized space (dropping the aspect) reds the
// aspect case.
func TestDeskewWindowSpills(t *testing.T) {
	// Centred, mild angle, square page: stays inside.
	if deskewWindowSpills(cropRect{x: 0.2, y: 0.2, w: 0.6, h: 0.6, angle: 5}, 1.0) {
		t.Error("a centred window at 5° must not spill")
	}
	// Flush to the left edge, 10°, square page: a bottom corner rotates past x=0.
	if !deskewWindowSpills(cropRect{x: 0, y: 0.2, w: 0.6, h: 0.6, angle: 10}, 1.0) {
		t.Error("an edge-flush window at 10° must spill")
	}
	// angle 0 never spills, even flush to the edge.
	if deskewWindowSpills(cropRect{x: 0, y: 0, w: 1, h: 1, angle: 0}, 1.0) {
		t.Error("angle 0 must never spill")
	}
	// Aspect-sensitivity: a near-top window rotated on a WIDE page pushes a corner past y=0 (the physical
	// x-extent is stretched, so rotation lifts the corner more in normalized y), while the SAME window on a
	// square page stays in. Proves the aspect enters the rotation (physical units), not just the raw fractions.
	wide := cropRect{x: 0.2, y: 0.08, w: 0.6, h: 0.6, angle: 8}
	if deskewWindowSpills(wide, 1.0) {
		t.Error("the window must stay inside on a square page")
	}
	if !deskewWindowSpills(wide, 3.0) {
		t.Error("the same window must spill on a 3:1 wide page (aspect must enter the rotation)")
	}
}

// TestRotateWindowPreservesGreys pins clamp8's 16→8-bit conversion: rotating a UNIFORM grey page returns the
// SAME grey for every one of the 256 levels (a rotation of a constant is that constant — the bicubic weights
// sum to 1, and the output window sits fully inside src so nothing reads the white fill). The old v/256
// conversion lifted greys 128–254 by one level, silently re-rendering a page nobody deskewed; this reds it.
func TestRotateWindowPreservesGreys(t *testing.T) {
	const size, out = 24, 12
	for g := 0; g < 256; g++ {
		src := image.NewRGBA(image.Rect(0, 0, size, size))
		for y := 0; y < size; y++ {
			for x := 0; x < size; x++ {
				src.Set(x, y, color.RGBA{uint8(g), uint8(g), uint8(g), 255})
			}
		}
		got := rotateWindow(src, out, out, float64(size)/2, float64(size)/2, 0.05)
		for y := 0; y < out; y++ {
			for x := 0; x < out; x++ {
				if v := int(lum(got, x, y)); v != g {
					t.Fatalf("grey %d came back as %d after a 0.05° rotation", g, v)
				}
			}
		}
	}
}

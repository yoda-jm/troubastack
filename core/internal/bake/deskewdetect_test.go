package bake

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"

	"troubastack/core/internal/app"
)

// synthLines builds a w×h ink grid with horizontal lines tilted by angleDeg, one per entry in yFracs (its
// centre row), each `thick` px thick. A line point at column x sits at row y0 + (x−cx)·tanθ.
func synthLines(w, h int, angleDeg float64, yFracs []float64, thick float64) grid {
	ink := make([]bool, w*h)
	tan := math.Tan(angleDeg * math.Pi / 180)
	cx := float64(w) / 2
	for _, yf := range yFracs {
		y0 := yf * float64(h)
		for x := 0; x < w; x++ {
			yc := y0 + (float64(x)-cx)*tan
			for dy := -thick / 2; dy <= thick/2; dy += 0.5 {
				y := int(math.Round(yc + dy))
				if y >= 0 && y < h {
					ink[y*w+x] = true
				}
			}
		}
	}
	return grid{w: w, h: h, ink: ink}
}

// evenFracs returns n line centres evenly spread down the page (excluding the very edges).
func evenFracs(n int) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		out[i] = float64(i+1) / float64(n+1)
	}
	return out
}

// naiveStrongestAngle is the WRONG detector the spec warns about: argmax of the profile variance over the
// sweep, with no multi-line check. It picks a lone line's own angle. Test-local, to prove the multi-line
// condition is what rejects that line.
func naiveStrongestAngle(g grid) float64 {
	bestV, bestA := -1.0, 0.0
	for a := -deskewDetectMaxDeg; a <= deskewDetectMaxDeg+1e-9; a += deskewSweepFine {
		if v := variance(rowProfile(g, a)); v > bestV {
			bestV, bestA = v, a
		}
	}
	return bestA
}

func TestDeskewDetectVectors(t *testing.T) {
	const w, h = 800, 1000
	type want struct {
		conf  bool
		angle float64 // checked only when conf
	}
	cases := []struct {
		name string
		g    grid
		w    want
	}{
		{"staves +2.3", synthLines(w, h, 2.3, evenFracs(40), 2), want{true, 2.3}}, // detected within ±0.1
		{"staves -4", synthLines(w, h, -4, evenFracs(40), 2), want{true, -4}},
		{"straight 0", synthLines(w, h, 0, evenFracs(40), 2), want{false, 0}},    // dead-band ⇒ no deskew
		{"blank", grid{w: w, h: h, ink: make([]bool, w*h)}, want{false, 0}},      // S3-4: no ink ⇒ 0, no NaN
		{"one line +6", synthLines(w, h, 6, []float64{0.5}, 2), want{false, 0}},  // S3-2: one line ⇒ propose 0
		{"diagonal 35", synthLines(w, h, 35, []float64{0.5}, 2), want{false, 0}}, // true diagonal ⇒ propose 0
	}
	for _, c := range cases {
		a, conf := detectPageAngle(c.g)
		if conf != c.w.conf {
			t.Errorf("%s: confident=%v (angle %.2f, peaks@θ* %d), want %v", c.name, conf, a, countSeparatedPeaks(rowProfile(c.g, a)), c.w.conf)
		}
		if conf && math.Abs(a-c.w.angle) > 0.1 {
			t.Errorf("%s: angle=%.2f, want %.2f ±0.1", c.name, a, c.w.angle)
		}
	}
	// The lone-line discrimination (S3-1/S3-2): a naive strongest-angle detector picks the +6° line's own
	// angle — which detectPageAngle must NOT propose. This is why the multi-line peak count is required.
	oneLine := synthLines(w, h, 6, []float64{0.5}, 2)
	if na := naiveStrongestAngle(oneLine); math.Abs(na-6) > 0.2 {
		t.Errorf("sanity: a naive detector should pick the line's ~6°, got %.2f (if this fails the vector can't discriminate)", na)
	}
	if _, conf := detectPageAngle(oneLine); conf {
		t.Error("the multi-line condition must reject a lone +6° line (confident must be false)")
	}
}

// synthBlock inks `n` lines tilted by angleDeg within a centred block (x,y each in [lo,hi] fractions).
func synthBlock(w, h int, angleDeg float64, n int, lo, hi float64) grid {
	ink := make([]bool, w*h)
	tan := math.Tan(angleDeg * math.Pi / 180)
	cx := float64(w) / 2
	x0, x1 := int(lo*float64(w)), int(hi*float64(w))
	for i := 0; i < n; i++ {
		yf := lo + (hi-lo)*float64(i+1)/float64(n+1)
		y0 := yf * float64(h)
		for x := x0; x < x1; x++ {
			yc := y0 + (float64(x)-cx)*tan
			for dy := -1.0; dy <= 1; dy += 0.5 {
				if y := int(math.Round(yc + dy)); y >= 0 && y < h {
					ink[y*w+x] = true
				}
			}
		}
	}
	return grid{w: w, h: h, ink: ink}
}

func gridToPNG(t *testing.T, g grid) []byte {
	t.Helper()
	im := image.NewGray(image.Rect(0, 0, g.w, g.h))
	for i := range im.Pix {
		im.Pix[i] = 255
	}
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			if g.ink[y*g.w+x] {
				im.SetGray(x, y, color.Gray{Y: 0})
			}
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, im); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// TestDetectCropForPagesDeskew wires it end to end AND covers B1's opt-in: a page tilted +5° proposes the +5°
// angle ONLY when the caller opts in (deskew=true). With the default (deskew=false) the SAME page proposes an
// upright box (angle 0), so T198's existing Auto button keeps producing upright crops until S4 adds the angle
// UI. Sabotage: make DetectCropForPages ignore the flag and always propose the angle → the default-case
// assertion below (angle must be 0) goes red.
func TestDetectCropForPagesDeskew(t *testing.T) {
	g := synthBlock(400, 520, 5, 20, 0.25, 0.75)
	b := &Baker{raster: fakeRaster{pages: 1, png: gridToPNG(t, g)}, aspecter: FixedAspecter{Aspects: []float64{400.0 / 520.0}}}

	// Opt in: the proposal carries the +5° angle and is a valid policy-A square.
	rects, err := b.DetectCropForPages(context.Background(), nil, nil, true, true)
	if err != nil {
		t.Fatal(err)
	}
	r, ok := rects["0"]
	if !ok {
		t.Fatal("no crop proposed for the tilted page")
	}
	if math.Abs(r.Angle-5) > 0.1 {
		t.Errorf("opt-in: proposed angle=%.2f, want 5 ±0.1", r.Angle)
	}
	if math.Abs(r.W-r.H) > 1e-6 {
		t.Errorf("proposal must be square (policy A), got w=%.4f h=%.4f", r.W, r.H)
	}
	if r.X < -1e-6 || r.Y < -1e-6 || r.X+r.W > 1+1e-6 || r.Y+r.H > 1+1e-6 {
		t.Errorf("proposal out of the page: %+v", r)
	}
	if r.Source != "auto" {
		t.Errorf("source=%q, want auto", r.Source)
	}

	// Default (no opt-in): the SAME tilted page proposes an upright box — angle 0.
	def, err := b.DetectCropForPages(context.Background(), nil, nil, true, false)
	if err != nil {
		t.Fatal(err)
	}
	dr, ok := def["0"]
	if !ok {
		t.Fatal("default: no crop proposed for the tilted page")
	}
	if dr.Angle != 0 {
		t.Errorf("default (no opt-in) must propose angle 0, got %.2f", dr.Angle)
	}
}

// TestDeskewDetectFeedsBakeSign closes the sign loop: the angle the detector proposes, fed to the REAL bake
// resampler (rotateWindow), must LEVEL the content — not double its tilt. This guards against the detector and
// the bake disagreeing on the sign of +θ (the y-flip trap), which a value-only angle check would miss.
func TestDeskewDetectFeedsBakeSign(t *testing.T) {
	const size, out = 400, 280
	cx, cy := float64(size)/2, float64(size)/2
	x0, x1 := out/2-80, out/2+80
	crooked := synthCrookedLine(size, 4.0, 1.0) // a +4° line, same slope convention as synthLines
	var buf bytes.Buffer
	if err := png.Encode(&buf, crooked); err != nil {
		t.Fatal(err)
	}
	g, err := decodePNG(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	theta, _ := detectPageAngle(g) // one line ⇒ not "confident", but the detected ANGLE is what we check
	if math.Abs(theta-4) > 0.1 {
		t.Fatalf("detected %.2f°, want ~4°", theta)
	}
	level := straightnessResidual(rotateWindow(crooked, out, out, cx, cy, theta), x0, x1)
	worse := straightnessResidual(rotateWindow(crooked, out, out, cx, cy, -theta), x0, x1)
	if level > 0.1 {
		t.Errorf("the detected angle must LEVEL via the real bake resampler, residual=%.3f", level)
	}
	if worse < 3*level+0.5 {
		t.Errorf("sign: −detected must leave the line clearly MORE sloped (−θ=%.3f vs +θ=%.3f)", worse, level)
	}
}

// synthBlockAt inks ~16 tilted lines filling a page-shaped block of (wFrac x hFrac) centred at (cxFrac,cyFrac),
// so the block can be placed off-centre and in corners.
func synthBlockAt(w, h int, angleDeg, cxFrac, cyFrac, wFrac, hFrac float64) grid {
	ink := make([]bool, w*h)
	tan := math.Tan(angleDeg * math.Pi / 180)
	cx, cy := cxFrac*float64(w), cyFrac*float64(h)
	halfW, halfH := wFrac*float64(w)/2, hFrac*float64(h)/2
	const n = 16
	for i := 0; i < n; i++ {
		ly := cy - halfH + 2*halfH*float64(i)/float64(n-1)
		for xx := cx - halfW; xx <= cx+halfW; xx++ {
			yy := ly + (xx-cx)*tan // tilt about the block centre
			for dy := -1.0; dy <= 1; dy += 0.5 {
				x, y := int(math.Round(xx)), int(math.Round(yy+dy))
				if x >= 0 && x < w && y >= 0 && y < h {
					ink[y*w+x] = true
				}
			}
		}
	}
	return grid{w: w, h: h, ink: ink}
}

// TestDeskewContentRectContainsInk is Fable's property: the bake reads the crop as a source-frame box rotated
// about its OWN centre (app.PageViewTransform), so EVERY ink pixel of a tilted, off-centre block must map
// inside [0,1]^2 under that transform with the proposed rect. Tested at ±3/±5/±8°, in the centre and three
// corners, on a NON-SQUARE page with a page-shaped block — the case the earlier page-centre straightening got
// wrong.
func TestDeskewContentRectContainsInk(t *testing.T) {
	const w, h = 400, 520 // non-square
	aspect := float64(w) / float64(h)
	angles := []float64{3, -3, 5, -5, 8, -8}
	spots := []struct {
		name   string
		cx, cy float64
	}{
		{"center", 0.50, 0.50},
		{"top-left", 0.28, 0.27},
		{"top-right", 0.72, 0.30},
		{"bottom-left", 0.27, 0.73},
	}
	for _, ang := range angles {
		for _, s := range spots {
			g := synthBlockAt(w, h, ang, s.cx, s.cy, 0.30, 0.39) // page-shaped block
			detected, _ := detectPageAngle(g)
			r, ok := deskewContentRect(g, detected, aspect)
			if !ok {
				t.Fatalf("%+.0f° %s: no rect proposed", ang, s.name)
			}
			outside := 0
			for p := range g.ink {
				if !g.ink[p] {
					continue
				}
				px := (float64(p%w) + 0.5) / float64(w)
				py := (float64(p/w) + 0.5) / float64(h)
				ox, oy := app.PageViewTransform(px, py, r.x, r.y, r.w, r.h, r.angle, aspect)
				if ox < -1e-9 || ox > 1+1e-9 || oy < -1e-9 || oy > 1+1e-9 {
					outside++
				}
			}
			if outside > 0 {
				t.Errorf("%+.0f° %s: %d ink px fall outside the window as the bake reads it", ang, s.name, outside)
			}
		}
	}
}

// straightenAboutPageCentre reproduces the OLD (buggy) approach: rotate the ink bitmap about the PAGE centre to
// level it, so a box can be measured on it. Test-local — the production code no longer does this.
func straightenAboutPageCentre(g grid, angleDeg float64) grid {
	th := angleDeg * math.Pi / 180
	sin, cos := math.Sin(th), math.Cos(th)
	cx, cy := float64(g.w)/2, float64(g.h)/2
	out := make([]bool, g.w*g.h)
	for y := 0; y < g.h; y++ {
		for x := 0; x < g.w; x++ {
			dx, dy := float64(x)-cx, float64(y)-cy
			sx := int(math.Round(cx + cos*dx - sin*dy))
			sy := int(math.Round(cy + sin*dx + cos*dy))
			if sx >= 0 && sx < g.w && sy >= 0 && sy < g.h && g.ink[sy*g.w+sx] {
				out[y*g.w+x] = true
			}
		}
	}
	return grid{w: g.w, h: g.h, ink: out}
}

// TestDeskewBoxBeatsNaivePageFrame is the positive control for the containment property: the OLD approach —
// measure the box on a grid straightened about the PAGE centre, then attach the angle — must leave ink OUTSIDE
// the window for an off-centre tilted block (Fable's probe: 48 px at +8°), while deskewContentRect contains all
// of it. If the old box also passed, the containment test would be proving nothing.
func TestDeskewBoxBeatsNaivePageFrame(t *testing.T) {
	const w, h = 400, 520
	aspect := float64(w) / float64(h)
	g := synthBlockAt(w, h, 8, 0.72, 0.30, 0.30, 0.39) // off-centre (top-right), +8°
	ang, _ := detectPageAngle(g)
	count := func(r cropRect) int {
		out := 0
		for p := range g.ink {
			if !g.ink[p] {
				continue
			}
			px := (float64(p%w) + 0.5) / float64(w)
			py := (float64(p/w) + 0.5) / float64(h)
			ox, oy := app.PageViewTransform(px, py, r.x, r.y, r.w, r.h, r.angle, aspect)
			if ox < -1e-9 || ox > 1+1e-9 || oy < -1e-9 || oy > 1+1e-9 {
				out++
			}
		}
		return out
	}
	good, ok := deskewContentRect(g, ang, aspect)
	if !ok {
		t.Fatal("no deskew rect")
	}
	if n := count(good); n != 0 {
		t.Errorf("deskewContentRect should contain all ink, %d outside", n)
	}
	// Old approach: box measured on the page-centre-straightened grid, angle attached.
	old, ok := contentRectOfGrid(straightenAboutPageCentre(g, ang))
	if !ok {
		t.Fatal("no old rect")
	}
	old.angle = ang
	if n := count(old); n == 0 {
		t.Error("control failed: the old page-centre box also contained all ink — the property test can't discriminate")
	} else {
		t.Logf("control: old page-centre box leaves %d ink px outside (deskew box leaves 0)", n)
	}
}

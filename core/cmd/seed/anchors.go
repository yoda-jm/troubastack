package main

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
)

// B13 — anchored placement. Demo annotations are positioned from the layout manifests emitted
// by mkcharts (generated charts) and hand-calibrated manifests for engraved PDFs, not from
// hand-tuned magic numbers. Every anchor-bound object records the target box it must cover, so
// the containment test can prove a highlight actually sits over its word/note.

// anchorBox is one recorded run (or a calibrated musical target) in [0,1]² page coords.
type anchorBox struct {
	Key  string  `json:"key,omitempty"`  // engraved manifests name their targets
	Desc string  `json:"desc,omitempty"` // engraved: what the box points at, in words
	Page int     `json:"page"`
	Text string  `json:"text,omitempty"`
	X0   float64 `json:"x0"`
	Y0   float64 `json:"y0"`
	X1   float64 `json:"x1"`
	Y1   float64 `json:"y1"`
}

func (a anchorBox) w() float64  { return a.X1 - a.X0 }
func (a anchorBox) h() float64  { return a.Y1 - a.Y0 }
func (a anchorBox) cx() float64 { return (a.X0 + a.X1) / 2 }
func (a anchorBox) cy() float64 { return (a.Y0 + a.Y1) / 2 }

// anchorSet is the manifest for one chart file.
type anchorSet struct {
	name  string
	boxes []anchorBox
}

// loadAnchors reads a <name>.anchors.json manifest. Path is resolved like the demo PDFs
// (relative to the seed's cwd — docs/demo-charts).
func loadAnchors(path string) (*anchorSet, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var boxes []anchorBox
	if err := json.Unmarshal(b, &boxes); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &anchorSet{name: path, boxes: boxes}, nil
}

// subOf returns the sub-box of substr within run `a` (occ: 1-based; -1 = last). Exact for
// monospace runs (proportional char offset); close for proportional fonts (callers pad).
func subOf(a anchorBox, substr string, occ int) anchorBox {
	idx := -1
	if occ < 0 {
		idx = strings.LastIndex(a.Text, substr)
	} else {
		seen, from := 0, 0
		for {
			i := strings.Index(a.Text[from:], substr)
			if i < 0 {
				break
			}
			seen++
			idx = from + i
			if seen == occ {
				break
			}
			from = idx + 1
		}
	}
	if idx < 0 {
		panic(fmt.Sprintf("subOf: %q not in run %q", substr, a.Text))
	}
	runes := float64(len(a.Text))
	f0 := float64(idx) / runes
	f1 := float64(idx+len(substr)) / runes
	return anchorBox{Page: a.Page, Text: substr, X0: a.X0 + f0*a.w(), Y0: a.Y0, X1: a.X0 + f1*a.w(), Y1: a.Y1}
}

// run returns the full box of the nth (1-based) run containing substr.
func (s *anchorSet) run(substr string, n int) anchorBox {
	seen := 0
	for _, a := range s.boxes {
		if a.Text == "" || !strings.Contains(a.Text, substr) {
			continue
		}
		seen++
		if seen == n {
			return a
		}
	}
	panic(fmt.Sprintf("anchor %q: no run #%d containing %q", s.name, n, substr))
}

// runNear returns the full box of the run on `page` containing substr whose vertical centre is
// nearest y — robust when the same chord/word appears on several lines.
func (s *anchorSet) runNear(page int, y float64, substr string) anchorBox {
	best := -1
	bestD := math.MaxFloat64
	for i, a := range s.boxes {
		if a.Page != page || a.Text == "" || !strings.Contains(a.Text, substr) {
			continue
		}
		if d := math.Abs(a.cy() - y); d < bestD {
			bestD, best = d, i
		}
	}
	if best < 0 {
		panic(fmt.Sprintf("anchor %q: no run near y%.3f containing %q", s.name, y, substr))
	}
	return s.boxes[best]
}

// text returns the sub-box of substr in the nth run containing it. Convenience for the common
// "highlight this word/line" case.
func (s *anchorSet) text(substr string, n int) anchorBox {
	return subOf(s.run(substr, n), substr, 1)
}

// boxAt returns the run on `page` whose centre is nearest (x,y) — for picking one cell out of a
// grid of identically-labelled bar cells (guitar chord charts).
func (s *anchorSet) boxAt(page int, x, y float64) anchorBox {
	best := -1
	bestD := math.MaxFloat64
	for i, a := range s.boxes {
		if a.Page != page {
			continue
		}
		if d := math.Hypot(a.cx()-x, a.cy()-y); d < bestD {
			bestD, best = d, i
		}
	}
	if best < 0 {
		panic(fmt.Sprintf("anchor %q: no run on page %d", s.name, page))
	}
	return s.boxes[best]
}

// anchorsPath resolves <base>.anchors.json from a few candidate roots so it works from both the
// seed's cwd (core → ../docs/demo-charts) and the test's cwd (core/cmd/seed).
func anchorsPath(base string) string {
	cands := []string{"../docs/demo-charts", "../../../docs/demo-charts", "../../docs/demo-charts", "docs/demo-charts"}
	for _, d := range cands {
		p := filepath.Join(d, base+".anchors.json")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return filepath.Join(cands[0], base+".anchors.json")
}

// mustAnchors loads a chart's manifest or panics (a missing manifest is a build bug).
func mustAnchors(base string) *anchorSet {
	s, err := loadAnchors(anchorsPath(base))
	if err != nil {
		panic(err)
	}
	return s
}

// key returns the named engraved-manifest target.
func (s *anchorSet) key(k string) anchorBox {
	for _, a := range s.boxes {
		if a.Key == k {
			return a
		}
	}
	panic(fmt.Sprintf("anchor %q: no key %q", s.name, k))
}

// ---- deterministic hand-drawn stroke geometry (B13 §2) -------------------
//
// Real markup is never ruler-straight. handStroke/handRing add tilt, wobble and overshoot,
// with jitter from a PRNG seeded by the object key — so a re-seed is byte-identical and the
// containment tolerances stay stable.

func keyRand(key string) *rand.Rand {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	return rand.New(rand.NewSource(int64(h.Sum64())))
}

// jit returns a value in [-a,a] from r.
func jit(r *rand.Rand, a float64) float64 { return (r.Float64()*2 - 1) * a }

// handStroke returns a wobbly, slightly-tilted highlighter swipe covering box `t` horizontally
// at its vertical centre. Overshoots the ends, hooks slightly at lift-off, never axis-parallel.
func handStroke(key string, t anchorBox) []wirePoint {
	r := keyRand(key)
	const (
		over0 = 0.004 // start before the target
		over1 = 0.006 // end after it
	)
	x0 := t.X0 - over0
	x1 := t.X1 + over1
	cy := t.cy()
	tilt := (0.003 + r.Float64()*0.005) // 0.3–0.8% of page height
	if r.Float64() < 0.5 {
		tilt = -tilt
	}
	n := 11 + r.Intn(6) // 11–16 points
	pts := make([]wirePoint, 0, n)
	for i := 0; i < n; i++ {
		f := float64(i) / float64(n-1)
		x := x0 + f*(x1-x0)
		// base line: linear tilt across the swipe
		y := cy + (f-0.5)*tilt
		// wobble: a couple of gentle bumps
		y += math.Sin(f*math.Pi*float64(2+r.Intn(2))) * (0.0015 + r.Float64()*0.0015)
		y += jit(r, 0.0004)
		pts = append(pts, wirePoint{X: x, Y: y})
	}
	// hook at lift-off: curl the last point slightly off-axis
	last := &pts[len(pts)-1]
	last.Y += (0.003) * signOf(tilt)
	return pts
}

// handRing returns a wobbly closed loop around box `t` (an organic circle/oval). Ends overlap
// by ~10% of the arc rather than meeting exactly.
func handRing(key string, t anchorBox) []wirePoint {
	r := keyRand(key)
	cx, cy := t.cx(), t.cy()
	rx := t.w()/2 + 0.006 + r.Float64()*0.004
	ry := t.h()/2 + 0.006 + r.Float64()*0.004
	rot := jit(r, 0.15) // slight tilt of the whole oval
	const n = 22
	pts := make([]wirePoint, 0, n+3)
	// start a touch past 0 and end past 2π so the ends overlap (~10% arc)
	start := -0.2
	end := 2*math.Pi + 0.4
	for i := 0; i <= n+2; i++ {
		f := float64(i) / float64(n)
		ang := start + f*(end-start)
		wob := 1 + jit(r, 0.06)
		px := cx + math.Cos(ang)*rx*wob
		py := cy + math.Sin(ang)*ry*wob
		// rotate slightly about the centre
		dx, dy := px-cx, py-cy
		px = cx + dx*math.Cos(rot) - dy*math.Sin(rot)
		py = cy + dx*math.Sin(rot) + dy*math.Cos(rot)
		pts = append(pts, wirePoint{X: px, Y: py})
	}
	return pts
}

func signOf(v float64) float64 {
	if v < 0 {
		return -1
	}
	return 1
}

// handBracket returns a wobbly left "[" spanning yTop..yBot at x, with slightly uneven arms.
func handBracket(key string, x, yTop, yBot float64) []wirePoint {
	r := keyRand(key)
	const tick = 0.013
	jx := func() float64 { return jit(r, 0.0009) }
	jy := func() float64 { return jit(r, 0.0011) }
	return []wirePoint{
		{X: x + tick + jx(), Y: yTop + jy()},
		{X: x + jx(), Y: yTop + jy()},
		{X: x + jx() - 0.001, Y: (yTop + yBot) / 2},
		{X: x + jx(), Y: yBot + jy()},
		{X: x + tick + jx(), Y: yBot + jy()},
	}
}

// downBowMark returns a small hand-drawn down-bow (⊓) centred over box t, above it.
func downBowMark(key string, t anchorBox) []wirePoint {
	r := keyRand(key)
	cx := t.cx()
	y := t.Y0 - 0.006
	w, h := 0.007, 0.008
	j := func() float64 { return jit(r, 0.0006) }
	return []wirePoint{
		{X: cx - w + j(), Y: y + j()}, {X: cx - w + j(), Y: y - h + j()},
		{X: cx + w + j(), Y: y - h + j()}, {X: cx + w + j(), Y: y + j()},
	}
}

// upBowMark returns a small hand-drawn up-bow (V) centred over box t, above it.
func upBowMark(key string, t anchorBox) []wirePoint {
	r := keyRand(key)
	cx := t.cx()
	y := t.Y0 - 0.006
	w, h := 0.007, 0.009
	j := func() float64 { return jit(r, 0.0006) }
	return []wirePoint{
		{X: cx - w + j(), Y: y - h + j()}, {X: cx + j(), Y: y + j()}, {X: cx + w + j(), Y: y - h + j()},
	}
}

// union returns the bounding box enclosing all given boxes (same page).
func union(boxes ...anchorBox) anchorBox {
	u := boxes[0]
	for _, a := range boxes[1:] {
		if a.X0 < u.X0 {
			u.X0 = a.X0
		}
		if a.Y0 < u.Y0 {
			u.Y0 = a.Y0
		}
		if a.X1 > u.X1 {
			u.X1 = a.X1
		}
		if a.Y1 > u.Y1 {
			u.Y1 = a.Y1
		}
	}
	return u
}

// ---- anchor-bound builder helpers + placement registry -------------------
//
// The registry records, per object uuid, the target box it must cover, so the containment
// test can verify placement independently of the drawing code.

type placement struct {
	uuid string
	kind string // "cover" (highlight/rect/ring must contain target) | "clear" (label/stamp)
	page int
	tgt  anchorBox
}

// highlightAnchor lays a hand-drawn highlighter swipe over target `t`, width sized from the
// target height, multiply blend so the print reads through. Registers a "cover" placement.
func (b *builderCtx) highlightAnchor(layerID, key string, t anchorBox, color string, opacity float64) {
	pts := handStroke(key, t)
	st := wireStyle{Color: color, Opacity: opacity, Width: t.h() * 1.15, Blend: "multiply"}
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "freehand",
		Points: pts, Page: t.Page, Style: st,
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "cover", page: t.Page, tgt: t})
}

// highlightTypeAnchor emits a real TypeHighlight object (two-corner filled multiply band) over
// target `t`, so the demo exercises the baker's dedicated highlight path (fill+multiply, no
// stroke). Unlike highlightAnchor (a hand-drawn freehand swipe), this shows the highlight TOOL.
// "cover" placement — the band must sit over real print.
func (b *builderCtx) highlightTypeAnchor(layerID, key string, t anchorBox, color string, opacity float64) {
	pad := t.h() * 0.14
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "highlight",
		Points: []wirePoint{{X: t.X0 - pad, Y: t.Y0 - pad}, {X: t.X1 + pad, Y: t.Y1 + pad}}, Page: t.Page,
		Style: wireStyle{Color: color, Opacity: opacity, Blend: "multiply"},
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "cover", page: t.Page, tgt: t})
}

// ringAnchor draws a hand-drawn ring around target `t` (freehand handRing). "cover" placement.
func (b *builderCtx) ringAnchor(layerID, key string, t anchorBox, st wireStyle) {
	pts := handRing(key, t)
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "freehand",
		Points: pts, Page: t.Page, Style: st,
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "cover", page: t.Page, tgt: t})
}

// rectAnchor draws a filled multiply rect around target `t` (+pad), e.g. a verse box. "cover".
func (b *builderCtx) rectAnchor(layerID, key string, t anchorBox, pad float64, color string, opacity float64) {
	x0, y0, x1, y1 := t.X0-pad, t.Y0-pad, t.X1+pad, t.Y1+pad
	yes := true
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "rect",
		Points: []wirePoint{{X: x0, Y: y0}, {X: x1, Y: y1}}, Page: t.Page,
		Style: wireStyle{Color: color, Opacity: opacity, Fill: &yes, Blend: "multiply"},
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "cover", page: t.Page, tgt: t})
}

// ellipseAnchor rings target `t` with the ellipse TOOL (showcases the type), bbox = t+pad.
// "cover" placement.
func (b *builderCtx) ellipseAnchor(layerID, key string, t anchorBox, pad float64, st wireStyle) {
	x0, y0, x1, y1 := t.X0-pad, t.Y0-pad, t.X1+pad, t.Y1+pad
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "ellipse",
		Points: []wirePoint{{X: x0, Y: y0}, {X: x1, Y: y1}}, Page: t.Page, Style: st,
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "cover", page: t.Page, tgt: t})
}

// labelNear places a text label at (x,y) on the target's page. "clear" placement (must sit in
// genuinely empty space) — the caller picks a clear spot; the ink test verifies it.
func (b *builderCtx) labelNear(layerID, key string, page int, x, y float64, body, color string, size float64) {
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "text",
		Points: []wirePoint{{X: x, Y: y}}, Page: page, Text: body,
		Style: wireStyle{Color: color, Opacity: 1, FontSize: size},
	})
	// ink renders text with textBaseline="top" (web/ink/src/index.ts:drawText): the point is the
	// TOP-LEFT and glyphs extend DOWN by ~fontSize. The placement box must match, else the ink
	// test checks empty space above the text instead of where it actually lands.
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "clear", page: page,
		tgt: anchorBox{Page: page, X0: x, Y0: y, X1: x + float64(len(body))*size*0.6, Y1: y + size}})
}

// iconAnchor stamps a tinted glyph (T51) at (x,y) with the given square size. "clear".
func (b *builderCtx) iconAnchor(layerID, key string, page int, x, y, size float64, glyph, color string) {
	b.im.Objects = append(b.im.Objects, wireObject{
		UUID: objectID(b.songID, key), LayerID: layerID, Type: "icon",
		Points: []wirePoint{{X: x, Y: y}, {X: x + size, Y: y + size}}, Page: page,
		Text: glyph, Style: wireStyle{Color: color, Opacity: 1},
	})
	b.im.placements = append(b.im.placements, placement{uuid: objectID(b.songID, key), kind: "clear", page: page,
		tgt: anchorBox{Page: page, X0: x, Y0: y, X1: x + size, Y1: y + size}})
}

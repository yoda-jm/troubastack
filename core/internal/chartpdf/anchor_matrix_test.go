package chartpdf

// The annotation-anchoring matrix: one subtest per (shape × event × variant) cell,
// TestAnchorMatrix/<shape>/<event>/<variant>, each asserting the DESIRED property from the anchoring spec
// (P-ids in the cell table), not today's behaviour. Every (shape, event) has the variant "base" (the
// original fixture); some also carry a variant with the geometry "base" happens to avoid, so a cell cannot
// read green when the behaviour only works for one lucky fixture (see mxVariants). Cells known to be red on
// today's code are listed in knownRed, keyed shape/event/variant, and skip unless ANCHOR_RED=1
// (testutil.AnchorKnownRed).
//
//	ANCHOR_RED=1 go test ./internal/chartpdf/ -run AnchorMatrix -v     (the full picture)
//	go test ./internal/chartpdf/                                        (CI: known-red cells skip)
//
// The harness drives the PRODUCTION functions at three seams, each named SEAM below:
//   - create  → AnchorNewMark (what sync/apply.go KindCreate, via httpapi.chartAnchorer, and the import
//     path call; T193: a no-op while chartpdf.anchoringFrozen);
//   - move / resize / restyle → mirrors sync/apply.go's default branch TODAY: the client's points are
//     taken, the client's anchor and render hash ride along unchanged (no re-anchor). When T191 lands, this
//     seam must call the new re-anchor function; the sync-level cells (anchor_matrix_sync_test.go) observe
//     the real apply path;
//   - a source edit or re-layout → a fresh RenderWithAnchors + ServedObjects (what serve and bake call;
//     T193: stored coordinates, unprojected, while frozen).
//
// Fixture text is synthetic (no real song). The target line deliberately carries a typo ("lihgt") so the
// typo cells can fix it.

import (
	"crypto/sha256"
	"fmt"
	"math"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/testutil"
)

const (
	mxT     = "Leave the porch lihgt on when the sun goes down" // the target line T (typo on purpose)
	mxTLong = mxT + " and the town asleep"                      // fits at the default size, wraps at 16 pt
	mxRow   = "Em           C"                                  // T's chord row
	mxB     = "Every mile is a song we have not sung"           // last line of verse 1 (free space below)
	mxC     = "Take me where the river bends"                   // a chorus line (jump destination)
	mxV3    = mxB                                               // the line a moved mark is moved onto
)

func mxBody(target string) string {
	return "## Verse 1\n" +
		"G            D\nPack a little light for the road ahead\n" +
		mxRow + "\n" + target + "\n" +
		"G            D\n" + mxB + "\n\n" +
		"## Chorus\n" +
		"C        G\nOh the open road\n" +
		"D            G\n" + mxC + "\n\n" +
		"## Verse 2\n" +
		"G            D\nMorning finds us somewhere new\n" +
		"Em           C\nCoffee cold and the radio on\n"
}

func mxSrc(header, body string) string {
	if header != "" {
		header += "\n"
	}
	return "# The Open Road\n" + header + "\n" + body
}

func mxReplace(t *testing.T, s, old, new string) string {
	t.Helper()
	if !strings.Contains(s, old) {
		t.Fatalf("fixture edit: %q not found", old)
	}
	return strings.Replace(s, old, new, 1)
}

// ---- frames: a line's own coordinate system (characters across, line-heights down) ----

type mxFrame struct {
	page          int
	x0, y0, cw, h float64
}

func mxFrameOf(a Anchor) mxFrame {
	n := float64(len([]rune(a.Text)))
	return mxFrame{page: a.Page, x0: a.X0, y0: a.Y0, cw: (a.X1 - a.X0) / n, h: a.Y1 - a.Y0}
}

func (f mxFrame) at(ch, rel float64) domain.Point {
	return domain.Point{X: f.x0 + ch*f.cw, Y: f.y0 + rel*f.h}
}

// mxIdent names a line by its text and source-order occurrence; prefix matches a wrapped line's first
// visual segment.
type mxIdent struct {
	text   string
	occ    int
	prefix bool
}

func (w *mxWorld) frame(id mxIdent) (mxFrame, bool) {
	if !id.prefix {
		a, ok := runAt(w.anchors, id.text, id.occ)
		return mxFrameOf(a), ok
	}
	seen := 0
	for _, i := range sourceOrder(w.anchors) {
		a := w.anchors[i]
		if strings.HasPrefix(id.text, a.Text) && strings.HasPrefix(a.Text, "Leave the porch") {
			seen++
			if seen == id.occ {
				return mxFrameOf(a), true
			}
		}
	}
	return mxFrame{}, false
}

// ---- the world: one chart's current source + render ----

type mxWorld struct {
	t       *testing.T
	src     string
	anchors []Anchor
	hash    string
}

func newMxWorld(t *testing.T, src string) *mxWorld {
	w := &mxWorld{t: t}
	w.setSrc(src)
	return w
}

func (w *mxWorld) setSrc(src string) {
	w.t.Helper()
	_, a, err := RenderWithAnchors(src)
	if err != nil {
		w.t.Fatalf("render: %v", err)
	}
	w.src, w.anchors = src, a
	w.hash = fmt.Sprintf("%x", sha256.Sum256([]byte(src)))
}

func (w *mxWorld) must(id mxIdent) mxFrame {
	w.t.Helper()
	f, ok := w.frame(id)
	if !ok {
		w.t.Fatalf("fixture: line %q #%d not in the render", id.text, id.occ)
	}
	return f
}

// ---- marks ----

type mxMark struct {
	obj    domain.Object
	placed []domain.Point // where the user put it (after any move/resize)
	ref    mxIdent        // the line it should follow
	keepX  bool           // margin mark: x is page-fixed, y follows the line (P4)
	fixed  bool           // far mark: should never link (P5)
	exact  bool           // compare with an exact tolerance, not the one-line tolerance (variant midline)
}

var mxShapes = []string{"freehand", "line", "rect", "ellipse", "text", "highlight", "icon", "jump_pair"}

func loop(cx, cy, rx, ry float64, n int) []domain.Point {
	pts := make([]domain.Point, 0, n)
	for i := 0; i < n; i++ {
		a := 2 * math.Pi * float64(i) / float64(n)
		pts = append(pts, domain.Point{X: cx + rx*math.Cos(a), Y: cy + ry*math.Sin(a)})
	}
	return pts
}

func square(cx, cy, side float64) []domain.Point {
	return []domain.Point{{X: cx - side/2, Y: cy - side/2}, {X: cx + side/2, Y: cy + side/2}}
}

func mxType(shape string) domain.ObjectType {
	switch shape {
	case "freehand":
		return domain.TypeFreehand
	case "line":
		return domain.TypeLine
	case "rect":
		return domain.TypeRect
	case "ellipse":
		return domain.TypeEllipse
	case "text":
		return domain.TypeText
	case "highlight":
		return domain.TypeHighlight
	}
	return domain.TypeIcon
}

// build places the shape in the given variant on the world's current render. Geometry is realistic: a
// rectangle is drawn with a margin around its line, an ellipse rings a word, a line is an arrow drawn
// right-to-left (so a flip is visible), an icon is a stamp a little taller than the line.
//
// fix is the cell's fixture variant (see mxVariants): "base" is the geometry above; the others reshape one
// mark to the geometry "base" avoids.
func (w *mxWorld) build(shape, variant string, tID mxIdent, fix string) []mxMark {
	t := w.t
	t.Helper()
	fT := w.must(tID)
	fB := w.must(mxIdent{mxB, 1, false})
	fC := w.must(mxIdent{mxC, 1, false})
	n := float64(len([]rune(tID.text)))
	if tID.prefix {
		n = 30
	}
	mk := func(pts []domain.Point, ref mxIdent) mxMark {
		o := domain.Object{UUID: shape + "-" + variant, Type: mxType(shape), Page: w.must(ref).page,
			Points: pts, Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.003, FontSize: 0.02}}
		switch shape {
		case "text":
			o.Text = "softly"
		case "icon", "jump_pair":
			o.Text = "star"
		}
		cp := append([]domain.Point(nil), pts...)
		return mxMark{obj: o, placed: cp, ref: ref}
	}
	bID := mxIdent{mxB, 1, false}
	cID := mxIdent{mxC, 1, false}
	var out []mxMark
	switch variant {
	case "on":
		switch shape {
		case "freehand":
			c := fT.at(17.5, 0.5)
			out = append(out, mk(loop(c.X, c.Y, 8*fT.cw, 1.0*fT.h, 24), tID))
		case "line":
			out = append(out, mk([]domain.Point{fT.at(30, 0.6), fT.at(5, 0.6)}, tID))
		case "rect":
			out = append(out, mk([]domain.Point{fT.at(-2, -0.4), fT.at(n+2, 1.4)}, tID))
		case "ellipse":
			out = append(out, mk([]domain.Point{fT.at(8.5, -0.5), fT.at(16.5, 1.5)}, tID))
		case "text":
			if fix == "midline" { // mid-line height, between two character boundaries; compared exactly
				m := mk([]domain.Point{fT.at(20.5, 0.5)}, tID)
				m.exact = true
				out = append(out, m)
				break
			}
			out = append(out, mk([]domain.Point{fT.at(20, 0.1)}, tID))
		case "highlight":
			if fix == "tall" { // about 2x its line's height, centred on the line
				out = append(out, mk([]domain.Point{fT.at(5, -0.5), fT.at(20, 1.5)}, tID))
				break
			}
			out = append(out, mk([]domain.Point{fT.at(5, 0.05), fT.at(20, 0.95)}, tID))
		case "icon":
			c := fT.at(12, 0.5)
			out = append(out, mk(square(c.X, c.Y, 1.6*fT.h), tID))
		case "jump_pair":
			c := fT.at(28, 0.5)
			d := fC.at(10, 0.5)
			out = append(out, mk(square(c.X, c.Y, 1.6*fT.h), tID), mk(square(d.X, d.Y, 1.6*fC.h), cID))
		}
	case "beside": // just under the last line of verse 1, in the gap before the chorus
		switch shape {
		case "freehand":
			var p []domain.Point
			for i := 0; i <= 12; i++ {
				p = append(p, fB.at(2+float64(i)*28/12, 1.15+0.3*float64(i%2)))
			}
			out = append(out, mk(p, bID))
		case "line":
			out = append(out, mk([]domain.Point{fB.at(30, 1.3), fB.at(2, 1.3)}, bID))
		case "rect":
			out = append(out, mk([]domain.Point{fB.at(2, 1.1), fB.at(30, 1.5)}, bID))
		case "ellipse":
			out = append(out, mk([]domain.Point{fB.at(5, 1.1), fB.at(15, 1.5)}, bID))
		case "text":
			out = append(out, mk([]domain.Point{fB.at(5, 1.1)}, bID))
		case "highlight":
			out = append(out, mk([]domain.Point{fB.at(2, 1.05), fB.at(30, 1.45)}, bID))
		case "icon":
			c := fB.at(10, 1.35)
			out = append(out, mk(square(c.X, c.Y, 0.5*fB.h), bID))
		case "jump_pair":
			c := fB.at(10, 1.35)
			d := fC.at(10, 1.35)
			out = append(out, mk(square(c.X, c.Y, 0.5*fB.h), bID), mk(square(d.X, d.Y, 0.5*fC.h), cID))
		}
	case "margin": // right of every line's text, level with T (where VLL's margin icons sit)
		y := func(f mxFrame, rel float64) float64 { return f.y0 + rel*f.h }
		var m []mxMark
		switch shape {
		case "freehand":
			m = append(m, mk(loop(0.87, y(fT, 0.5), 0.03, 0.8*fT.h, 24), tID))
		case "line":
			m = append(m, mk([]domain.Point{{X: 0.85, Y: y(fT, -0.5)}, {X: 0.85, Y: y(fT, 1.5)}}, tID))
		case "rect", "ellipse":
			m = append(m, mk([]domain.Point{{X: 0.84, Y: y(fT, -0.2)}, {X: 0.90, Y: y(fT, 1.2)}}, tID))
		case "text":
			m = append(m, mk([]domain.Point{{X: 0.84, Y: y(fT, 0.1)}}, tID))
		case "highlight":
			m = append(m, mk([]domain.Point{{X: 0.84, Y: y(fT, 0)}, {X: 0.90, Y: y(fT, 1)}}, tID))
		case "icon":
			m = append(m, mk(square(0.87, y(fT, 0.5), 1.2*fT.h), tID))
		case "jump_pair":
			m = append(m, mk(square(0.87, y(fT, 0.5), 1.2*fT.h), tID), mk(square(0.87, y(fC, 0.5), 1.2*fC.h), cID))
		}
		for i := range m {
			m[i].keepX = true
		}
		out = m
	case "far": // the blank lower half of the page
		var m []mxMark
		switch shape {
		case "freehand":
			m = append(m, mk(loop(0.4, 0.68, 0.08, 0.02, 24), tID))
		case "line":
			m = append(m, mk([]domain.Point{{X: 0.5, Y: 0.68}, {X: 0.3, Y: 0.7}}, tID))
		case "rect", "ellipse", "highlight":
			m = append(m, mk([]domain.Point{{X: 0.3, Y: 0.66}, {X: 0.5, Y: 0.70}}, tID))
		case "text":
			m = append(m, mk([]domain.Point{{X: 0.3, Y: 0.66}}, tID))
		case "icon":
			m = append(m, mk(square(0.4, 0.68, 0.03), tID))
		case "jump_pair":
			m = append(m, mk(square(0.4, 0.68, 0.03), tID), mk(square(0.6, 0.68, 0.03), cID))
		}
		for i := range m {
			m[i].fixed = true
			m[i].obj.Page = 0
		}
		out = m
	case "multi": // covers T's chord row, T, the next chord row and the next lyric (mxB)
		row := w.must(mxIdent{mxRow, 1, false})
		top := row.y0 - 0.1*row.h
		bot := fB.y0 + 1.1*fB.h
		if fix == "centred_on_line" {
			// The same multi-line mark from the same top, but its centre on T's mid-line instead of in the
			// gap between two lines (where "base" lands, routing it to the offset path by luck).
			bot = 2*(fT.y0+0.5*fT.h) - top
		}
		x0, x1 := fT.at(2, 0).X, fT.at(30, 0).X
		var pts []domain.Point
		switch shape {
		case "freehand":
			pts = loop((x0+x1)/2, (top+bot)/2, (x1-x0)/2, (bot-top)/2, 24)
		case "line":
			pts = []domain.Point{{X: x1, Y: top}, {X: x0, Y: bot}}
		case "rect", "ellipse", "highlight":
			pts = []domain.Point{{X: x0, Y: top}, {X: x1, Y: bot}}
		default:
			return nil
		}
		// Rigid follow (P12, a DECISION): a multi-line mark follows the TOPMOST line it covers and never
		// stretches; if the lines inside it move apart, it is flagged rather than reshaped.
		out = append(out, mk(pts, mxIdent{mxRow, 1, false}))
	}
	if shape == "jump_pair" && len(out) == 2 {
		out[0].obj.UUID, out[1].obj.UUID = "jump-src", "jump-dst"
		out[0].obj.JumpTo = "jump-dst"
	}
	return out
}

// ---- expectations ----

func mapPt(p domain.Point, from, to mxFrame, keepX bool) domain.Point {
	x := to.x0 + (p.X-from.x0)/from.cw*to.cw
	if keepX {
		x = p.X
	}
	return domain.Point{X: x, Y: to.y0 + (p.Y-from.y0)/from.h*to.h}
}

// expected maps a mark's placed points from its line's frame in the old render to the same line's frame
// in the new one: a rigid move with the line, scaled only by a type-size change (P8, P9). A multi-line mark
// is referenced to its topmost line, so the same map applies (P12).
func expected(m mxMark, old, cur *mxWorld, oldRef, newRef mxIdent) ([]domain.Point, mxFrame) {
	f, g := old.must(oldRef), cur.must(newRef)
	out := make([]domain.Point, len(m.placed))
	for i, p := range m.placed {
		out[i] = mapPt(p, f, g, m.keepX)
	}
	return out, g
}

type mxTol struct{ x, y float64 }

func lineTol(g mxFrame) mxTol { return mxTol{x: g.cw, y: 0.25 * g.h} }

// checkGeom compares a mark to its expected points, per shape: a line and a freehand point by point IN
// ORDER (direction and stroke order matter, P10); boxes by their bounds; text by its anchor point; an icon
// by its bounds AND its size (P11).
func checkGeom(t *testing.T, shape string, got domain.Object, want []domain.Point, wantPage int, tol mxTol, keepX bool) {
	t.Helper()
	tx := tol.x
	if keepX {
		tx = 0.002
	}
	if got.Page != wantPage {
		t.Errorf("%s: on page %d, want page %d", got.UUID, got.Page, wantPage)
		return
	}
	near := func(a, b domain.Point) bool { return math.Abs(a.X-b.X) <= tx && math.Abs(a.Y-b.Y) <= tol.y }
	switch shape {
	case "line", "freehand":
		if len(got.Points) != len(want) {
			t.Errorf("%s: %d points, want %d", got.UUID, len(got.Points), len(want))
			return
		}
		for i := range want {
			if !near(got.Points[i], want[i]) {
				t.Errorf("%s: point %d at (%.4f,%.4f), want (%.4f,%.4f) ±(%.4f,%.4f)", got.UUID, i,
					got.Points[i].X, got.Points[i].Y, want[i].X, want[i].Y, tx, tol.y)
				return
			}
		}
	case "text":
		if len(got.Points) == 0 || !near(got.Points[0], want[0]) {
			t.Errorf("%s: anchor point %v, want %v", got.UUID, got.Points, want[0])
		}
	default:
		gx0, gy0, gx1, gy1 := boundsOf(got.Points)
		wx0, wy0, wx1, wy1 := boundsOf(want)
		if math.Abs(gx0-wx0) > tx || math.Abs(gx1-wx1) > tx || math.Abs(gy0-wy0) > tol.y || math.Abs(gy1-wy1) > tol.y {
			t.Errorf("%s: box x[%.4f,%.4f] y[%.4f,%.4f], want x[%.4f,%.4f] y[%.4f,%.4f] ±(%.4f,%.4f)", got.UUID,
				gx0, gx1, gy0, gy1, wx0, wx1, wy0, wy1, tx, tol.y)
			return
		}
		if shape == "icon" || shape == "jump_pair" {
			gs, ws := math.Min(gx1-gx0, gy1-gy0), math.Min(wx1-wx0, wy1-wy0)
			if math.Abs(gs-ws) > 0.1*ws {
				t.Errorf("%s: icon drawn at size %.4f, want %.4f ±10%%", got.UUID, gs, ws)
			}
		}
	}
}

// status is what T192 would show, computed from today's data: fixed (no link), linked (a link resolved on
// the current render), lost (a link whose text is gone: its cache stays stale).
func status(o domain.Object, w *mxWorld) string {
	switch {
	case o.Anchor == nil:
		return "fixed"
	case o.PointsRenderHash == w.hash:
		return "linked"
	}
	return "lost"
}

// ---- events ----

type mxEvent struct {
	name   string
	props  string
	vari   string                              // fixture variant
	base   func(t *testing.T) string           // source before; nil = the default chart
	target mxIdent                             // the line T as named in base
	pre    string                              // "", "move", "resize", "restyle"
	edit   func(t *testing.T, s string) string // the source change; nil = create-only cell
	after  func(id mxIdent) mxIdent            // how the followed line is named afterwards
	expect string                              // linked | fixed | follow | exact | lost | transpose
	multi  bool                                // only for shapes with a multi-line form
}

var tID = mxIdent{mxT, 1, false}

func insertAbove(t *testing.T, s string) string {
	return mxReplace(t, s, "## Verse 1\n", "## Verse 1\nHold the map up to the light\n")
}

func mxHeader(h string) func(t *testing.T, s string) string {
	return func(t *testing.T, s string) string {
		return mxReplace(t, s, "# The Open Road\n", "# The Open Road\n"+h+"\n")
	}
}

var mxEvents = []mxEvent{
	{name: "create_on_run", props: "P2", vari: "on", expect: "linked"},
	{name: "create_beside_run", props: "P3", vari: "beside", expect: "linked"},
	{name: "create_margin", props: "P4", vari: "margin", expect: "linked"},
	{name: "create_far", props: "P5", vari: "far", expect: "fixed"},
	{name: "move_then_edit", props: "P23 P8", vari: "on", pre: "move", edit: insertAbove, expect: "follow"},
	{name: "resize_then_edit", props: "P24 P8", vari: "on", pre: "resize", edit: insertAbove, expect: "follow"},
	{name: "restyle_then_edit", props: "P25 P8", vari: "on", pre: "restyle", edit: insertAbove, expect: "follow"},
	{name: "edit_unrelated_line", props: "P13", vari: "on", expect: "exact",
		edit: func(t *testing.T, s string) string {
			return mxReplace(t, s, "Pack a little light", "Pack a little lamp")
		}},
	{name: "insert_line_above", props: "P8", vari: "on", edit: insertAbove, expect: "follow"},
	{name: "delete_line_above", props: "P8", vari: "on", expect: "follow",
		edit: func(t *testing.T, s string) string {
			return mxReplace(t, s, "G            D\nPack a little light for the road ahead\n", "")
		}},
	{name: "section_added_above", props: "P8", vari: "on", expect: "follow",
		edit: func(t *testing.T, s string) string {
			return mxReplace(t, s, "## Verse 1\n", "## Intro\nG  D  G  D\n\n## Verse 1\n")
		}},
	{name: "typo_linked_line", props: "P19", vari: "on", expect: "follow",
		edit:  func(t *testing.T, s string) string { return mxReplace(t, s, "lihgt", "light") },
		after: func(id mxIdent) mxIdent { id.text = strings.Replace(id.text, "lihgt", "light", 1); return id }},
	{name: "typo_then_edit", props: "P19 P8", vari: "on", expect: "follow",
		edit: func(t *testing.T, s string) string {
			return insertAbove(t, mxReplace(t, s, "lihgt", "light"))
		},
		after: func(id mxIdent) mxIdent { id.text = strings.Replace(id.text, "lihgt", "light", 1); return id }},
	{name: "delete_linked_line", props: "P20", vari: "on", expect: "lost",
		edit: func(t *testing.T, s string) string { return mxReplace(t, s, mxRow+"\n"+mxT+"\n", "") }},
	{name: "duplicate_inserted_before", props: "P14", vari: "on", expect: "follow",
		edit: func(t *testing.T, s string) string {
			return mxReplace(t, s, "## Verse 1\n", "## Verse 1\n"+mxRow+"\n"+mxT+"\n")
		},
		after: func(id mxIdent) mxIdent { id.occ++; return id }},
	{name: "multiline_reflow", props: "P6 P7 P8", vari: "multi", edit: insertAbove, expect: "follow", multi: true},
	{name: "multiline_insert_inside", props: "P12", vari: "multi", expect: "follow", multi: true,
		edit: func(t *testing.T, s string) string {
			return mxReplace(t, s, mxT+"\n", mxT+"\nAnd the dog asleep beside the door\n")
		}},
	{name: "size_change", props: "P9", vari: "on", edit: mxHeader("size: 16"), expect: "follow"},
	{name: "fit_change", props: "P9", vari: "on", edit: mxHeader("fit: page"), expect: "follow"},
	{name: "columns_change", props: "P16 P15", vari: "on", edit: mxHeader("columns: 2"), expect: "follow",
		// two columns narrow the measure, so T wraps: follow its first visual segment
		after: func(id mxIdent) mxIdent { id.prefix = true; return id }},
	{name: "size_change_wrapped_line", props: "P15", vari: "on", edit: mxHeader("size: 16"), expect: "follow",
		base:   func(t *testing.T) string { return mxSrc("", mxBody(mxTLong)) },
		target: mxIdent{mxTLong, 1, false},
		after:  func(id mxIdent) mxIdent { return mxIdent{mxTLong, 1, true} }},
	{name: "transpose_bake", props: "P18", vari: "on", expect: "transpose"},
	{name: "page_count_shrink", props: "P17 P29", vari: "on", expect: "follow",
		base: func(t *testing.T) string {
			return mxReplace(t, mxSrc("", mxBody(mxT)), mxRow+"\n"+mxT, "{np}\n"+mxRow+"\n"+mxT)
		},
		edit: func(t *testing.T, s string) string { return mxReplace(t, s, "{np}\n", "") }},
	{name: "beside_then_edit", props: "P3 P8", vari: "beside", edit: insertAbove, expect: "follow"},
	{name: "margin_then_edit", props: "P4 P8", vari: "margin", edit: insertAbove, expect: "follow"},
	{name: "far_then_edit", props: "P5", vari: "far", edit: insertAbove, expect: "exact"},
}

// mxVariants lists the fixture variants of a (shape, event) cell. "base" is always first. The others
// exercise the geometry that made a "base" cell green by fixture, not by design (red-first-baseline.md,
// "Read these GREENs carefully"):
//   - highlight "tall": a highlight about 2x its line's height, centred on it ("base" is drawn exactly over
//     the line box, so a re-fit to the line box moves it by less than the tolerance);
//   - text "midline": the label at mid-line height and between character boundaries, compared with an
//     exact x+y tolerance ("base" sits 0.1 line below the top on a boundary, inside the one-line
//     tolerance of a snap to the line top);
//   - multiline_* "centred_on_line" (freehand, rect, ellipse, highlight): the multi-line mark with its
//     centre on a line ("base" centres in the gap between two lines).
func mxVariants(shape string, ev mxEvent) []string {
	v := []string{"base"}
	switch {
	case ev.vari == "on" && ev.edit != nil && shape == "highlight":
		v = append(v, "tall")
	case ev.vari == "on" && ev.edit != nil && shape == "text":
		v = append(v, "midline")
	case ev.vari == "multi" && (shape == "freehand" || shape == "rect" || shape == "ellipse" || shape == "highlight"):
		v = append(v, "centred_on_line")
	}
	return v
}

// knownRed: the cells that fail on today's code (baseline 2026-10-11, see red-first-baseline.md), keyed
// shape/event/variant, as {properties, the work expected to fix it}. Delete a line when its fix lands; the cell then runs in CI.
// Fix labels: T191 (re-link on move/resize), T191-D3 (record the full extent + span), shape-rules (line
// direction, fixed-size stamps and text), line-identity (follow a source LINE through edits, duplicates
// and re-wrapping, not a text+occurrence), margin-link (margin marks follow the line they are level with:
// a DECISION for VLL), T193-freeze (green before the stage-0 freeze, red while it is on).
//
// T193 also flipped every edit_unrelated_line cell GREEN (P13: an edit elsewhere must leave the mark
// exactly where it was; frozen, nothing moves), so their lines were deleted: they run in CI, and stage 2
// must keep them green.
var knownRed = map[string][2]string{
	"freehand/create_margin/base":              {"P4", "margin-link"},
	"freehand/move_then_edit/base":             {"P23 P8", "T191"},
	"freehand/resize_then_edit/base":           {"P24 P8", "T191"},
	"freehand/restyle_then_edit/base":          {"P25 P8", "T191-D3"},
	"freehand/insert_line_above/base":          {"P8", "T191-D3"},
	"freehand/delete_line_above/base":          {"P8", "T191-D3"},
	"freehand/section_added_above/base":        {"P8", "T191-D3"},
	"freehand/typo_linked_line/base":           {"P19", "line-identity"},
	"freehand/typo_then_edit/base":             {"P19 P8", "line-identity"},
	"freehand/duplicate_inserted_before/base":  {"P14", "line-identity"},
	"freehand/size_change/base":                {"P9", "T191-D3"},
	"freehand/fit_change/base":                 {"P9", "T191-D3"},
	"freehand/columns_change/base":             {"P16 P15", "line-identity"},
	"freehand/size_change_wrapped_line/base":   {"P15", "line-identity"},
	"freehand/page_count_shrink/base":          {"P17 P29", "T191-D3"},
	"freehand/margin_then_edit/base":           {"P4 P8", "margin-link"},
	"line/create_margin/base":                  {"P4 P10", "margin-link"},
	"line/move_then_edit/base":                 {"P23 P8 P10", "T191"},
	"line/resize_then_edit/base":               {"P24 P8 P10", "T191"},
	"line/restyle_then_edit/base":              {"P25 P8 P10", "shape-rules"},
	"line/insert_line_above/base":              {"P8 P10", "shape-rules"},
	"line/delete_line_above/base":              {"P8 P10", "shape-rules"},
	"line/section_added_above/base":            {"P8 P10", "shape-rules"},
	"line/typo_linked_line/base":               {"P19 P10", "line-identity"},
	"line/typo_then_edit/base":                 {"P19 P8 P10", "line-identity"},
	"line/duplicate_inserted_before/base":      {"P14 P10", "line-identity"},
	"line/multiline_reflow/base":               {"P6 P7 P8 P10", "shape-rules"},
	"line/multiline_insert_inside/base":        {"P12 P10", "shape-rules"},
	"line/size_change/base":                    {"P9 P10", "shape-rules"},
	"line/fit_change/base":                     {"P9 P10", "shape-rules"},
	"line/columns_change/base":                 {"P16 P15 P10", "line-identity"},
	"line/size_change_wrapped_line/base":       {"P15 P10", "line-identity"},
	"line/page_count_shrink/base":              {"P17 P29 P10", "shape-rules"},
	"line/beside_then_edit/base":               {"P3 P8 P10", "shape-rules"},
	"line/margin_then_edit/base":               {"P4 P8 P10", "margin-link"},
	"rect/create_margin/base":                  {"P4", "margin-link"},
	"rect/move_then_edit/base":                 {"P23 P8", "T191"},
	"rect/resize_then_edit/base":               {"P24 P8", "T191"},
	"rect/restyle_then_edit/base":              {"P25 P8", "T191-D3"},
	"rect/insert_line_above/base":              {"P8", "T191-D3"},
	"rect/delete_line_above/base":              {"P8", "T191-D3"},
	"rect/section_added_above/base":            {"P8", "T191-D3"},
	"rect/typo_linked_line/base":               {"P19", "line-identity"},
	"rect/typo_then_edit/base":                 {"P19 P8", "line-identity"},
	"rect/duplicate_inserted_before/base":      {"P14", "line-identity"},
	"rect/size_change/base":                    {"P9", "T191-D3"},
	"rect/fit_change/base":                     {"P9", "T191-D3"},
	"rect/columns_change/base":                 {"P16 P15", "line-identity"},
	"rect/size_change_wrapped_line/base":       {"P15", "line-identity"},
	"rect/page_count_shrink/base":              {"P17 P29", "T191-D3"},
	"rect/margin_then_edit/base":               {"P4 P8", "margin-link"},
	"ellipse/create_margin/base":               {"P4", "margin-link"},
	"ellipse/move_then_edit/base":              {"P23 P8", "T191"},
	"ellipse/resize_then_edit/base":            {"P24 P8", "T191"},
	"ellipse/restyle_then_edit/base":           {"P25 P8", "T191-D3"},
	"ellipse/insert_line_above/base":           {"P8", "T191-D3"},
	"ellipse/delete_line_above/base":           {"P8", "T191-D3"},
	"ellipse/section_added_above/base":         {"P8", "T191-D3"},
	"ellipse/typo_linked_line/base":            {"P19", "line-identity"},
	"ellipse/typo_then_edit/base":              {"P19 P8", "line-identity"},
	"ellipse/duplicate_inserted_before/base":   {"P14", "line-identity"},
	"ellipse/size_change/base":                 {"P9", "T191-D3"},
	"ellipse/fit_change/base":                  {"P9", "T191-D3"},
	"ellipse/columns_change/base":              {"P16 P15", "line-identity"},
	"ellipse/size_change_wrapped_line/base":    {"P15", "line-identity"},
	"ellipse/page_count_shrink/base":           {"P17 P29", "T191-D3"},
	"ellipse/margin_then_edit/base":            {"P4 P8", "margin-link"},
	"text/create_margin/base":                  {"P4 P11", "margin-link"},
	"text/move_then_edit/base":                 {"P23 P8 P11", "T191"},
	"text/typo_linked_line/base":               {"P19 P11", "line-identity"},
	"text/typo_then_edit/base":                 {"P19 P8 P11", "line-identity"},
	"text/duplicate_inserted_before/base":      {"P14 P11", "line-identity"},
	"text/columns_change/base":                 {"P16 P15 P11", "line-identity"},
	"text/size_change_wrapped_line/base":       {"P15 P11", "line-identity"},
	"text/margin_then_edit/base":               {"P4 P8 P11", "margin-link"},
	"highlight/create_margin/base":             {"P4", "margin-link"},
	"highlight/move_then_edit/base":            {"P23 P8", "T191"},
	"highlight/resize_then_edit/base":          {"P24 P8", "T191"},
	"highlight/typo_linked_line/base":          {"P19", "line-identity"},
	"highlight/typo_then_edit/base":            {"P19 P8", "line-identity"},
	"highlight/duplicate_inserted_before/base": {"P14", "line-identity"},
	"highlight/columns_change/base":            {"P16 P15", "line-identity"},
	"highlight/size_change_wrapped_line/base":  {"P15", "line-identity"},
	"highlight/margin_then_edit/base":          {"P4 P8", "margin-link"},
	"icon/create_margin/base":                  {"P4 P11", "margin-link"},
	"icon/move_then_edit/base":                 {"P23 P8 P11", "T191"},
	"icon/resize_then_edit/base":               {"P24 P8 P11", "T191"},
	"icon/restyle_then_edit/base":              {"P25 P8 P11", "shape-rules"},
	"icon/insert_line_above/base":              {"P8 P11", "shape-rules"},
	"icon/delete_line_above/base":              {"P8 P11", "shape-rules"},
	"icon/section_added_above/base":            {"P8 P11", "shape-rules"},
	"icon/typo_linked_line/base":               {"P19 P11", "line-identity"},
	"icon/typo_then_edit/base":                 {"P19 P8 P11", "line-identity"},
	"icon/duplicate_inserted_before/base":      {"P14 P11", "line-identity"},
	"icon/size_change/base":                    {"P9 P11", "shape-rules"},
	"icon/fit_change/base":                     {"P9 P11", "shape-rules"},
	"icon/columns_change/base":                 {"P16 P15 P11", "line-identity"},
	"icon/size_change_wrapped_line/base":       {"P15 P11", "line-identity"},
	"icon/page_count_shrink/base":              {"P17 P29 P11", "shape-rules"},
	"icon/beside_then_edit/base":               {"P3 P8 P11", "shape-rules"},
	"icon/margin_then_edit/base":               {"P4 P8 P11", "margin-link"},
	"jump_pair/create_margin/base":             {"P4 P11 P28", "margin-link"},
	"jump_pair/move_then_edit/base":            {"P23 P8 P11 P28", "T191"},
	"jump_pair/resize_then_edit/base":          {"P24 P8 P11 P28", "T191"},
	"jump_pair/restyle_then_edit/base":         {"P25 P8 P11 P28", "shape-rules"},
	"jump_pair/insert_line_above/base":         {"P8 P11 P28", "shape-rules"},
	"jump_pair/delete_line_above/base":         {"P8 P11 P28", "shape-rules"},
	"jump_pair/section_added_above/base":       {"P8 P11 P28", "shape-rules"},
	"jump_pair/typo_linked_line/base":          {"P19 P11 P28", "line-identity"},
	"jump_pair/typo_then_edit/base":            {"P19 P8 P11 P28", "line-identity"},
	"jump_pair/delete_linked_line/base":        {"P20 P11 P28", "shape-rules"},
	"jump_pair/duplicate_inserted_before/base": {"P14 P11 P28", "line-identity"},
	"jump_pair/size_change/base":               {"P9 P11 P28", "shape-rules"},
	"jump_pair/fit_change/base":                {"P9 P11 P28", "shape-rules"},
	"jump_pair/columns_change/base":            {"P16 P15 P11 P28", "line-identity"},
	"jump_pair/size_change_wrapped_line/base":  {"P15 P11 P28", "line-identity"},
	"jump_pair/page_count_shrink/base":         {"P17 P29 P11 P28", "shape-rules"},
	"jump_pair/beside_then_edit/base":          {"P3 P8 P11 P28", "shape-rules"},
	"jump_pair/margin_then_edit/base":          {"P4 P8 P11 P28", "margin-link"},
	// The variants that break a lucky "base" green (mxVariants), red on today's code:
	"freehand/multiline_reflow/centred_on_line":         {"P6 P7 P8", "T191-D3"},
	"freehand/multiline_insert_inside/centred_on_line":  {"P12", "T191-D3"},
	"rect/multiline_reflow/centred_on_line":             {"P6 P7 P8", "T191-D3"},
	"rect/multiline_insert_inside/centred_on_line":      {"P12", "T191-D3"},
	"ellipse/multiline_reflow/centred_on_line":          {"P6 P7 P8", "T191-D3"},
	"ellipse/multiline_insert_inside/centred_on_line":   {"P12", "T191-D3"},
	"text/move_then_edit/midline":                       {"P23 P8 P11", "T191"},
	"text/resize_then_edit/midline":                     {"P24 P8 P11", "shape-rules"},
	"text/restyle_then_edit/midline":                    {"P25 P8 P11", "shape-rules"},
	"text/insert_line_above/midline":                    {"P8 P11", "shape-rules"},
	"text/delete_line_above/midline":                    {"P8 P11", "shape-rules"},
	"text/section_added_above/midline":                  {"P8 P11", "shape-rules"},
	"text/typo_linked_line/midline":                     {"P19 P11", "line-identity"},
	"text/typo_then_edit/midline":                       {"P19 P8 P11", "line-identity"},
	"text/duplicate_inserted_before/midline":            {"P14 P11", "line-identity"},
	"text/size_change/midline":                          {"P9 P11", "shape-rules"},
	"text/fit_change/midline":                           {"P9 P11", "shape-rules"},
	"text/columns_change/midline":                       {"P16 P15 P11", "line-identity"},
	"text/size_change_wrapped_line/midline":             {"P15 P11", "line-identity"},
	"text/page_count_shrink/midline":                    {"P17 P29 P11", "shape-rules"},
	"highlight/move_then_edit/tall":                     {"P23 P8", "T191"},
	"highlight/resize_then_edit/tall":                   {"P24 P8", "T191"},
	"highlight/restyle_then_edit/tall":                  {"P25 P8", "T191-D3"},
	"highlight/insert_line_above/tall":                  {"P8", "T191-D3"},
	"highlight/delete_line_above/tall":                  {"P8", "T191-D3"},
	"highlight/section_added_above/tall":                {"P8", "T191-D3"},
	"highlight/typo_linked_line/tall":                   {"P19", "line-identity"},
	"highlight/typo_then_edit/tall":                     {"P19 P8", "line-identity"},
	"highlight/duplicate_inserted_before/tall":          {"P14", "line-identity"},
	"highlight/multiline_reflow/centred_on_line":        {"P6 P7 P8", "T191-D3"},
	"highlight/multiline_insert_inside/centred_on_line": {"P12", "T191-D3"},
	"highlight/size_change/tall":                        {"P9", "T191-D3"},
	"highlight/fit_change/tall":                         {"P9", "T191-D3"},
	"highlight/columns_change/tall":                     {"P16 P15", "line-identity"},
	"highlight/size_change_wrapped_line/tall":           {"P15", "line-identity"},
	"highlight/page_count_shrink/tall":                  {"P17 P29", "T191-D3"},
	// T193-freeze: green before the anchoring freeze, red under it — a new mark is not linked, and a mark
	// no longer follows its line after an edit, until stage 2 (rigid follow) lands. Delete with stage 2.
	"freehand/create_on_run/base":            {"P2", "T193-freeze"},
	"freehand/create_beside_run/base":        {"P3", "T193-freeze"},
	"freehand/delete_linked_line/base":       {"P20", "T193-freeze"},
	"freehand/multiline_reflow/base":         {"P6 P7 P8", "T193-freeze"},
	"freehand/multiline_insert_inside/base":  {"P12", "T193-freeze"},
	"freehand/beside_then_edit/base":         {"P3 P8", "T193-freeze"},
	"line/create_on_run/base":                {"P2 P10", "T193-freeze"},
	"line/create_beside_run/base":            {"P3 P10", "T193-freeze"},
	"line/delete_linked_line/base":           {"P20 P10", "T193-freeze"},
	"rect/create_on_run/base":                {"P2", "T193-freeze"},
	"rect/create_beside_run/base":            {"P3", "T193-freeze"},
	"rect/delete_linked_line/base":           {"P20", "T193-freeze"},
	"rect/multiline_reflow/base":             {"P6 P7 P8", "T193-freeze"},
	"rect/multiline_insert_inside/base":      {"P12", "T193-freeze"},
	"rect/beside_then_edit/base":             {"P3 P8", "T193-freeze"},
	"ellipse/create_on_run/base":             {"P2", "T193-freeze"},
	"ellipse/create_beside_run/base":         {"P3", "T193-freeze"},
	"ellipse/delete_linked_line/base":        {"P20", "T193-freeze"},
	"ellipse/multiline_reflow/base":          {"P6 P7 P8", "T193-freeze"},
	"ellipse/multiline_insert_inside/base":   {"P12", "T193-freeze"},
	"ellipse/beside_then_edit/base":          {"P3 P8", "T193-freeze"},
	"text/create_on_run/base":                {"P2 P11", "T193-freeze"},
	"text/create_beside_run/base":            {"P3 P11", "T193-freeze"},
	"text/resize_then_edit/base":             {"P24 P8 P11", "T193-freeze"},
	"text/restyle_then_edit/base":            {"P25 P8 P11", "T193-freeze"},
	"text/insert_line_above/base":            {"P8 P11", "T193-freeze"},
	"text/delete_line_above/base":            {"P8 P11", "T193-freeze"},
	"text/section_added_above/base":          {"P8 P11", "T193-freeze"},
	"text/delete_linked_line/base":           {"P20 P11", "T193-freeze"},
	"text/delete_linked_line/midline":        {"P20 P11", "T193-freeze"},
	"text/size_change/base":                  {"P9 P11", "T193-freeze"},
	"text/fit_change/base":                   {"P9 P11", "T193-freeze"},
	"text/page_count_shrink/base":            {"P17 P29 P11", "T193-freeze"},
	"text/beside_then_edit/base":             {"P3 P8 P11", "T193-freeze"},
	"highlight/create_on_run/base":           {"P2", "T193-freeze"},
	"highlight/create_beside_run/base":       {"P3", "T193-freeze"},
	"highlight/restyle_then_edit/base":       {"P25 P8", "T193-freeze"},
	"highlight/insert_line_above/base":       {"P8", "T193-freeze"},
	"highlight/delete_line_above/base":       {"P8", "T193-freeze"},
	"highlight/section_added_above/base":     {"P8", "T193-freeze"},
	"highlight/delete_linked_line/base":      {"P20", "T193-freeze"},
	"highlight/delete_linked_line/tall":      {"P20", "T193-freeze"},
	"highlight/multiline_reflow/base":        {"P6 P7 P8", "T193-freeze"},
	"highlight/multiline_insert_inside/base": {"P12", "T193-freeze"},
	"highlight/size_change/base":             {"P9", "T193-freeze"},
	"highlight/fit_change/base":              {"P9", "T193-freeze"},
	"highlight/page_count_shrink/base":       {"P17 P29", "T193-freeze"},
	"highlight/beside_then_edit/base":        {"P3 P8", "T193-freeze"},
	"icon/create_on_run/base":                {"P2 P11", "T193-freeze"},
	"icon/create_beside_run/base":            {"P3 P11", "T193-freeze"},
	"icon/delete_linked_line/base":           {"P20 P11", "T193-freeze"},
	"jump_pair/create_on_run/base":           {"P2 P11 P28", "T193-freeze"},
	"jump_pair/create_beside_run/base":       {"P3 P11 P28", "T193-freeze"},
}

func TestAnchorMatrix(t *testing.T) {
	for _, shape := range mxShapes {
		for _, ev := range mxEvents {
			shape, ev := shape, ev
			if ev.multi && (shape == "icon" || shape == "text" || shape == "jump_pair") {
				continue // N/A: a stamp or a text label has no multi-line form
			}
			for _, fix := range mxVariants(shape, ev) {
				fix := fix
				name := shape + "/" + ev.name + "/" + fix
				t.Run(name, func(t *testing.T) {
					if r, ok := knownRed[name]; ok {
						testutil.AnchorKnownRed(t, r[0], r[1])
					}
					runCell(t, shape, ev, fix)
				})
			}
		}
	}
}

func runCell(t *testing.T, shape string, ev mxEvent, fix string) {
	src := mxSrc("", mxBody(mxT))
	if ev.base != nil {
		src = ev.base(t)
	}
	target := tID
	if ev.target.text != "" {
		target = ev.target
	}
	w := newMxWorld(t, src)
	if ev.name == "size_change_wrapped_line" { // precondition: the line is one run now and wraps at 16 pt
		if _, ok := runAt(w.anchors, mxTLong, 1); !ok {
			t.Fatal("fixture: the long line already wraps at the default size")
		}
	}
	if ev.name == "page_count_shrink" && w.must(target).page != 1 {
		t.Fatal("fixture: the target line is not on page 2")
	}
	marks := w.build(shape, ev.vari, target, fix)
	if len(marks) == 0 {
		t.Skip("N/A")
	}

	// SEAM create: what sync/apply.go KindCreate (via httpapi.chartAnchorer) and the import path call.
	objs := make([]domain.Object, len(marks))
	for i, m := range marks {
		objs[i] = AnchorNewMark(m.obj, w.anchors, w.hash)
	}

	switch ev.expect {
	case "linked":
		for i, m := range marks {
			wantRef := m.ref
			a := objs[i].Anchor
			if a == nil {
				t.Errorf("%s: not linked at creation, want linked to %q", objs[i].UUID, wantRef.text)
				continue
			}
			if a.RunText != wantRef.text || a.Occurrence != wantRef.occ {
				t.Errorf("%s: linked to %q #%d, want %q #%d", objs[i].UUID, a.RunText, a.Occurrence, wantRef.text, wantRef.occ)
			}
		}
		return
	case "fixed":
		for _, o := range objs {
			if o.Anchor != nil {
				t.Errorf("%s: linked to %q, want fixed (no text within reach)", o.UUID, o.Anchor.RunText)
			}
		}
		return
	}

	// SEAM move/resize/restyle: today's sync/apply.go default branch takes the client object as is — the
	// new points, the OLD anchor, the current render hash. T191 replaces this with a re-anchor.
	for i := range marks {
		switch ev.pre {
		case "move":
			from, to := w.must(marks[i].ref), w.must(mxIdent{mxV3, 1, false})
			if marks[i].ref.text == mxC { // the jump destination stays put
				continue
			}
			dy := to.y0 - from.y0
			for j := range objs[i].Points {
				objs[i].Points[j].Y += dy
				marks[i].placed[j].Y += dy
			}
			marks[i].ref = mxIdent{mxV3, 1, false}
			objs[i].PointsRenderHash = w.hash
		case "resize":
			x0, y0, _, _ := boundsOf(objs[i].Points)
			for j := range objs[i].Points {
				p := &objs[i].Points[j]
				p.X, p.Y = x0+(p.X-x0)*1.5, y0+(p.Y-y0)*1.5
				marks[i].placed[j] = *p
			}
			objs[i].PointsRenderHash = w.hash
		case "restyle":
			objs[i].Style.Color = "#2563eb"
		}
	}

	old := &mxWorld{t: t, src: w.src, anchors: w.anchors, hash: w.hash}
	if ev.expect == "transpose" {
		// The bake: marks are projected against the STORED (untransposed) render, then composited on the
		// transposed raster. P18: every mark sits on the same words of the transposed page.
		from, _ := ParseKey("G")
		to, _ := ParseKey("A")
		tr, err := TransposeToKey(w.src, "A", from, to)
		if err != nil {
			t.Fatal(err)
		}
		out, _ := ServedObjects(objs, w.anchors, w.hash) // current hash: a no-op, as in stageFile
		cur := newMxWorld(t, tr)
		for i, m := range marks {
			want, g := expected(m, old, cur, m.ref, m.ref)
			checkGeom(t, shape, out[i], want, g.page, mxTol{0.001, 0.001}, false)
		}
		return
	}

	// SEAM edit: a source change re-renders the chart; serve and bake call ServedObjects.
	w.setSrc(ev.edit(t, w.src))
	out, _ := ServedObjects(objs, w.anchors, w.hash)

	for i, m := range marks {
		o := out[i]
		expect := ev.expect
		if expect == "lost" && m.ref.text != target.text {
			expect = "follow" // a jump destination on another line: its line survives
		}
		switch expect {
		case "lost":
			if st := status(o, w); st != "lost" {
				t.Errorf("%s: status %s, want lost (its line was deleted)", o.UUID, st)
			}
			for j := range m.placed {
				if o.Points[j] != m.placed[j] {
					t.Errorf("%s: a lost mark moved (point %d %v → %v); it must stay where it was", o.UUID, j, m.placed[j], o.Points[j])
					break
				}
			}
		case "exact":
			if m.fixed {
				if o.Anchor != nil {
					t.Errorf("%s: a far mark is linked to %q", o.UUID, o.Anchor.RunText)
				}
				for j := range m.placed {
					if o.Points[j] != m.placed[j] {
						t.Errorf("%s: a fixed mark moved", o.UUID)
						break
					}
				}
				continue
			}
			want, g := expected(m, old, w, m.ref, m.ref)
			checkGeom(t, shape, o, want, g.page, mxTol{0.001, 0.001}, m.keepX)
		case "follow":
			newRef := m.ref
			if ev.after != nil && m.ref.text != mxC && m.ref.text != mxB {
				newRef = ev.after(m.ref)
			}
			if _, ok := w.frame(newRef); !ok {
				t.Fatalf("fixture: followed line %q #%d missing after the edit", newRef.text, newRef.occ)
			}
			if st := status(o, w); st != "linked" {
				t.Errorf("%s: status %s after the edit, want linked", o.UUID, st)
			}
			want, g := expected(m, old, w, m.ref, newRef)
			tol := lineTol(g)
			if m.exact {
				tol = mxTol{0.001, 0.001}
			}
			checkGeom(t, shape, o, want, g.page, tol, m.keepX)
		}
	}
	if shape == "jump_pair" && (len(out) != 2 || out[0].JumpTo != out[1].UUID) {
		t.Errorf("jump pairing lost: source JumpTo=%q", out[0].JumpTo)
	}
}

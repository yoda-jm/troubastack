package chartpdf

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// T195 (anchoring stage 2): the linking rule and the rigid projection, property by property. These replace
// the tests that pinned the old text+occurrence mechanism (critique.md §7), each rewritten to the property
// stage 2 must hold instead — the reason is on each test. They run on REAL renders through mxWorld (the
// matrix's chart: a source, its render, its line ids carried by every save).

// lw is a small chart world for one test.
func lw(t *testing.T, src string) *mxWorld { return newMxWorld(t, src) }

// lineRun is the run of the source line whose text is exactly text (its first visual segment).
func lineRun(t *testing.T, w *mxWorld, text string) Anchor {
	t.Helper()
	a, ok := runAt(w.anchors, text, 1)
	if !ok {
		t.Fatalf("fixture: %q not drawn", text)
	}
	return a
}

func idOf(w *mxWorld, a Anchor) string { return w.li.IDs[a.SrcLine] }

func rectAround(a Anchor, dy0, dy1, dx0, dx1 float64) []domain.Point {
	h, c := a.Y1-a.Y0, cw(a)
	return []domain.Point{{X: a.X0 + dx0*c, Y: a.Y0 + dy0*h}, {X: a.X0 + dx1*c, Y: a.Y0 + dy1*h}}
}

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

const verseBlock = "# T\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\nand morning finds the room\n"

// Was T172 TestUnderlineAnchorsToTheLineAbove (an underline below its line used to anchor to nothing).
// Stage 2: it links to the line above it, recorded as BELOW that line (DY > 1), with no horizontal
// relation lost.
func TestLink_UnderlineLinksToTheLineAbove(t *testing.T) {
	w := lw(t, verseBlock)
	last := lineRun(t, w, "and morning finds the room")
	h := last.Y1 - last.Y0
	o := LinkMark(domain.Object{UUID: "u", Type: domain.TypeLine, Page: last.Page,
		Points: []domain.Point{{X: last.X0 + 0.01, Y: last.Y1 + 0.25*h}, {X: last.X1 - 0.01, Y: last.Y1 + 0.25*h}}}, w.chart())
	if o.Link == nil || o.Link.State != domain.LinkLinked || o.Link.Line != idOf(w, last) {
		t.Fatalf("an underline under the last line linked %+v, want that line", o.Link)
	}
	if o.Link.DY <= 1 {
		t.Errorf("DY=%v: an underline must be recorded BELOW its line (>1 lh from its top)", o.Link.DY)
	}
}

// Was T172 TestUnderlineStaysUnderItsLineAtANewTypeSize, which pinned the old projection's box re-fit.
// Stage 2 (P9): the same chart at 11 pt then 13 pt; the underline stays under its line, proportionally,
// and its own length scales with the type (it is never re-fitted to the text).
func TestLink_UnderlineStaysUnderItsLineAtANewTypeSize(t *testing.T) {
	const body = "\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\nand morning finds the room\n"
	w := lw(t, "# T\nsize: 11"+body)
	src := lineRun(t, w, "and morning finds the room")
	h := src.Y1 - src.Y0
	o := LinkMark(domain.Object{UUID: "u", Type: domain.TypeLine, Page: src.Page,
		Points: []domain.Point{{X: src.X0 + 0.01, Y: src.Y1 + 0.25*h}, {X: src.X1 - 0.01, Y: src.Y1 + 0.25*h}}}, w.chart())
	w.setSrc("# T\nsize: 13" + body)
	dst := lineRun(t, w, "and morning finds the room")
	got := ProjectMark(o, w.chart())
	k := (dst.Y1 - dst.Y0) / h
	if got.Points[0].Y < dst.Y1 {
		t.Errorf("the underline projected ON its line (%.4f above the line's bottom %.4f)", got.Points[0].Y, dst.Y1)
	}
	relBefore := (o.Points[0].Y - src.Y0) / h
	relAfter := (got.Points[0].Y - dst.Y0) / (dst.Y1 - dst.Y0)
	if !near(relBefore, relAfter, 0.02) {
		t.Errorf("offset drifted across the size change: %.3f lh before, %.3f after", relBefore, relAfter)
	}
	lenBefore, lenAfter := o.Points[1].X-o.Points[0].X, got.Points[1].X-got.Points[0].X
	if !near(lenAfter, lenBefore*k, 1e-9) {
		t.Errorf("the underline is %.4f long, want %.4f (scaled by k=%.3f, never re-fitted)", lenAfter, lenBefore*k, k)
	}
}

// Was T172 TestRefusalIsKept. The same both-sides check on the stage-2 rule (P5): 2.5 lh below the last
// line links, 6 lh below is fixed.
func TestLink_FarIsFixed(t *testing.T) {
	w := lw(t, verseBlock)
	last := lineRun(t, w, "and morning finds the room")
	h := last.Y1 - last.Y0
	at := func(lh float64) *domain.MarkLink {
		return LinkMark(domain.Object{UUID: "f", Type: domain.TypeRect, Page: last.Page,
			Points: []domain.Point{{X: last.X0, Y: last.Y1 + lh*h}, {X: last.X0 + 0.1, Y: last.Y1 + (lh+0.1)*h}}}, w.chart()).Link
	}
	if l := at(2.5); l == nil || l.State != domain.LinkLinked {
		t.Errorf("a mark 2.5 lh below a line is %+v, want linked (inside the measured radius)", l)
	}
	if l := at(6); l == nil || l.State != domain.LinkFixed {
		t.Errorf("a mark 6 lh from any text is %+v, want fixed (P5)", l)
	}
}

// Was T172 TestNoHorizontalOverlapStaysUnanchorable, which pinned the margin REFUSAL. VLL's decision
// (P4) inverts it: a margin mark level with a line links to it, keeps its x, and follows vertically.
func TestLink_MarginMarkFollowsVertically(t *testing.T) {
	w := lw(t, verseBlock)
	line := lineRun(t, w, "beneath a paper moon")
	h := line.Y1 - line.Y0
	for name, x := range map[string][2]float64{"left_margin": {0.005, 0.03}, "right_of_text": {0.85, 0.9}} {
		o := LinkMark(domain.Object{UUID: name, Type: domain.TypeIcon, Page: line.Page,
			Points: []domain.Point{{X: x[0], Y: line.Y0 - 0.1*h}, {X: x[1], Y: line.Y1 + 0.1*h}}}, w.chart())
		if o.Link == nil || o.Link.Line != idOf(w, line) || o.Link.XRef != xrefMargin {
			t.Fatalf("%s: link %+v, want a margin link to the line it is level with", name, o.Link)
		}
		w2 := lw(t, verseBlock)
		w2.li = w.li
		w2.setSrc(strings.Replace(verseBlock, "## Verse\n", "## Verse\nan added line\n", 1))
		got := ProjectMark(o, w2.chart())
		moved := lineRun(t, w2, "beneath a paper moon")
		if got.Points[0].X != o.Points[0].X || got.Points[1].X != o.Points[1].X {
			t.Errorf("%s: a margin mark's x changed: %v → %v", name, o.Points, got.Points)
		}
		if !near(got.Points[0].Y-o.Points[0].Y, moved.Y0-line.Y0, 1e-9) {
			t.Errorf("%s: moved %.4f, its line moved %.4f", name, got.Points[0].Y-o.Points[0].Y, moved.Y0-line.Y0)
		}
	}
}

// Was T172 TestOnRunAnchorIsUnchanged, which pinned "an on-run mark projects to the run's own box" — the
// collapse. Stage 2 (P7, P8): a rectangle drawn with margins around a line keeps its OWN box, translated
// with the line, exactly.
func TestLink_OnRunMarkKeepsItsOwnBox(t *testing.T) {
	w := lw(t, verseBlock)
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "r", Type: domain.TypeRect, Page: line.Page, Points: rectAround(line, -0.4, 1.4, -2, 22)}, w.chart())
	if o.Link == nil || o.Link.Line != idOf(w, line) {
		t.Fatalf("link %+v, want the line it surrounds", o.Link)
	}
	w.setSrc(strings.Replace(verseBlock, "## Verse\n", "## Verse\nan added line\n", 1))
	moved := lineRun(t, w, "beneath a paper moon")
	got := ProjectMark(o, w.chart())
	dy := moved.Y0 - line.Y0
	for i := range o.Points {
		if !near(got.Points[i].X, o.Points[i].X, 1e-9) || !near(got.Points[i].Y, o.Points[i].Y+dy, 1e-9) {
			t.Errorf("point %d: %v, want %v translated by %.4f — never re-fitted to the line box", i, got.Points[i], o.Points[i], dy)
		}
	}
}

// Was T172 TestSpanCoversBothEnds, which pinned ELASTIC stretching to the span's last run. VLL's decision
// (P12) inverts it: a box over three lines never stretches; when a line is inserted inside it, it follows
// its top line, keeps its size, and is flagged check. An insert ABOVE it moves everything together: linked.
func TestLink_MultiLineNeverStretches(t *testing.T) {
	src := "# T\n\n## Verse\nfirst line here\nsecond line here\nthird line here\nfourth line here\n"
	w := lw(t, src)
	top, bot := lineRun(t, w, "first line here"), lineRun(t, w, "third line here")
	h := top.Y1 - top.Y0
	o := LinkMark(domain.Object{UUID: "b", Type: domain.TypeRect, Page: top.Page,
		Points: []domain.Point{{X: top.X0 - 0.01, Y: top.Y0 - 0.1*h}, {X: top.X1 + 0.01, Y: bot.Y1 + 0.1*h}}}, w.chart())
	if o.Link == nil || o.Link.Line != idOf(w, top) || len(o.Link.Peers) == 0 {
		t.Fatalf("link %+v, want the top line, watching the lines inside", o.Link)
	}
	size := func(p []domain.Point) float64 { _, y0, _, y1 := boundsOf(p); return y1 - y0 }
	for _, c := range []struct {
		name, edit, state string
	}{
		{"insert_inside", strings.Replace(src, "second line here\n", "second line here\nan inserted line\n", 1), "check"},
		{"delete_inside", strings.Replace(src, "second line here\n", "", 1), "check"},
		{"insert_above", strings.Replace(src, "## Verse\n", "## Verse\nan inserted line\n", 1), "linked"},
	} {
		w2 := lw(t, src)
		w2.li = w.li
		w2.setSrc(c.edit)
		got := ProjectMark(o, w2.chart())
		nt := lineRun(t, w2, "first line here")
		if !near(size(got.Points), size(o.Points), 1e-9) {
			t.Errorf("%s: the box is %.4f tall, was %.4f — it stretched", c.name, size(got.Points), size(o.Points))
		}
		if !near(got.Points[0].Y-o.Points[0].Y, nt.Y0-top.Y0, 1e-9) {
			t.Errorf("%s: the box did not follow its top line", c.name)
		}
		if string(got.Link.State) != c.state {
			t.Errorf("%s: state %s, want %s", c.name, got.Link.State, c.state)
		}
	}
}

// Was T146 TestAnchors_MarkKeepsItsLineAcrossTheColumnRelayout (occurrence counted in source order). The
// property survives under line ids: marks on each copy of a REPEATED chord line, made on the one-column
// render, are still on their own copy after the chart is saved in two columns.
func TestLink_MarkKeepsItsLineAcrossTheColumnRelayout(t *testing.T) {
	const pairs = 6
	w := lw(t, columnsFixture(false, pairs))
	var marks []domain.Object
	for k := 1; k <= pairs; k++ {
		a := chordAboveWaypoint(t, w.anchors, k)
		marks = append(marks, LinkMark(domain.Object{UUID: fmt.Sprint(k), Type: domain.TypeHighlight, Page: a.Page,
			Points: []domain.Point{{X: a.X0, Y: a.Y0}, {X: a.X1, Y: a.Y1}}}, w.chart()))
	}
	w.setSrc(columnsFixture(true, pairs))
	left, right := 0, 0
	for _, a := range w.anchors {
		if a.Text == repeatedChords {
			if a.X0 > 0.5 {
				right++
			} else {
				left++
			}
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("fixture does not straddle the column break (left=%d right=%d)", left, right)
	}
	for k, m := range marks {
		got := ProjectMark(m, w.chart())
		x0, y0, _, _ := boundsOf(got.Points)
		if wp := waypointBelow(w.anchors, got.Page, x0, y0); wp != waypointLyric(k+1) {
			t.Errorf("copy %d: the mark is above %q after the re-layout, want %q", k+1, wp, waypointLyric(k+1))
		}
	}
}

// Was T146 TestAnchors_OccurrenceIsLayoutIndependent and TestAnchors_SingleColumnOccurrenceUnchanged:
// occurrence numbers no longer exist. Their invariant, restated for ids (P14): each copy of a repeated
// line links to its OWN id, the same id whichever layout the mark was drawn on.
func TestLink_RepeatedLinesLinkToTheirOwnID(t *testing.T) {
	const pairs = 6
	one := lw(t, columnsFixture(false, pairs))
	two := lw(t, columnsFixture(false, pairs))
	two.li = one.li
	two.setSrc(columnsFixture(true, pairs))
	seen := map[string]bool{}
	for k := 1; k <= pairs; k++ {
		idOn := func(w *mxWorld) string {
			a := chordAboveWaypoint(t, w.anchors, k)
			l := LinkMark(domain.Object{UUID: "m", Type: domain.TypeHighlight, Page: a.Page,
				Points: []domain.Point{{X: a.X0, Y: a.Y0}, {X: a.X1, Y: a.Y1}}}, w.chart()).Link
			return l.Line
		}
		a, b := idOn(one), idOn(two)
		if a != b {
			t.Errorf("copy %d links to %s in one column, %s in two", k, a, b)
		}
		if seen[a] {
			t.Errorf("copy %d shares its id %s with another copy", k, a)
		}
		seen[a] = true
	}
}

// Was T146 TestAnchors_ProjectRefusesWhenTheRunIsGone. P20: when the linked line is deleted the mark is
// lost and stays EXACTLY where it was; it is never re-attached to a neighbour.
func TestLink_DeletedLineIsLostAndStays(t *testing.T) {
	w := lw(t, verseBlock)
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "d", Type: domain.TypeEllipse, Page: line.Page, Points: rectAround(line, -0.5, 1.5, 2, 9)}, w.chart())
	w.setSrc(strings.Replace(verseBlock, "beneath a paper moon\n", "", 1))
	got := ProjectMark(o, w.chart())
	if got.Link.State != domain.LinkLost {
		t.Errorf("state %s, want lost", got.Link.State)
	}
	for i := range o.Points {
		if got.Points[i] != o.Points[i] {
			t.Errorf("a lost mark moved: %v → %v", o.Points, got.Points)
			break
		}
	}
	if got.PointsRenderHash == w.hash {
		t.Error("a lost mark claims to be current on the new render")
	}
}

// Was T145 TestAnchorObject_And_Reproject and TestSourceAnchor_SurvivesReflow (they pinned the old re-fit to
// the run's box). P17 + P9: a type-size change reflows a line onto a later page; the mark follows it there,
// scaled by k, its own shape kept; an unlinked mark and one already measured on the render are untouched.
func TestLink_FollowsItsLineOntoAnotherPage(t *testing.T) {
	var body strings.Builder
	for _, wd := range []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "romeo"} {
		body.WriteString("C       G\nthe verse line called " + wd + "\n\n")
	}
	w := lw(t, "# Song\nsize: 8\n\n"+body.String())
	const target = "the verse line called romeo"
	a := lineRun(t, w, target)
	mark := LinkMark(domain.Object{UUID: "m", Type: domain.TypeHighlight, Page: a.Page, Points: rectAround(a, -0.2, 1.2, 4, 15)}, w.chart())
	plain := domain.Object{UUID: "n", Points: []domain.Point{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}}}
	current := LinkMark(mark, w.chart())
	w.setSrc("# Song\nsize: 16\n\n" + body.String())
	b := lineRun(t, w, target)
	if b.Page == a.Page {
		t.Fatal("fixture: the line did not change page")
	}
	out := ProjectMarks([]domain.Object{mark, plain}, w.chart())
	got := out[0]
	if got.Page != b.Page {
		t.Fatalf("projected onto page %d, want the line's new page %d", got.Page, b.Page)
	}
	k := (b.Y1 - b.Y0) / (a.Y1 - a.Y0)
	_, gy0, _, gy1 := boundsOf(got.Points)
	_, oy0, _, oy1 := boundsOf(mark.Points)
	if !near(gy1-gy0, (oy1-oy0)*k, 1e-9) || !near(gy0, b.Y0-0.2*(b.Y1-b.Y0), 0.002) {
		t.Errorf("projected y [%.4f,%.4f], want its own box scaled by k=%.2f at the line's new top", gy0, gy1, k)
	}
	if !near(got.Points[0].X, b.X0+4*cw(b), cw(b)) {
		t.Errorf("projected x %.4f, want char 4 of the line (%.4f)", got.Points[0].X, b.X0+4*cw(b))
	}
	if out[1].Points[0].X != 0.1 || out[1].Link != nil {
		t.Error("an unlinked mark must pass through untouched")
	}
	cur := current
	cur.Link = cur.Link.Clone()
	cur.Link.Render = w.hash
	if c := ProjectMark(cur, w.chart()); c.Points[0] != cur.Points[0] {
		t.Error("a mark measured on the current render must not be projected")
	}
}

// P19, the headline typo property: a word inserted BEFORE a circled word on the same line moves the
// circle right by exactly that word, and it stays linked (nothing under it changed).
func TestLink_WordInsertedBeforeCircledWordShiftsIt(t *testing.T) {
	src := "# T\n\n## Verse\nthe road goes on and on\n"
	w := lw(t, src)
	line := lineRun(t, w, "the road goes on and on")
	// "road" is chars 4..8; the ellipse rings it.
	o := LinkMark(domain.Object{UUID: "e", Type: domain.TypeEllipse, Page: line.Page, Points: rectAround(line, -0.4, 1.4, 3.6, 8.4)}, w.chart())
	w.setSrc(strings.Replace(src, "the road", "the long road", 1))
	nl := lineRun(t, w, "the long road goes on and on")
	got := ProjectMark(o, w.chart())
	wantX := nl.X0 + 9*cw(nl) - 0.4*cw(line) // "road" now starts at char 9
	if !near(got.Points[0].X, wantX, cw(nl)) {
		t.Errorf("the circle's left edge is at %.4f, want %.4f (\"road\" moved right by \"long \")", got.Points[0].X, wantX)
	}
	if got.Link.State != domain.LinkLinked {
		t.Errorf("state %s: an edit before the circled word changes nothing under it, want linked", got.Link.State)
	}
	// The typo half: an edit INSIDE the circled word keeps the follow and flags it.
	w.setSrc(strings.Replace(w.src, "long road", "long raod", 1))
	if got := ProjectMark(o, w.chart()); got.Link.State != domain.LinkCheck {
		t.Errorf("state %s after a typo under the circle, want check (P19)", got.Link.State)
	}
}

// P22: an underline in the thin gap between a lyric and the NEXT chord row is nearly level with both; it
// links to the line above and records the other as runner-up. Inserting a line between the two separates
// them: the mark follows its line and is flagged check. An insert above both leaves it linked.
func TestLink_RunnerUpSeparationIsCheck(t *testing.T) {
	src := "# T\n\n## Verse\nG     D\nthe first lyric line\nC     G\nthe second lyric line\n"
	w := lw(t, src)
	lyric, chords := lineRun(t, w, "the first lyric line"), lineRun(t, w, "C     G")
	if g := math.Abs(chords.Y0 - lyric.Y1); g > 0.3*(lyric.Y1-lyric.Y0) {
		t.Fatalf("fixture: the lyric and the next chord row are %.2f lh apart; the mark would not be ambiguous", g/(lyric.Y1-lyric.Y0))
	}
	y := (chords.Y0 + lyric.Y1) / 2
	o := LinkMark(domain.Object{UUID: "u", Type: domain.TypeLine, Page: lyric.Page,
		Points: []domain.Point{{X: lyric.X0 + 0.01, Y: y}, {X: lyric.X0 + 0.08, Y: y}}}, w.chart())
	if o.Link == nil || o.Link.Line != idOf(w, lyric) || len(o.Link.Peers) != 1 || o.Link.Peers[0].Why != "runner-up" {
		t.Fatalf("link %+v, want the lyric (the line above), the chord row as runner-up", o.Link)
	}
	for _, c := range []struct{ name, edit, state string }{
		{"separated", strings.Replace(src, "the first lyric line\n", "the first lyric line\nand a line between\n", 1), "check"},
		{"together", strings.Replace(src, "## Verse\n", "## Verse\nan intro line\n", 1), "linked"},
	} {
		w2 := lw(t, src)
		w2.li = w.li
		w2.setSrc(c.edit)
		if got := ProjectMark(o, w2.chart()); string(got.Link.State) != c.state {
			t.Errorf("%s: state %s, want %s", c.name, got.Link.State, c.state)
		}
	}
}

// P9 for a text label: its anchor point follows, and its font size scales with the type (B3).
func TestLink_TextLabelScalesItsFont(t *testing.T) {
	w := lw(t, "# T\nsize: 11\n\n## Verse\nbeneath a paper moon\n")
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "x", Type: domain.TypeText, Text: "softly", Page: line.Page,
		Points: []domain.Point{{X: line.X0 + 3.5*cw(line), Y: line.Y0 + 0.3*(line.Y1-line.Y0)}}, Style: domain.Style{FontSize: 0.02}}, w.chart())
	w.setSrc("# T\nsize: 15\n\n## Verse\nbeneath a paper moon\n")
	nl := lineRun(t, w, "beneath a paper moon")
	got := ProjectMark(o, w.chart())
	k := (nl.Y1 - nl.Y0) / (line.Y1 - line.Y0)
	if !near(got.Style.FontSize, 0.02*k, 1e-12) {
		t.Errorf("font size %.5f, want %.5f (scaled by k=%.3f)", got.Style.FontSize, 0.02*k, k)
	}
	if !near(got.Points[0].Y, nl.Y0+0.3*(nl.Y1-nl.Y0), 1e-9) {
		t.Errorf("label y %.4f, want 0.3 lh below the line's new top", got.Points[0].Y)
	}
}

// P21: a lost mark that the user moves onto another line is linked again there, and follows it.
func TestLink_LostMarkMovedRelinks(t *testing.T) {
	w := lw(t, verseBlock)
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "l", Type: domain.TypeRect, Page: line.Page, Points: rectAround(line, -0.3, 1.3, 0, 8)}, w.chart())
	w.setSrc(strings.Replace(verseBlock, "beneath a paper moon\n", "", 1))
	lost := ProjectMark(o, w.chart())
	if lost.Link.State != domain.LinkLost {
		t.Fatalf("fixture: state %s, want lost", lost.Link.State)
	}
	dst := lineRun(t, w, "and morning finds the room")
	// What the real client sends (T195 review F1): the SERVED object with new points and nothing else
	// changed — its pointsRenderHash is whatever it was served with. ProjectMark leaves a lost mark's old
	// hash (the REST layer re-stamps it; RelinkMoved must re-link either way, since a lost mark does not
	// move between renders).
	moved := lost.Clone()
	moved.Points = rectAround(dst, -0.3, 1.3, 0, 8)
	if moved.PointsRenderHash == w.hash {
		t.Fatal("fixture: the served lost mark already claims the current render; the cell would prove nothing")
	}
	re := RelinkMoved(moved, o, ptrChart(w.chart()))
	if re.Link == nil || re.Link.State != domain.LinkLinked || re.Link.Line != idOf(w, dst) {
		t.Fatalf("a lost mark moved onto a line has link %+v, want linked to it", re.Link)
	}
	w.setSrc(strings.Replace(w.src, "## Verse\n", "## Verse\nan added line\n", 1))
	nd := lineRun(t, w, "and morning finds the room")
	if got := ProjectMark(re, w.chart()); !near(got.Points[0].Y, re.Points[0].Y+(nd.Y0-dst.Y0), 1e-9) || got.Link.State != domain.LinkLinked {
		t.Errorf("the re-linked mark did not follow its new line: %v (%s)", got.Points, got.Link.State)
	}
}

// P25: a restyle of a projected mark stores the position the client saw, re-measured on the SAME line, so
// it neither moves nor changes line, and a text label's scaled font is not scaled twice.
func TestRebaseMark_KeepsTheLineAndDoesNotMove(t *testing.T) {
	w := lw(t, "# T\nsize: 11\n\n## Verse\nbeneath a paper moon\n")
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "x", Type: domain.TypeText, Text: "softly", Page: line.Page,
		Points: []domain.Point{{X: line.X0 + 3.5*cw(line), Y: line.Y0}}, Style: domain.Style{FontSize: 0.02, Color: "#000000"}}, w.chart())
	w.setSrc("# T\nsize: 15\n\n## Verse\nbeneath a paper moon\n")
	served := ProjectMark(o, w.chart())
	edited := served.Clone()
	edited.Style.Color = "#2563eb"
	edited.Link = &domain.MarkLink{State: domain.LinkLinked, Line: "forged"}
	st := RebaseMark(edited, o, ptrChart(w.chart()))
	if st.Link.Line != o.Link.Line || st.Link.Render != w.hash || st.Style.Color != "#2563eb" {
		t.Fatalf("rebased link %+v / colour %s, want the stored line on the current render, the new colour", st.Link, st.Style.Color)
	}
	if st.Points[0] != served.Points[0] || st.Style.FontSize != served.Style.FontSize {
		t.Errorf("the restyle moved or re-sized the label: %v / %.4f, served %v / %.4f", st.Points, st.Style.FontSize, served.Points, served.Style.FontSize)
	}
	if again := ProjectMark(st, w.chart()); again.Points[0] != served.Points[0] || again.Style.FontSize != served.Style.FontSize {
		t.Errorf("serving the restyled mark moved it again: %v / %.4f", again.Points, again.Style.FontSize)
	}
}

// T194's Rederived flag: a mark linked BEFORE the file's ids were re-derived is check; one linked after is
// not, even several saves later.
func TestProject_RederivedFlagMarksOlderLinksCheck(t *testing.T) {
	w := lw(t, verseBlock)
	line := lineRun(t, w, "beneath a paper moon")
	before := LinkMark(domain.Object{UUID: "b", Type: domain.TypeRect, Page: line.Page, Points: rectAround(line, -0.3, 1.3, 0, 8)}, w.chart())
	w.setSrc(strings.Replace(verseBlock, "## Verse\n", "## Verse\nan added line\n", 1))
	w.li.Rederived = &Rederivation{Reason: RederiveImportChanged, Render: w.hash}
	nl := lineRun(t, w, "beneath a paper moon")
	after := LinkMark(domain.Object{UUID: "a", Type: domain.TypeRect, Page: nl.Page, Points: rectAround(nl, -0.3, 1.3, 0, 8)}, w.chart())
	w.setSrc(strings.Replace(w.src, "## Verse\n", "## Verse\nanother line\n", 1))
	if st := ProjectMark(before, w.chart()).Link.State; st != domain.LinkCheck {
		t.Errorf("a link older than the re-derivation is %s, want check", st)
	}
	if st := ProjectMark(after, w.chart()).Link.State; st != domain.LinkLinked {
		t.Errorf("a link made after the re-derivation is %s, want linked", st)
	}
}

// ---- the T146 column fixture, kept for the rewritten column tests ----

// repeatedChords is a chord line repeated over unique lyrics: a chord/lyric pair never splits across a
// column (the T146 pair rule), so each unique lyric is a landmark for its copy.
const repeatedChords = "G     D     Am    C"

func waypointLyric(k int) string { return fmt.Sprintf("waypoint %d here", k) }

func columnsFixture(twoCol bool, pairs int) string {
	var b strings.Builder
	b.WriteString("# An Invented Chart\n")
	if twoCol {
		b.WriteString("columns: 2\n")
	}
	b.WriteString("\n## Verse\n")
	const every = 8
	for i := 0; i < pairs*every; i++ {
		if i%every == 0 {
			fmt.Fprintf(&b, "%s\n%s\n", repeatedChords, waypointLyric(i/every+1))
			continue
		}
		fmt.Fprintf(&b, "filler %02d short\n", i)
	}
	return b.String()
}

func chordAboveWaypoint(t *testing.T, anchors []Anchor, k int) Anchor {
	t.Helper()
	var wp Anchor
	for _, a := range anchors {
		if a.Text == waypointLyric(k) {
			wp = a
		}
	}
	best := Anchor{}
	for _, a := range anchors {
		if a.Text != repeatedChords || a.Page != wp.Page || a.Y0 >= wp.Y0 || math.Abs(a.X0-wp.X0) > 1e-9 {
			continue
		}
		if best.Text == "" || a.Y0 > best.Y0 {
			best = a
		}
	}
	if best.Text == "" {
		t.Fatalf("k=%d: no chord line found above waypoint %d", k, k)
	}
	return best
}

func waypointBelow(anchors []Anchor, page int, x0, y0 float64) string {
	best, bestDy := "", 0.0
	for _, a := range anchors {
		if a.Page != page || !strings.HasPrefix(a.Text, "waypoint ") || (a.X0 > 0.5) != (x0 > 0.5) {
			continue
		}
		if dy := a.Y0 - y0; dy >= 0 && (best == "" || dy < bestDy) {
			best, bestDy = a.Text, dy
		}
	}
	return best
}

// F3 (T195 review), the pure half: the reviewer's vector — a mark whose chart is saved 40 times, more than
// maxLineHistory, then edited before its word. Projected LAZILY from its first render, its character map
// is gone (check, 4 characters off). Re-based at every save (RebaseOnRender, what SaveChartSource now
// does), it lands on its word, linked. Both halves are asserted, so the test also shows why eager exists.
func TestRebaseOnRender_OutlivesTheBoundedHistory(t *testing.T) {
	src := "# T\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\n\n## Notes\nnote 0\n"
	w := lw(t, src)
	line := lineRun(t, w, "beneath a paper moon")
	lazy := LinkMark(domain.Object{UUID: "m", Type: domain.TypeIcon, Page: line.Page, Points: rectAround(line, 0, 1, 16, 18)}, w.chart())
	eager := lazy
	for i := 1; i <= 40; i++ {
		w.setSrc(strings.Replace(w.src, fmt.Sprintf("note %d\n", i-1), fmt.Sprintf("note %d\n", i), 1))
		eager, _ = RebaseOnRender(eager, w.chart())
	}
	w.setSrc(strings.Replace(w.src, "beneath a paper moon", "beneath a big paper moon", 1))
	nl := lineRun(t, w, "beneath a big paper moon")
	want := nl.X0 + 20*cw(nl)
	if got := ProjectMark(eager, w.chart()); got.Link.State != domain.LinkLinked || !near(got.Points[0].X, want, cw(nl)/4) {
		t.Errorf("re-based at every save: state %s, x %.4f, want linked at %.4f", got.Link.State, got.Points[0].X, want)
	}
	if got := ProjectMark(lazy, w.chart()); got.Link.State != domain.LinkCheck {
		t.Errorf("projected lazily past the history: state %s — the bound no longer bites, re-check why eager exists", got.Link.State)
	}
}

// Decision 4 follow-up (T195 review): a margin stamp near the paper's right edge, scaled up by a larger
// type size, stays on the page — same scaled size, slid inward — instead of being cut off.
func TestLink_ScaledMarginStampStaysOnThePage(t *testing.T) {
	w := lw(t, "# T\nsize: 9\n\n## Verse\nbeneath a paper moon\n")
	line := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "s", Type: domain.TypeIcon, Page: line.Page,
		Points: []domain.Point{{X: 0.95, Y: line.Y0}, {X: 0.995, Y: line.Y1}}}, w.chart())
	if o.Link.XRef != xrefMargin {
		t.Fatalf("fixture: %+v is not a margin link", o.Link)
	}
	w.setSrc("# T\nsize: 18\n\n## Verse\nbeneath a paper moon\n")
	got := ProjectMark(o, w.chart())
	x0, _, x1, _ := boundsOf(got.Points)
	nl := lineRun(t, w, "beneath a paper moon")
	scale := (nl.Y1 - nl.Y0) / (line.Y1 - line.Y0)
	if x1 > 1+1e-12 || x0 < 0 {
		t.Errorf("the scaled stamp spans x [%.4f, %.4f]: off the page", x0, x1)
	}
	if !near(x1-x0, 0.045*scale, 1e-9) {
		t.Errorf("the stamp is %.4f wide, want %.4f (scaled by k, never squeezed to fit)", x1-x0, 0.045*scale)
	}
}

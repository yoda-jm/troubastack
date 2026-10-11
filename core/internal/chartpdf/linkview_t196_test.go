package chartpdf

import (
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// T196 (anchoring stage 3): the serve-time link view — where a linked mark's text is on the CURRENT render
// (link.at) and what that line says (link.text). Every expectation is computed from the renderer's runs
// (the manifest), never from the link code: the dot must sit on the character the mark is linked to.

func wantAt(t *testing.T, label string, v *domain.LinkView, a Anchor, x float64) {
	t.Helper()
	if v == nil {
		t.Fatalf("%s: no link view, want one at (%.4f,%.4f) on page %d", label, x, a.Y0, a.Page)
	}
	if v.At.Page != a.Page || !near(v.At.X, x, 1e-9) || !near(v.At.Y, a.Y0, 1e-9) || !near(v.At.H, a.Y1-a.Y0, 1e-9) {
		t.Fatalf("%s: at %+v, want page %d x %.4f y %.4f h %.4f", label, v.At, a.Page, x, a.Y0, a.Y1-a.Y0)
	}
}

func served(o domain.Object, w *mxWorld) domain.Object { return ServeMark(o, w.chart()) }

// On-run: a box around a word. The dot is the top-left of the character under the mark's left edge.
func TestLinkView_OnRunIsTheLeftEdgeCharacter(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "beneath a paper moon")
	c, h := cw(a), a.Y1-a.Y0
	// "paper" starts at rune 10; the box's left edge is 0.3 of a character into it.
	o := LinkMark(domain.Object{UUID: "r", Type: domain.TypeRect, Page: a.Page,
		Points: []domain.Point{{X: a.X0 + 10.3*c, Y: a.Y0 - 0.1*h}, {X: a.X0 + 15*c, Y: a.Y1 + 0.1*h}}}, w.chart())
	got := served(o, w)
	wantAt(t, "on-run", got.View, a, a.X0+10*c)
	if got.View.Text != "beneath a paper moon" {
		t.Errorf("text %q, want the line's text", got.View.Text)
	}
}

// Beside, below: an underline under the last line links to it (DY > 1); the dot is on the line, not on
// the mark.
func TestLinkView_BesideIsOnTheLineNotTheMark(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "and morning finds the room")
	c, h := cw(a), a.Y1-a.Y0
	o := LinkMark(domain.Object{UUID: "u", Type: domain.TypeLine, Page: a.Page,
		Points: []domain.Point{{X: a.X0 + 4.5*c, Y: a.Y1 + 0.3*h}, {X: a.X0 + 11*c, Y: a.Y1 + 0.3*h}}}, w.chart())
	if o.Link == nil || o.Link.DY <= 1 {
		t.Fatalf("fixture: the underline must be linked BELOW its line, got %+v", o.Link)
	}
	got := served(o, w)
	wantAt(t, "beside (below)", got.View, a, a.X0+4*c)
	if got.View.Text != "and morning finds the room" {
		t.Errorf("text %q", got.View.Text)
	}
}

// Beside, right of a short line (xRef end): the dot is at the end of that line's text.
func TestLinkView_RightOfTheTextIsTheLineEnd(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "beneath a paper moon") // shorter than the line above it
	long := lineRun(t, w, "the kettle hums a quiet tune")
	if a.X1 >= long.X1-0.03 {
		t.Fatalf("fixture: the line must be shorter than the column's longest")
	}
	o := LinkMark(domain.Object{UUID: "i", Type: domain.TypeIcon, Page: a.Page,
		Points: []domain.Point{{X: a.X1 + 0.01, Y: a.Y0}, {X: a.X1 + 0.02, Y: a.Y1}}}, w.chart())
	if o.Link == nil || o.Link.XRef != xrefEnd {
		t.Fatalf("fixture: want an end-of-line link, got %+v", o.Link)
	}
	wantAt(t, "end", served(o, w).View, a, a.X1)
}

// Margin: a mark left of every line. No character is under it; the dot is the start of its line.
func TestLinkView_MarginIsTheNearerLineEnd(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "m", Type: domain.TypeIcon, Page: a.Page,
		Points: []domain.Point{{X: a.X0 - 0.04, Y: a.Y0}, {X: a.X0 - 0.02, Y: a.Y1}}}, w.chart())
	if o.Link == nil || o.Link.XRef != xrefMargin {
		t.Fatalf("fixture: want a margin link, got %+v", o.Link)
	}
	wantAt(t, "left margin", served(o, w).View, a, a.X0)
}

// Multi-line: a box around two lines links to the TOP one; the dot is there, at its start (the box
// starts left of the text).
func TestLinkView_MultiLineIsTheTopLine(t *testing.T) {
	w := lw(t, verseBlock)
	top := lineRun(t, w, "the kettle hums a quiet tune")
	bot := lineRun(t, w, "beneath a paper moon")
	o := LinkMark(domain.Object{UUID: "b", Type: domain.TypeRect, Page: top.Page,
		Points: []domain.Point{{X: top.X0 - 0.005, Y: top.Y0 - 0.002}, {X: top.X1, Y: bot.Y1 + 0.002}}}, w.chart())
	if o.Link == nil || o.Link.Line != idOf(w, top) {
		t.Fatalf("fixture: want the top line, got %+v", o.Link)
	}
	got := served(o, w)
	wantAt(t, "multi-line", got.View, top, top.X0)
	if got.View.Text != "the kettle hums a quiet tune" {
		t.Errorf("text %q", got.View.Text)
	}
}

// After an edit the view is computed on the NEW render, in the same projection: a word inserted before
// the circled word moves the dot by exactly that word, and the text is the edited line's.
func TestLinkView_FollowsTheEditInTheSamePass(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "beneath a paper moon")
	c, h := cw(a), a.Y1-a.Y0
	o := LinkMark(domain.Object{UUID: "r", Type: domain.TypeEllipse, Page: a.Page,
		Points: []domain.Point{{X: a.X0 + 10.2*c, Y: a.Y0 - 0.1*h}, {X: a.X0 + 15*c, Y: a.Y1 + 0.1*h}}}, w.chart())
	w.setSrc(strings.Replace(verseBlock, "## Verse\n", "## Verse\nan added line up top\n", 1))
	w.setSrc(strings.Replace(w.src, "beneath a paper moon", "beneath a white paper moon", 1))
	b := lineRun(t, w, "beneath a white paper moon")
	got := served(o, w)
	if got.Link.State != domain.LinkLinked {
		t.Fatalf("fixture: state %s, want linked", got.Link.State)
	}
	wantAt(t, "after the edit", got.View, b, b.X0+16*cw(b)) // "paper" is now rune 16
	if got.View.Text != "beneath a white paper moon" {
		t.Errorf("text %q, want the edited line", got.View.Text)
	}
	// A typo under the mark: still followed, state check, and the view is still served.
	w.setSrc(strings.Replace(w.src, "white paper moon", "white papr moon", 1))
	got = served(o, w)
	if got.Link.State != domain.LinkCheck || got.View == nil {
		t.Fatalf("typo under the mark: state %s view %v, want check with a view", got.Link.State, got.View)
	}
}

// No view: a lost mark (its line deleted), a fixed mark, an old-style mark, an unlinked mark.
func TestLinkView_AbsentWhenThereIsNoLiveLink(t *testing.T) {
	w := lw(t, verseBlock)
	a := lineRun(t, w, "beneath a paper moon")
	h := a.Y1 - a.Y0
	box := []domain.Point{{X: a.X0, Y: a.Y0 - 0.1*h}, {X: a.X0 + 0.05, Y: a.Y1 + 0.1*h}}
	linked := LinkMark(domain.Object{UUID: "l", Type: domain.TypeRect, Page: a.Page, Points: box}, w.chart())
	fixed := LinkMark(domain.Object{UUID: "f", Type: domain.TypeRect, Page: a.Page,
		Points: []domain.Point{{X: 0.4, Y: 0.9}, {X: 0.5, Y: 0.95}}}, w.chart())
	if fixed.Link == nil || fixed.Link.State != domain.LinkFixed {
		t.Fatalf("fixture: want fixed, got %+v", fixed.Link)
	}
	old := domain.Object{UUID: "o", Type: domain.TypeRect, Page: a.Page, Points: box,
		Anchor: &domain.SourceAnchor{RunText: "beneath a paper moon", Occurrence: 1}}
	plain := domain.Object{UUID: "p", Type: domain.TypeRect, Page: a.Page, Points: box}
	for _, o := range []domain.Object{fixed, old, plain} {
		if v := served(o, w).View; v != nil {
			t.Errorf("%s: view %+v, want none", o.UUID, v)
		}
	}
	if served(linked, w).View == nil {
		t.Fatalf("control: the linked mark must have a view")
	}
	w.setSrc(strings.Replace(verseBlock, "beneath a paper moon\n", "", 1))
	got := served(linked, w)
	if got.Link.State != domain.LinkLost || got.View != nil {
		t.Errorf("deleted line: state %s view %+v, want lost with no view", got.Link.State, got.View)
	}
}

// The line's text is what it DRAWS: no bold markers, and a wrapped line is one line of text.
func TestLinkView_TextIsTheDrawnLine(t *testing.T) {
	long := "a lantern swings above the quay and the tide rolls in across the stones while the gulls keep calling out"
	src := "# T\n\n## Verse\nthe **kettle** hums a quiet tune\n" + long + "\n"
	w := lw(t, src)
	var bold, wrapped []Anchor
	for _, a := range w.anchors {
		switch a.SrcLine {
		case 3:
			bold = append(bold, a)
		case 4:
			wrapped = append(wrapped, a)
		}
	}
	if len(bold) < 2 {
		t.Fatalf("fixture: the bold line must draw as several runs, got %d", len(bold))
	}
	wraps := map[int]bool{}
	for _, a := range wrapped {
		wraps[a.SrcWrap] = true
	}
	if len(wraps) < 2 {
		t.Fatalf("fixture: the long line must wrap, got %d visual lines", len(wraps))
	}
	vl := w.chart().visualLines()
	if got := lineText(linesOf(vl, w.li.IDs[3]), src); got != "the kettle hums a quiet tune" {
		t.Errorf("bold line text %q", got)
	}
	if got := lineText(linesOf(vl, w.li.IDs[4]), src); got != long {
		t.Errorf("wrapped line text %q, want %q", got, long)
	}
}

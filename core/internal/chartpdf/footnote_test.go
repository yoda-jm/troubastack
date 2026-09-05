package chartpdf

import (
	"math"
	"strings"
	"testing"

	"github.com/go-pdf/fpdf"
)

// A footnote long enough to wrap several times, at a pinned size (scale 1) so the re-measure is exact.
const wrapFootnoteChart = "# Amazing Grace\nsize: 11\n\n## Verse 1\nAmazing grace how sweet the sound\n\n" +
	"{footnote}\nWords by John Newton (1725-1807), 1779; a public-domain hymn. This attribution line is " +
	"intentionally long so that it wraps across the body column several times to exercise the wrapping path.\n"

func footnoteAnchors(t *testing.T, src string) []Anchor {
	t.Helper()
	_, anchors, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	return anchors
}

// The footnote renders as multiple wrapped lines, each with its own anchor (not one box for the block).
func TestFootnote_wrapsIntoPerLineAnchors(t *testing.T) {
	anchors := footnoteAnchors(t, wrapFootnoteChart)
	var fn []Anchor
	for _, a := range anchors {
		if strings.Contains(a.Text, "Newton") || strings.Contains(a.Text, "wraps") ||
			strings.Contains(a.Text, "attribution") || strings.Contains(a.Text, "wrapping path") {
			fn = append(fn, a)
		}
	}
	if len(fn) < 2 {
		t.Fatalf("expected the footnote to wrap into ≥2 anchored lines, got %d: %+v", len(fn), fn)
	}
	for _, a := range fn {
		if a.X1 <= a.X0 || a.Y1 <= a.Y0 || a.X0 < 0 || a.X1 > 1 {
			t.Errorf("bad footnote box: %+v", a)
		}
	}
}

// Fable's requirement 3 — the invariant on a line that ACTUALLY WRAPPED: the recorded box width equals
// the width the text renders at, re-measured by independent code. Wrapping is where a box is most
// likely to drift from its text. Teeth: mis-scale or mis-record the width and this fails.
func TestFootnote_boxContainsWrappedLineText(t *testing.T) {
	anchors := footnoteAnchors(t, wrapFootnoteChart)
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	m.SetFont("Helvetica", "I", footnotePt) // scale 1 (size: 11)
	tr := m.UnicodeTranslatorFromDescriptor("")

	wrapped := 0
	colW := right - leftMargin // T146: the body/footnote column now spans the reduced left margin
	for _, a := range anchors {
		if !(strings.Contains(a.Text, "Newton") || strings.Contains(a.Text, "wraps") ||
			strings.Contains(a.Text, "attribution") || strings.Contains(a.Text, "wrapping path")) {
			continue
		}
		// POSITION: every footnote line is drawn at the (T146-reduced) left margin — re-derived, not read from the box.
		if gotX := a.X0 * pageW; math.Abs(gotX-leftMargin) > 0.05 {
			t.Errorf("wrapped footnote line %q: box LEFT = %.3fmm, drawn at margin %.3fmm (position drift)", a.Text, gotX, leftMargin)
		}
		// SIZE: independent re-measure of the width.
		gotW := (a.X1 - a.X0) * pageW
		wantW := m.GetStringWidth(tr(a.Text))
		if math.Abs(gotW-wantW) > 0.05 {
			t.Errorf("footnote line %q: box %.3fmm wide, text renders %.3fmm", a.Text, gotW, wantW)
		}
		if gotW > colW*0.6 { // this line used most of the column ⇒ it really wrapped
			wrapped++
		}
	}
	if wrapped == 0 {
		t.Fatal("no footnote line actually wrapped — the invariant wasn't exercised on a wrapped line")
	}
}

// A GOLDEN for the wrapped footnote lines — pins the full box (incl. Y), so a VERTICAL position drift
// of a wrapped line reddens here even though the width-and-left-edge invariant above wouldn't see it.
// Teeth-checked with a +2mm y nudge to drawFootnoteLine's rec.
func TestFootnote_goldenWrappedBoxes(t *testing.T) {
	anchors := footnoteAnchors(t, wrapFootnoteChart)
	var fn []Anchor
	for _, a := range anchors {
		if strings.Contains(a.Text, "Newton") || strings.Contains(a.Text, "wraps") ||
			strings.Contains(a.Text, "attribution") || strings.Contains(a.Text, "wrapping path") {
			fn = append(fn, a)
		}
	}
	// T146: the wider body column (left margin 12→8mm) re-wraps the footnote — "the" now fits on the first
	// line — and every X shifts left by 0.0190 (X0 → 8/210 = 0.0381). Y is unchanged.
	want := []Anchor{
		{Page: 0, Text: "Words by John Newton (1725-1807), 1779; a public-domain hymn. This attribution line is intentionally long so that it wraps across the", X0: 0.0381, Y0: 0.1407, X1: 0.9263, Y1: 0.1549},
		{Page: 0, Text: "body column several times to exercise the wrapping path.", X0: 0.0381, Y0: 0.1549, X1: 0.4230, Y1: 0.1690},
	}
	if len(fn) != len(want) {
		t.Fatalf("got %d footnote lines, want %d:\n%+v", len(fn), len(want), fn)
	}
	const eps = 0.0005
	for i, w := range want {
		g := fn[i]
		if g.Text != w.Text || math.Abs(g.X0-w.X0) > eps || math.Abs(g.Y0-w.Y0) > eps ||
			math.Abs(g.X1-w.X1) > eps || math.Abs(g.Y1-w.Y1) > eps {
			t.Errorf("footnote line %d:\n got  %+v\n want %+v", i, g, w)
		}
	}
}

// **bold** is LITERAL inside a footnote (documented): the asterisks are text, so a footnote can't trip
// Part 1's per-bold-word anchoring. The whole run keeps its ** — it is not split into a bold segment.
func TestFootnote_boldIsLiteral(t *testing.T) {
	anchors := footnoteAnchors(t, "# S\nsize: 11\n\nx\n\n{footnote}\nCapo **3** on the guitar\n")
	found := false
	for _, a := range anchors {
		if strings.Contains(a.Text, "Capo") {
			found = true
			if !strings.Contains(a.Text, "**3**") {
				t.Errorf("footnote should keep ** literal, got %q", a.Text)
			}
		}
		if a.Text == "3" {
			t.Errorf("footnote **3** was parsed as a bold word (its own anchor) — must be literal")
		}
	}
	if !found {
		t.Fatal("footnote text not anchored")
	}
}

// {fn} is the alias; {fn} x and {{fn}} are NOT markers — they stay literal body text (reNewPage
// discipline). Teeth: a broken regex that matched {fn} x would swallow following lines as a footnote.
func TestFootnote_aliasAndNonMarkers(t *testing.T) {
	// {fn} alias opens a footnote.
	if got := footnoteAnchors(t, "# S\nsize: 11\n\nbody\n\n{fn}\nattribution here\n"); !hasText(got, "attribution here") {
		t.Error("{fn} alias did not open a footnote")
	}
	// {fn} x is literal body text, rendered as a normal line (not a footnote opener).
	got := footnoteAnchors(t, "# S\nsize: 11\n\n{fn} keepme\nafter\n")
	if !hasText(got, "{fn} keepme") {
		t.Errorf("`{fn} keepme` should render as literal text, anchors: %+v", got)
	}
}

// A blank line ends the footnote: it is exactly one paragraph. Content after the blank is body again.
func TestFootnote_blankLineEndsIt(t *testing.T) {
	got := footnoteAnchors(t, "# S\nsize: 11\n\n{footnote}\nthe note\n\nback to body\n")
	// "back to body" is a normal text line (its own full-height run), not part of the footnote.
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	tr := m.UnicodeTranslatorFromDescriptor("")
	var note, body *Anchor
	for i := range got {
		switch got[i].Text {
		case "the note":
			note = &got[i]
		case "back to body":
			body = &got[i]
		}
	}
	if note == nil || body == nil {
		t.Fatalf("expected both the footnote line and the body line anchored: %+v", got)
	}
	// The body line is Helvetica (6mm cell), the footnote Helvetica-I (leadFootnote): different box
	// heights prove the blank line switched back out of the footnote.
	m.SetFont("Helvetica", "", 11)
	_ = tr
	noteH := (note.Y1 - note.Y0) * pageH
	bodyH := (body.Y1 - body.Y0) * pageH
	if math.Abs(noteH-bodyH) < 0.5 {
		t.Errorf("footnote line (%.2fmm) and body line (%.2fmm) have the same box height — blank line didn't end the footnote", noteH, bodyH)
	}
}

// A footnote is part of the content T76 fits and T77 paginates: adding one increases the measured
// content height, so auto-fit sees it. (Pagination across pages is covered by the shared layout()
// path — a footnote line calls page() like any body line.)
func TestFootnote_countsTowardContentHeight(t *testing.T) {
	base := "# S\nsize: 11\n\n## V\nline one\n"
	withFn := base + "\n{footnote}\nA public-domain attribution that adds real height to the chart body.\n"
	if contentHeight(withFn) <= contentHeight(base) {
		t.Errorf("adding a footnote did not increase content height (%.2f vs %.2f) — auto-fit would ignore it",
			contentHeight(withFn), contentHeight(base))
	}
}

func hasText(anchors []Anchor, text string) bool {
	for _, a := range anchors {
		if a.Text == text {
			return true
		}
	}
	return false
}

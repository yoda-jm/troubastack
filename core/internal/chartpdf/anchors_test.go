package chartpdf

import (
	"bytes"
	"math"
	"sort"
	"testing"

	"github.com/go-pdf/fpdf"
)

// The fixture the anchor tests pin. Small but exercises every run kind: title, subtitle, section,
// a chord+lyric pair, and a text line split by **bold** (the per-word case B13 needs).
const anchorFixture = "# Amazing Grace\nfit: page\nJohn Newton, 1779\n\n## Verse 1\n" +
	"G        G7      C   G\nA-ma-zing grace, how sweet the sound\n\n" +
	"That **saved** a wretch like me\nI once was lost, but now am found\n"

// Anchors must be a pure side-channel: asking for them must not change a single byte of the PDF.
func TestAnchors_byteIdenticalToRender(t *testing.T) {
	plain, err := Render(anchorFixture)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	withA, anchors, err := RenderWithAnchors(anchorFixture)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	if !bytes.Equal(plain, withA) {
		t.Fatalf("RenderWithAnchors changed the PDF bytes (%d vs %d) — anchors must be a side channel", len(plain), len(withA))
	}
	if len(anchors) == 0 {
		t.Fatal("no anchors produced")
	}
}

// Golden: the exact anchor manifest for the fixture. Anchors that silently drift are worse than none
// (they place VLL's demo highlights, B13), so this pins every box. Teeth: nudge any advance constant
// (e.g. leadLyric) or mis-record a width and the coordinates move → this fails. Coordinates come from
// the SAME layout walk that draws, so a passing golden is also the box-contains-its-text guarantee.
func TestAnchors_golden(t *testing.T) {
	_, got, err := RenderWithAnchors(anchorFixture)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	// T146: the left margin dropped 12→8mm, so every X shifts left by (12-8)/210 = 0.0190 (X0 of a
	// left-anchored run is now 8/210 = 0.0381); every Y is unchanged — the pure left-shift the change is.
	want := []Anchor{
		{Page: 0, Text: "Amazing Grace", X0: 0.0381, Y0: 0.0404, X1: 0.3227, Y1: 0.0796},
		{Page: 0, Text: "John Newton, 1779", X0: 0.0381, Y0: 0.0796, X1: 0.2697, Y1: 0.1041},
		{Page: 0, Text: "Verse 1", X0: 0.0381, Y0: 0.1237, X1: 0.1337, Y1: 0.1530},
		{Page: 0, Text: "G        G7      C   G", X0: 0.0381, Y0: 0.1555, X1: 0.3929, Y1: 0.1800},
		{Page: 0, Text: "A-ma-zing grace, how sweet the sound", X0: 0.0381, Y0: 0.1766, X1: 0.6187, Y1: 0.2010},
		{Page: 0, Text: "That ", X0: 0.0381, Y0: 0.2148, X1: 0.0994, Y1: 0.2441},
		{Page: 0, Text: "saved", X0: 0.0994, Y0: 0.2148, X1: 0.1756, Y1: 0.2441},
		{Page: 0, Text: " a wretch like me", X0: 0.1756, Y0: 0.2148, X1: 0.3772, Y1: 0.2441},
		{Page: 0, Text: "I once was lost, but now am found", X0: 0.0381, Y0: 0.2407, X1: 0.4445, Y1: 0.2701},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d anchors, want %d:\n%+v", len(got), len(want), got)
	}
	const eps = 0.0005
	for i, w := range want {
		g := got[i]
		if g.Page != w.Page || g.Text != w.Text ||
			math.Abs(g.X0-w.X0) > eps || math.Abs(g.Y0-w.Y0) > eps ||
			math.Abs(g.X1-w.X1) > eps || math.Abs(g.Y1-w.Y1) > eps {
			t.Errorf("anchor %d:\n got  %+v\n want %+v", i, g, w)
		}
	}
}

// The invariant that matters for B13: every box actually SPANS the text it names, at the size AND the
// PLACE it was rendered — not "some boxes exist", and not "right width, wrong place". We independently
// re-derive each run's LEFT EDGE (a full-line run starts at `margin`; a `**bold**` segment starts at
// `margin` + the measured width of the segments before it on its line) and its WIDTH (fresh fpdf, same
// font), and assert both against the recorded box. Teeth: drift a recorded box's POSITION or width →
// the re-derived left edge / width no longer matches → red. (The golden pins the full box incl. Y.)
func TestAnchors_boxSpansItsTextAtItsPosition(t *testing.T) {
	_, anchors, err := RenderWithAnchors(anchorFixture)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	lines := chartLines(anchorFixture)
	subtitle, _, _, _, _, skip := parseHeader(lines)
	scale := autoFitBodyPt(lines, subtitle, skip) / defaultBodyPt

	fontOf := func(text string) (fam, style string, ptmm float64) {
		switch {
		case text == "Amazing Grace":
			return "Helvetica", "B", 16 * scale
		case text == "John Newton, 1779":
			return "Helvetica", "I", 11 * scale
		case text == "Verse 1":
			return "Helvetica", "B", 11 * scale
		case text == "G        G7      C   G":
			return "Courier", "B", 11 * scale
		case text == "A-ma-zing grace, how sweet the sound":
			return "Courier", "", 11 * scale
		case text == "saved":
			return "Helvetica", "B", 11 * scale // the **bold** word
		default:
			return "Helvetica", "", 11 * scale // plain text-line segments
		}
	}
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	tr := m.UnicodeTranslatorFromDescriptor("")
	measure := func(text string) float64 {
		fam, style, pt := fontOf(text)
		m.SetFont(fam, style, pt)
		return m.GetStringWidth(tr(text))
	}

	// Group by visual line (same Y0) and re-derive each segment's expected left edge along the line.
	const eps = 0.05 // mm
	byLine := map[int64][]Anchor{}
	var order []int64
	for _, a := range anchors {
		if a.X1 <= a.X0 || a.Y1 <= a.Y0 || a.X0 < 0 || a.Y0 < 0 || a.X1 > 1 || a.Y1 > 1 {
			t.Errorf("degenerate/off-page box for %q: %+v", a.Text, a)
			continue
		}
		key := int64(math.Round(a.Y0 * 1e6))
		if _, ok := byLine[key]; !ok {
			order = append(order, key)
		}
		byLine[key] = append(byLine[key], a)
	}
	for _, key := range order {
		line := byLine[key]
		sort.SliceStable(line, func(i, j int) bool { return line[i].X0 < line[j].X0 })
		expX := leftMargin // first run on a line starts at the (T146-reduced) left margin — re-derived, not read from the box
		for _, a := range line {
			gotX := a.X0 * pageW
			if math.Abs(gotX-expX) > eps {
				t.Errorf("box LEFT for %q = %.3fmm, but its text is drawn at %.3fmm (position drift)", a.Text, gotX, expX)
			}
			w := measure(a.Text)
			gotW := (a.X1 - a.X0) * pageW
			if math.Abs(gotW-w) > eps {
				t.Errorf("box WIDTH for %q = %.3fmm, but the text renders %.3fmm wide", a.Text, gotW, w)
			}
			expX += w // the next segment on this line begins where this one ends
		}
	}
	// One independent Y anchor: the title is drawn at the top margin. A global vertical drift moves it.
	for _, a := range anchors {
		if a.Text == "Amazing Grace" {
			if got := a.Y0 * pageH; math.Abs(got-topMargin) > eps {
				t.Errorf("title top = %.3fmm, want the top margin %.3fmm (vertical drift)", got, topMargin)
			}
		}
	}
}

// Blocker 1 (aaf7522): the chord row's performance-note run must be anchored too — a `(x2)` isn't
// something to play, but it IS drawn text, and mkcharts anchors every run. Its box must span the note
// glyphs at their real position (after the chords + the two-space lead-in).
func TestAnchors_chordAnnotationIsAnchored(t *testing.T) {
	src := "# T\nsize: 11\n\n## Verse 1\nC       G  (x2)\nsome words here\n"
	_, anchors, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	var annot *Anchor
	for i := range anchors {
		if anchors[i].Text == "(x2)" {
			annot = &anchors[i]
		}
	}
	if annot == nil {
		t.Fatalf("the performance note `(x2)` was not anchored; anchors: %+v", anchors)
	}
	// Its box spans the note at the annot font, and sits to the RIGHT of the chords (not at the margin).
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	m.SetFont("Helvetica", "I", 10) // annot font at scale 1
	tr := m.UnicodeTranslatorFromDescriptor("")
	gotW := (annot.X1 - annot.X0) * pageW
	if wantW := m.GetStringWidth(tr("(x2)")); math.Abs(gotW-wantW) > 0.05 {
		t.Errorf("annot box width = %.3fmm, text renders %.3fmm", gotW, wantW)
	}
	if annot.X0*pageW <= margin+1 {
		t.Errorf("annot left = %.3fmm, should sit after the chords, not at the margin", annot.X0*pageW)
	}
}

// The per-word case B13 leans on: a **bold** word gets its OWN box, abutting its neighbours, so a
// highlight can cover exactly that word. Teeth: if the running x weren't tracked, "saved" would start
// at the left margin like "That " and this fails.
func TestAnchors_boldWordIsSeparatelyAnchored(t *testing.T) {
	_, anchors, err := RenderWithAnchors(anchorFixture)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	byText := map[string]Anchor{}
	for _, a := range anchors {
		byText[a.Text] = a
	}
	that, ok1 := byText["That "]
	saved, ok2 := byText["saved"]
	rest, ok3 := byText[" a wretch like me"]
	if !ok1 || !ok2 || !ok3 {
		t.Fatalf("expected the text line split into 3 runs, got: %+v", anchors)
	}
	const eps = 0.0005
	if math.Abs(saved.X0-that.X1) > eps {
		t.Errorf("\"saved\" starts at X0=%.4f but \"That \" ends at X1=%.4f — bold word not placed after it", saved.X0, that.X1)
	}
	if math.Abs(rest.X0-saved.X1) > eps {
		t.Errorf("\" a wretch like me\" starts at X0=%.4f but \"saved\" ends at X1=%.4f", rest.X0, saved.X1)
	}
	if !(saved.X0 > that.X0) {
		t.Errorf("\"saved\" (X0=%.4f) should sit to the right of the line start (X0=%.4f)", saved.X0, that.X0)
	}
}

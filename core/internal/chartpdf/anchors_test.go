package chartpdf

import (
	"bytes"
	"math"
	"testing"

	"github.com/go-pdf/fpdf"
)

// The fixture the anchor tests pin. Small but exercises every run kind: title, subtitle, section,
// a chord+lyric pair, and a text line split by **bold** (the per-word case B13 needs).
const anchorFixture = "# Amazing Grace\nJohn Newton, 1779\n\n## Verse 1\n" +
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
	want := []Anchor{
		{Page: 0, Text: "Amazing Grace", X0: 0.0571, Y0: 0.0404, X1: 0.3418, Y1: 0.0796},
		{Page: 0, Text: "John Newton, 1779", X0: 0.0571, Y0: 0.0796, X1: 0.2887, Y1: 0.1041},
		{Page: 0, Text: "Verse 1", X0: 0.0571, Y0: 0.1237, X1: 0.1528, Y1: 0.1530},
		{Page: 0, Text: "G        G7      C   G", X0: 0.0571, Y0: 0.1555, X1: 0.4119, Y1: 0.1800},
		{Page: 0, Text: "A-ma-zing grace, how sweet the sound", X0: 0.0571, Y0: 0.1766, X1: 0.6377, Y1: 0.2010},
		{Page: 0, Text: "That ", X0: 0.0571, Y0: 0.2148, X1: 0.1184, Y1: 0.2441},
		{Page: 0, Text: "saved", X0: 0.1184, Y0: 0.2148, X1: 0.1946, Y1: 0.2441},
		{Page: 0, Text: " a wretch like me", X0: 0.1946, Y0: 0.2148, X1: 0.3962, Y1: 0.2441},
		{Page: 0, Text: "I once was lost, but now am found", X0: 0.0571, Y0: 0.2407, X1: 0.4635, Y1: 0.2701},
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

// The invariant that matters for B13: every box actually SPANS the text it names at the size rendered
// — not "some boxes exist". We re-measure each run's width independently (fresh fpdf, same font) and
// assert it equals the box width. Teeth: record a constant or wrong-scaled width and this fails.
func TestAnchors_boxContainsItsTextAtRenderedSize(t *testing.T) {
	_, anchors, err := RenderWithAnchors(anchorFixture)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	// The fixture auto-fits; derive the same scale the renderer used so the re-measure font matches.
	lines := chartLines(anchorFixture)
	subtitle, _, _, _, skip := parseHeader(lines)
	scale := autoFitBodyPt(lines, subtitle, skip) / defaultBodyPt

	// font+size a run is drawn in, by its text (enough to disambiguate the fixture's runs).
	fontOf := func(a Anchor) (fam, style string, ptmm float64) {
		switch {
		case a.Text == "Amazing Grace":
			return "Helvetica", "B", 16 * scale
		case a.Text == "John Newton, 1779":
			return "Helvetica", "I", 11 * scale
		case a.Text == "Verse 1":
			return "Helvetica", "B", 11 * scale
		case a.Text == "G        G7      C   G":
			return "Courier", "B", 11 * scale
		case a.Text == "A-ma-zing grace, how sweet the sound":
			return "Courier", "", 11 * scale
		case a.Text == "saved":
			return "Helvetica", "B", 11 * scale // the **bold** word
		default:
			return "Helvetica", "", 11 * scale // plain text-line segments
		}
	}
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	tr := m.UnicodeTranslatorFromDescriptor("")
	for _, a := range anchors {
		if a.X1 <= a.X0 || a.Y1 <= a.Y0 {
			t.Errorf("degenerate box for %q: %+v", a.Text, a)
			continue
		}
		if a.X0 < 0 || a.Y0 < 0 || a.X1 > 1 || a.Y1 > 1 {
			t.Errorf("off-page box for %q: %+v", a.Text, a)
		}
		fam, style, pt := fontOf(a)
		m.SetFont(fam, style, pt)
		wantW := m.GetStringWidth(tr(a.Text))
		gotW := (a.X1 - a.X0) * pageW
		if math.Abs(gotW-wantW) > 0.05 { // mm
			t.Errorf("box width for %q = %.3fmm, but the text renders %.3fmm wide", a.Text, gotW, wantW)
		}
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

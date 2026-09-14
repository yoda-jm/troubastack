package chartpdf

import (
	"math"
	"testing"

	"troubastack/core/internal/domain"
)

// runs builds a manifest of stacked lines: each 0.6 wide starting at x=0.2, height h, spaced 2h apart.
func runs(h float64, texts ...string) []Anchor {
	out := make([]Anchor, 0, len(texts))
	for i, t := range texts {
		top := 0.1 + float64(i)*2*h
		out = append(out, Anchor{Page: 0, Text: t, X0: 0.2, X1: 0.8, Y0: top, Y1: top + h, Seq: i})
	}
	return out
}

// TestUnderlineAnchorsToTheLineAbove is T172's teeth vector, named by the spec: a mark whose box sits
// ENTIRELY BELOW its run. Under the centre test it anchored to nothing — its centre is in the gap between
// lines — so a reflow left it behind while the words moved.
func TestUnderlineAnchorsToTheLineAbove(t *testing.T) {
	h := 0.04
	as := runs(h, "the first line", "the second line", "the third line")
	// an underline just under the SECOND line: y from its bottom to a quarter-height below
	second := as[1]
	sa, ok := AnchorAt(as, 0, 0.25, second.Y1+0.1*h, 0.55, second.Y1+0.35*h)
	if !ok {
		t.Fatal("an underline under a line is still un-anchorable — T172's whole point")
	}
	if sa.RunText != "the second line" {
		t.Errorf("anchored to %q, want the line it underlines", sa.RunText)
	}
	if sa.Offset == nil {
		t.Fatal("no Offset recorded: the mark would re-project ON the words it underlines")
	}
	if sa.Offset.RelY0 <= 1 {
		t.Errorf("RelY0=%v — an underline must sit BELOW the run (>1 run-height from its top)", sa.Offset.RelY0)
	}
}

// TestUnderlineStaysUnderItsLineAtANewTypeSize is the property the whole task exists for, and it is checked
// on a REAL pair of renders rather than a hand-built manifest: the same chart at 11 pt and at 13 pt, which
// is the exact change that stranded 15 of VLL's marks.
func TestUnderlineStaysUnderItsLineAtANewTypeSize(t *testing.T) {
	// The underline goes under the LAST line of the block, which is where a real one has room. Measured:
	// consecutive lyric runs OVERLAP vertically by 0.12 run-heights — there is no whitespace between the
	// lines of a verse at all — so a mark "just under" a mid-block line is inside the NEXT line's box and
	// the centre test already claims it. The case T172 adds is the mark that is below everything: under a
	// block's last line, beside a section break. My first fixture put the underline mid-block and the test
	// correctly refused to call it an underline of the line above.
	const body = "\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\nand morning finds the room\n"
	_, small, err := RenderWithAnchors("# T\nsize: 11" + body)
	if err != nil {
		t.Fatal(err)
	}
	_, large, err := RenderWithAnchors("# T\nsize: 13" + body)
	if err != nil {
		t.Fatal(err)
	}
	target := "and morning finds the room" // the block's last line
	src, ok := runAt(small, target, 1)
	if !ok {
		t.Fatalf("fixture line %q not in the 11 pt render", target)
	}
	gap := (src.Y1 - src.Y0) * 0.25 // a quarter of a line-height below it

	sa, ok := AnchorAt(small, src.Page, src.X0+0.01, src.Y1+gap, src.X1-0.01, src.Y1+gap*1.6)
	if !ok || sa.RunText != target {
		t.Fatalf("underline anchored to %q (ok=%v), want %q", sa.RunText, ok, target)
	}

	_, _, py0, _, py1, ok := Project(sa, large)
	if !ok {
		t.Fatal("the anchor no longer resolves against the larger render")
	}
	dst, _ := runAt(large, target, 1)
	if py0 < dst.Y1 {
		t.Errorf("the underline projected ON its line (top %.4f is above the line's bottom %.4f) — "+
			"re-projection put the mark over the words it was drawn under", py0, dst.Y1)
	}
	// and it stays PROPORTIONALLY where it was: a quarter-height below, at either size
	relBefore := (src.Y1 + gap - src.Y0) / (src.Y1 - src.Y0)
	relAfter := (py0 - dst.Y0) / (dst.Y1 - dst.Y0)
	if math.Abs(relBefore-relAfter) > 0.02 {
		t.Errorf("offset drifted across the size change: %.3f run-heights before, %.3f after", relBefore, relAfter)
	}
	if py1 <= py0 {
		t.Error("projected box is inverted or empty")
	}
}

// TestRefusalIsKept — R3. A mark far from any text has no referent, and inventing one is worse than
// leaving its coordinates. 3.0 run-heights is the measured bound; this checks BOTH sides of it, because a
// radius that never refuses and a radius that refuses everything both pass a one-sided test.
func TestRefusalIsKept(t *testing.T) {
	h := 0.04
	as := runs(h, "the only line")
	line := as[0]
	if _, ok := AnchorAt(as, 0, 0.3, line.Y1+2.5*h, 0.5, line.Y1+2.6*h); !ok {
		t.Error("a mark 2.5 run-heights away was refused — inside the measured radius")
	}
	if _, ok := AnchorAt(as, 0, 0.3, line.Y1+6*h, 0.5, line.Y1+6.1*h); ok {
		t.Error("a mark 6 run-heights from any text was anchored — that is the outlier R3 exists to refuse")
	}
}

// TestNoHorizontalOverlapStaysUnanchorable — Fable's deliberate refusal: a mark beside a BLOCK is not
// beside a RUN. Anchoring it to whichever line is nearest records a relationship we cannot justify.
func TestNoHorizontalOverlapStaysUnanchorable(t *testing.T) {
	h := 0.04
	as := runs(h, "a line of words")
	line := as[0]
	// a margin bracket to the LEFT of the text column, vertically level with the line
	if sa, ok := AnchorAt(as, 0, 0.05, line.Y0, 0.12, line.Y1); ok {
		t.Errorf("a margin mark anchored to %q — it overlaps no run horizontally", sa.RunText)
	}
}

// TestOnRunAnchorIsUNCHANGED: the centre test still wins when it applies, and it records NO offset. Every
// anchor written before T172 has none, so "absent" has to keep meaning exactly what it meant.
func TestOnRunAnchorIsUnchanged(t *testing.T) {
	h := 0.04
	as := runs(h, "right on the words")
	line := as[0]
	sa, ok := AnchorAt(as, 0, 0.3, line.Y0+0.1*h, 0.5, line.Y1-0.1*h)
	if !ok {
		t.Fatal("a mark on its run stopped anchoring")
	}
	if sa.Offset != nil {
		t.Errorf("an on-run mark recorded an Offset (%+v) — absent must keep meaning 'on the run'", *sa.Offset)
	}
	_, x0, y0, x1, y1, ok := Project(sa, as)
	if !ok {
		t.Fatal("projection failed")
	}
	if math.Abs(y0-line.Y0) > 1e-9 || math.Abs(y1-line.Y1) > 1e-9 {
		t.Errorf("an offset-less anchor projected to (%.4f,%.4f), want the run's own box (%.4f,%.4f)", y0, y1, line.Y0, line.Y1)
	}
	if x1 <= x0 {
		t.Error("projected box is empty")
	}
}

// TestSpanCoversBothEnds — R2. A bracket over two lines relates to runs N..M; projecting it onto the first
// alone would shrink it to one line every time the chart re-rendered.
func TestSpanCoversBothEnds(t *testing.T) {
	h := 0.04
	as := runs(h, "first line here", "second line here", "third line here")
	sa := domain.SourceAnchor{
		RunText: "first line here", Occurrence: 1, CharStart: 0, CharEnd: 15,
		Span: &domain.AnchorSpan{RunText: "third line here", Occurrence: 1},
	}
	_, _, y0, _, y1, ok := Project(sa, as)
	if !ok {
		t.Fatal("a spanning anchor did not resolve")
	}
	if math.Abs(y0-as[0].Y0) > 1e-9 {
		t.Errorf("span top %.4f, want the first run's top %.4f", y0, as[0].Y0)
	}
	if math.Abs(y1-as[2].Y1) > 1e-9 {
		t.Errorf("span bottom %.4f, want the LAST run's bottom %.4f — the span collapsed to one line", y1, as[2].Y1)
	}

	// a span whose far end was edited away falls back to the near end, never to a stretched guess
	gone := sa
	gone.Span = &domain.AnchorSpan{RunText: "a line that is not there", Occurrence: 1}
	_, _, gy0, _, gy1, ok := Project(gone, as)
	if !ok {
		t.Fatal("losing the far end lost the whole anchor")
	}
	if math.Abs(gy0-as[0].Y0) > 1e-9 || math.Abs(gy1-as[0].Y1) > 1e-9 {
		t.Errorf("fallback box (%.4f,%.4f), want the first run alone (%.4f,%.4f)", gy0, gy1, as[0].Y0, as[0].Y1)
	}
}

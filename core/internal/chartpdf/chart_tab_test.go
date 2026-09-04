package chartpdf

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// staveLine builds a stave content line of exactly n runes (n>=2): "e|" then dashes.
func staveLine(n int) string {
	if n < 2 {
		n = 2
	}
	return "e|" + strings.Repeat("-", n-2)
}

// tabAnchors returns the anchors whose text looks like a stave/tab line (drawn via drawTabLine). We
// identify them by their text being present in the source block; simpler: return all anchors and let
// callers filter by Text.
func mustAnchors(t *testing.T, src string) []Anchor {
	t.Helper()
	_, an, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatalf("RenderWithAnchors: %v", err)
	}
	return an
}

func anchorFor(an []Anchor, text string) (Anchor, bool) {
	for _, a := range an {
		if a.Text == text {
			return a, true
		}
	}
	return Anchor{}, false
}

// TestTab_Markers: the openers/closers open a block (content drawn verbatim); the near-misses are NOT
// markers and render as literal text.
func TestTab_Markers(t *testing.T) {
	for _, open := range []string{"{start_of_tab}", "{sot}", "{SOT}", "  {sot}  "} {
		src := "# T\n" + open + "\ne|--0--|\n{eot}\n"
		an := mustAnchors(t, src)
		if _, ok := anchorFor(an, "e|--0--|"); !ok {
			t.Fatalf("opener %q: stave line not drawn; anchors=%v", open, texts(an))
		}
		if _, ok := anchorFor(an, open); ok {
			t.Fatalf("opener %q was drawn as text, should be consumed", open)
		}
	}
	// Near-misses render as literal body text (an anchor with that exact text).
	for _, txt := range []string{"{tab}", "{sot} x", "{{sot}}", "sot"} {
		src := "# T\n" + txt + "\n"
		an := mustAnchors(t, src)
		if _, ok := anchorFor(an, txt); !ok {
			t.Fatalf("non-marker %q should render as text; anchors=%v", txt, texts(an))
		}
	}
}

// TestTab_VerbatimInsideBlock: inside a block, section/marker/bold syntax is literal.
func TestTab_VerbatimInsideBlock(t *testing.T) {
	src := "# T\n{sot}\n## Not A Section\n{np}\n**not bold**\n{eot}\n"
	an := mustAnchors(t, src)
	for _, want := range []string{"## Not A Section", "{np}", "**not bold**"} {
		if _, ok := anchorFor(an, want); !ok {
			t.Fatalf("inside a block %q must be verbatim; anchors=%v", want, texts(an))
		}
	}
}

// TestTab_WidthKeepShrinkRefuse: proportional at typical widths, shrink when wide, refuse past the floor.
func TestTab_WidthKeepShrinkRefuse(t *testing.T) {
	height := func(n int) float64 {
		line := staveLine(n)
		// size: 11 pins the body so this isolates the WIDTH rule from auto-fit's body sizing.
		a, ok := anchorFor(mustAnchors(t, "# T\nsize: 11\n{sot}\n"+line+"\n{eot}\n"), line)
		if !ok {
			t.Fatalf("%d-char stave not drawn", n)
		}
		// never clipped: the box right edge stays within the column.
		if a.X1 > float64(right)/float64(pageW)+1e-9 {
			t.Fatalf("%d-char stave X1=%.4f exceeds the column right edge", n, a.X1)
		}
		return a.Y1 - a.Y0
	}
	h60, h90, h111 := height(60), height(90), height(111)
	if !approxEq(h60, h90) {
		t.Fatalf("short lines should both be proportional (same tab size): h60=%.5f h90=%.5f", h60, h90)
	}
	if !(h111 < h90-1e-6) {
		t.Fatalf("a 111-char line should shrink the tab (h111=%.5f !< h90=%.5f)", h111, h90)
	}
	// Past the floor: refused.
	wide := staveLine(140)
	if _, err := Render("# T\n{sot}\n" + wide + "\n{eot}\n"); !errors.Is(err, ErrTabTooWide) {
		t.Fatalf("a 140-char stave should be refused with ErrTabTooWide, got %v", err)
	}
}

// TestTab_OneSizePerChart: two blocks with different longest lines draw at one shared size.
func TestTab_OneSizePerChart(t *testing.T) {
	short, long := staveLine(40), staveLine(110)
	src := "# T\n{sot}\n" + short + "\n{eot}\n\n## Riff\n{sot}\n" + long + "\n{eot}\n"
	an := mustAnchors(t, src)
	as, oks := anchorFor(an, short)
	al, okl := anchorFor(an, long)
	if !oks || !okl {
		t.Fatalf("both staves must draw; anchors=%v", texts(an))
	}
	if hs, hl := as.Y1-as.Y0, al.Y1-al.Y0; !approxEq(hs, hl) {
		t.Fatalf("blocks drew at two sizes (h short=%.5f, h long=%.5f); want one shared size", hs, hl)
	}
}

// TestTab_ChordRowInsideBlockAtTabSize: a chord row over the strings is drawn at the SAME tab size as
// the string lines (bold, chord colour) — its box height matches, not the normal chord lead.
func TestTab_ChordRowInsideBlockAtTabSize(t *testing.T) {
	chordRow := "  G       D"
	str := staveLine(40)
	src := "# T\n{sot}\n" + chordRow + "\n" + str + "\n{eot}\n"
	an := mustAnchors(t, src)
	ac, okc := anchorFor(an, chordRow)
	astr, oks := anchorFor(an, str)
	if !okc || !oks {
		t.Fatalf("chord row + string line must both draw; anchors=%v", texts(an))
	}
	if !approxEq(ac.Y1-ac.Y0, astr.Y1-astr.Y0) {
		t.Fatalf("chord row inside a block should be at the tab size (h=%.5f) like the strings (h=%.5f)", ac.Y1-ac.Y0, astr.Y1-astr.Y0)
	}
}

// TestTab_TransposeSkipsBlock: transposing changes body chord rows but never a line inside a block,
// and preserves the line count (the T60 anchoring invariant).
func TestTab_TransposeSkipsBlock(t *testing.T) {
	src := "# Song\n{sot}\n     G    D\ne|--0--2--|\n{eot}\n\n## Verse\nG      C\nlyrics here\n"
	from, _ := ParseKey("G")
	to, _ := ParseKey("A") // +2
	out, err := Transpose(src, from, to)
	if err != nil {
		t.Fatalf("transpose: %v", err)
	}
	if a, b := strings.Count(src, "\n"), strings.Count(out, "\n"); a != b {
		t.Fatalf("line count changed %d -> %d", a, b)
	}
	if !strings.Contains(out, "     G    D") || !strings.Contains(out, "e|--0--2--|") {
		t.Fatalf("tab block was transposed:\n%s", out)
	}
	if strings.Contains(out, "G      C") {
		t.Fatalf("body chord row was NOT transposed:\n%s", out)
	}
	if !strings.Contains(out, "A") || !strings.Contains(out, "D") {
		t.Fatalf("body chord row should show the transposed chords:\n%s", out)
	}
}

// TestTab_StaveIsNeverSplit: every line of a stave lands on one page even when filler pushes the block
// across a boundary (the stave is a layout unit, like a chord+lyric pair).
func TestTab_StaveIsNeverSplit(t *testing.T) {
	var b strings.Builder
	b.WriteString("# T\nsize: 11\n")
	for i := 0; i < 60; i++ { // filler to push the block near the page bottom
		b.WriteString("filler line of body text\n")
	}
	b.WriteString("{sot}\n")
	stave := []string{"e|--0--|", "B|--1--|", "G|--0--|", "D|--2--|", "A|--3--|", "E|-----|"}
	for _, s := range stave {
		b.WriteString(s + "\n")
	}
	b.WriteString("{eot}\n")
	an := mustAnchors(t, b.String())
	page := -1
	for _, s := range stave {
		a, ok := anchorFor(an, s)
		if !ok {
			t.Fatalf("stave line %q missing", s)
		}
		if page == -1 {
			page = a.Page
		} else if a.Page != page {
			t.Fatalf("stave split across pages: %q on page %d, expected %d", s, a.Page, page)
		}
	}
}

// TestTab_MeasureMatchesRender: the drift guard holds for a mixed chart — measure()'s paginated end-y
// equals the renderer's returned y (both walk the same layout, tab blocks included).
func TestTab_MeasureMatchesRender(t *testing.T) {
	// size: 11 pins the body so renderChart does not auto-fit while measure() stays at the parsed size
	// (same reason as TestT75_MeasureMatchesRender).
	src := "# The Open Road\nsize: 11\nOasis\n\n## Riff\n{sot}\n     G                 D\ne|-----0-----------0-----|\nB|---0-----3-----0-------|\n{eot}\n\n## Verse\nG                 C\nMorning on the highway\n"
	_, _, y, err := renderChart(src, false)
	if err != nil {
		t.Fatalf("renderChart: %v", err)
	}
	if m := measure(src); m-y > 0.01 || m-y < -0.01 {
		t.Fatalf("measure()=%.4f != renderChart y=%.4f (drift)", m, y)
	}
}

func approxEq(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }

// TestTab_OpenerOriginalGrammar: an opener may carry `original=KEY` (recognised, key extracted); a bare
// trailing token stays text (Fable's rule — attributes only in key=value form). Re-pins the boundary.
func TestTab_OpenerOriginalGrammar(t *testing.T) {
	if !isTabStart("{sot original=G}") || !isTabStart("{start_of_tab original=Bbm}") {
		t.Fatal("an opener with original=KEY must be recognised")
	}
	if got := tabOpenerOriginalKey("{sot original=G}"); got != "G" {
		t.Fatalf("original key = %q, want G", got)
	}
	for _, notMarker := range []string{"{sot} x", "{sot bad}", "{{sot}}", "{sot}x"} {
		if isTabStart(notMarker) {
			t.Fatalf("%q must NOT be an opener (attributes only in key=value form)", notMarker)
		}
	}
	// An authored original= opens a block AND draws the marker (author's claim about their document).
	an := mustAnchors(t, "# T\nsize: 11\n{sot original=G}\ne|--0--|\n{eot}\n")
	if _, ok := anchorFor(an, "e|--0--|"); !ok {
		t.Fatal("an opener with original= must still open the block")
	}
}

// TestTab_TransposeMarker: the required marker — baking a tab chart transposed marks the block in the
// PDF, and (the property that matters) the anchor manifest is geometry-identical at +0 and +N, only the
// body chord-row TEXT differing. Zero-height marker ⇒ nothing below it moves (bake does not re-anchor).
func TestTab_TransposeMarker(t *testing.T) {
	src := "# Song\nsize: 11\n\n## Riff\n{sot}\n     G     D\ne|--0--2--0--|\n{eot}\n\n## Verse\nG      C\nlyric line here\n"
	from, _ := ParseKey("G")
	to, _ := ParseKey("A") // +2
	up, err := Transpose(src, from, to)
	if err != nil {
		t.Fatalf("transpose: %v", err)
	}
	// The transpose stamped the opener; the tab body is untouched and line count preserved.
	if !strings.Contains(up, "{sot original=G}") {
		t.Fatalf("opener not stamped with original key:\n%s", up)
	}
	if strings.Count(src, "\n") != strings.Count(up, "\n") {
		t.Fatal("transpose changed the line count")
	}

	_, a0, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	_, aN, err := RenderWithAnchors(up)
	if err != nil {
		t.Fatal(err)
	}
	if len(a0) != len(aN) {
		t.Fatalf("anchor count changed %d -> %d (the marker must not add an anchor)", len(a0), len(aN))
	}
	textDiffs := 0
	for i := range a0 {
		g, n := a0[i], aN[i]
		if g.Page != n.Page || !approxEq(g.X0, n.X0) || !approxEq(g.Y0, n.Y0) || !approxEq(g.X1, n.X1) || !approxEq(g.Y1, n.Y1) {
			t.Fatalf("anchor %d geometry moved under transposition: %+v vs %+v", i, g, n)
		}
		if g.Text != n.Text {
			textDiffs++
		}
	}
	if textDiffs == 0 {
		t.Fatal("expected the body chord row's text to change under +2")
	}

	// Marker present at +2, absent at +0 (acceptance as written).
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not installed; geometry invariant already checked")
	}
	base, _ := Render(src)
	trans, _ := Render(up)
	if strings.Contains(pdftotext(t, base), "tab in original key") {
		t.Fatal("marker must be ABSENT at +0")
	}
	if !strings.Contains(pdftotext(t, trans), "tab in original key (G)") {
		t.Fatal("marker must be PRESENT and name the original key at +2")
	}
}

// TestTab_ExistingChartsByteIdentical: a chart with no block is untouched; adding a block moves the sha
// (teeth-check that the block path actually changes output).
func TestTab_ExistingChartsByteIdentical(t *testing.T) {
	plain := "# T\n\n## Verse\nG C\nla la la\n"
	withTab := plain + "{sot}\ne|--0--|\n{eot}\n"
	a, err := Render(plain)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(withTab)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) == string(b) {
		t.Fatal("adding a tab block did not change the render (the block path is inert)")
	}
}

func texts(an []Anchor) []string {
	out := make([]string, 0, len(an))
	for _, a := range an {
		out = append(out, a.Text)
	}
	return out
}

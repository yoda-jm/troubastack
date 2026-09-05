package chartpdf

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const sample = `# The Open Road

## Verse 1
G            D
Pack a little light for the road ahead,
Em           C
leave the rest of yesterday unsaid.

## Chorus
C           G
So drive, drive into the wide unknown —
the map is just a rumour.

A plain line with **bold** words.`

// pdftotext extracts text from a PDF, skipping the test if poppler is absent.
func pdftotext(t *testing.T, pdf []byte) string {
	t.Helper()
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext (poppler) not installed")
	}
	cmd := exec.Command("pdftotext", "-layout", "-", "-")
	cmd.Stdin = bytes.NewReader(pdf)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		t.Fatalf("pdftotext: %v", err)
	}
	return out.String()
}

func TestRender_ExtractsContent(t *testing.T) {
	pdf, err := Render(sample)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatal("output is not a PDF")
	}
	text := pdftotext(t, pdf)
	for _, want := range []string{
		"The Open Road", "Verse 1", "Chorus",
		"Pack a little light for the road ahead,",
		"So drive, drive into the wide unknown", // em-dash line
		"the map is just a rumour.",
		"G", "D", "Em", "C", // chords
		"bold", // the bold word survives (markers stripped)
	} {
		if !strings.Contains(text, want) {
			t.Errorf("extracted text missing %q\n--- got ---\n%s", want, text)
		}
	}
	// The em-dash must survive as an em-dash (T16 cp1252 mapping), not mojibake.
	if !strings.Contains(text, "—") {
		t.Errorf("em-dash lost / mojibake'd\n--- got ---\n%s", text)
	}
	// The bold markers themselves must NOT appear as literal text.
	if strings.Contains(text, "**") {
		t.Errorf("literal ** markers leaked into output\n--- got ---\n%s", text)
	}
}

func TestRender_Deterministic(t *testing.T) {
	a, err := Render(sample)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Render(sample)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) {
		t.Fatal("render is not byte-deterministic")
	}
}

func TestRender_RejectsNonLatin1(t *testing.T) {
	_, err := Render("# Title\n\nkanji: 漢字")
	if !errors.Is(err, ErrUnsupportedChar) {
		t.Fatalf("err = %v, want ErrUnsupportedChar", err)
	}
}

func TestIsChordRow(t *testing.T) {
	for _, c := range []string{"G", "Em C G", "C#m7 Dsus4", "A7sus4 F/G", "N.C."} {
		if !isChordRow(c) {
			t.Errorf("isChordRow(%q) = false, want true", c)
		}
	}
	for _, l := range []string{"Pack a little light", "the open road", "So drive"} {
		if isChordRow(l) {
			t.Errorf("isChordRow(%q) = true, want false (it's lyrics)", l)
		}
	}
}

func TestSubtitleOf(t *testing.T) {
	cases := []struct {
		name    string
		src     string
		want    string
		wantIdx int
	}{
		{"adjacent artist", "# My Song\nThe Artist\n\n## Verse 1", "The Artist", 1},
		{"blank then lyric — NO subtitle (regression)", "# My Song\n\nPack a little light for the road\n\n## Verse 1", "", -1},
		{"adjacent chord row — none", "# My Song\nAm C G\n\n## Verse 1", "", -1},
		{"adjacent section — none", "# My Song\n## Verse 1\nlyric", "", -1},
		{"title at EOF", "# My Song", "", -1},
		{"no title at all", "just some text\nmore text", "", -1},
		{"subtitle then chord row — none", "# My Song\nThe Artist\nAm C\nlyric", "", -1},
		{"subtitle then EOF", "# My Song\nThe Artist", "The Artist", 1},
		{"subtitle then section", "# My Song\nThe Artist\n## Verse 1", "The Artist", 1},
	}
	for _, c := range cases {
		got, idx := subtitleOf(strings.Split(c.src, "\n"))
		if got != c.want || idx != c.wantIdx {
			t.Errorf("%s: subtitleOf = (%q, %d), want (%q, %d)", c.name, got, idx, c.want, c.wantIdx)
		}
	}
}

// normChartLine reduces a source line to its rendered text: strip the `## ` section marker and
// `**bold**` markers, collapse whitespace (chords are laid out with wide gaps).
func normChartLine(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "## ")
	s = strings.ReplaceAll(s, "**", "")
	return strings.Join(strings.Fields(s), " ")
}

// TestSubtitleHeader_BodyPreservation is the guard that matters (T70): over every committed
// .chart fixture, (a) none gains a subtitle — demo charts have a blank after the title, so the
// adjacency rule lifts nothing out of the body; and (b) every non-blank, non-title source line
// still appears in the rendered PDF text. A property, not a golden, so it holds as fixtures change.
func TestSubtitleHeader_BodyPreservation(t *testing.T) {
	fixtures, _ := filepath.Glob("../../../docs/demo-charts/*.chart")
	if len(fixtures) == 0 {
		t.Skip("no .chart fixtures found")
	}
	for _, fx := range fixtures {
		src, err := os.ReadFile(fx)
		if err != nil {
			t.Fatal(err)
		}
		lines := strings.Split(strings.ReplaceAll(string(src), "\r\n", "\n"), "\n")
		if sub, idx := subtitleOf(lines); sub != "" || idx != -1 {
			t.Errorf("%s: unexpectedly gained subtitle %q (idx %d) — demo charts must be unaffected", fx, sub, idx)
		}
		pdf, err := Render(string(src))
		if err != nil {
			t.Fatalf("%s: render: %v", fx, err)
		}
		got := strings.Join(strings.Fields(pdftotext(t, pdf)), " ")
		for i, ln := range lines {
			s := strings.TrimSpace(ln)
			if s == "" || strings.HasPrefix(s, "# ") || isNewPageMarker(s) || isFootnoteMarker(s) {
				continue // blank, the title line, or a consumed flow marker ({new_page}/{footnote})
			}
			if want := normChartLine(ln); want != "" && !strings.Contains(got, want) {
				t.Errorf("%s: line %d %q lost from rendered output", fx, i, s)
			}
		}
	}
}

func TestChordRowParts(t *testing.T) {
	chordRows := []string{
		"Am E7",
		"Am E7 (x2)",
		"Am E7 G D F C Dm E7 (2x, 1x Arpèges, 1x normal)",
		"G            D", // spacing preserved
		"N.C.",
	}
	notChordRows := []string{
		"A (very) long day",         // has "(" but doesn't end in ")"
		"(x2)",                      // no chords precede the "("
		"Am E7 (2x",                 // unbalanced
		"On a dark desert highway,", // lyric
		"",                          // blank
	}
	for _, s := range chordRows {
		if !isChordRow(s) {
			t.Errorf("isChordRow(%q) = false, want true", s)
		}
	}
	for _, s := range notChordRows {
		if isChordRow(s) {
			t.Errorf("isChordRow(%q) = true, want false", s)
		}
	}
	if ch, an, ok := chordRowParts("Am E7 (x2)"); !ok || ch != "Am E7" || an != "(x2)" {
		t.Errorf("chordRowParts(\"Am E7 (x2)\") = (%q, %q, %v), want (\"Am E7\", \"(x2)\", true)", ch, an, ok)
	}
	if ch, _, _ := chordRowParts("G            D"); ch != "G            D" {
		t.Errorf("chord spacing not preserved: %q", ch)
	}
}

// accentChart exercises accents + an em-dash across the title, subtitle, a section name, a
// chord-row annotation and a lyric — for the durable no-mojibake assertion.
const accentChart = `# Café del Mar — Live
Björk

## Intro
Am E7 G (2x, 1x Arpèges, 1x normal)

## Verse 7 (Arpèges)
Am
Voilà, l'été déjà
`

func TestRender_AnnotationsAndAccents(t *testing.T) {
	pdf, err := Render(accentChart)
	if err != nil {
		t.Fatal(err)
	}
	text := pdftotext(t, pdf)

	// The accented SECTION header specifically must be intact — the full string only comes from
	// the header, so this is section-targeted (not satisfied by "Arpèges" appearing in a body line).
	if !strings.Contains(text, "Verse 7 (Arpèges)") {
		t.Errorf("accented section header missing/mojibake'd\n--- got ---\n%s", text)
	}
	// The chord-row annotation renders (chords + the note on one line).
	for _, want := range []string{"Am E7 G", "2x, 1x Arpèges, 1x normal"} {
		if !strings.Contains(text, want) {
			t.Errorf("chord-row annotation missing %q\n--- got ---\n%s", want, text)
		}
	}
	// THE DURABLE GUARD: no cp1252→UTF-8 mojibake sequence may appear anywhere in the output.
	// This would have caught all three sectionLabel/subtitle instances.
	for _, bad := range []string{"Ã", "â€", "Â"} {
		if strings.Contains(text, bad) {
			t.Errorf("mojibake sequence %q in rendered output\n--- got ---\n%s", bad, text)
		}
	}
}

func TestParseHeaderDirectives(t *testing.T) {
	cases := []struct {
		name        string
		src         string
		wantSub     string
		wantPt      float64
		wantSizeSet bool // T76: a `size:` directive (in range or not) disables auto-fit
		wantSkip    []int
	}{
		{"size then artist", "# S\nsize: 13\nThe Artist\n\n## V\nx", "The Artist", 13, true, []int{1, 2}},
		{"artist then size", "# S\nThe Artist\nsize: 13\n\n## V\nx", "The Artist", 13, true, []int{1, 2}},
		{"case-insensitive, no space", "# S\nSize:14\n\n## V\nx", "", 14, true, []int{1}},
		{"out of range: ignored but consumed, still disables auto-fit", "# S\nsize: 99\nThe Artist\n\n## V", "The Artist", 11, true, []int{1, 2}},
		{"malformed: not a directive, stays subtitle, auto-fit stays on", "# S\nsize: abc\n\n## V", "size: abc", 11, false, []int{1}},
		{"plain artist, no directive", "# S\nThe Artist\n\n## V", "The Artist", 11, false, []int{1}},
		{"blank after title: no header", "# S\n\n## V\nx", "", 11, false, nil},
	}
	for _, c := range cases {
		sub, _, pt, sizeSet, _, skip := parseHeader(strings.Split(c.src, "\n"))
		if sub != c.wantSub || pt != c.wantPt {
			t.Errorf("%s: (sub=%q, pt=%v), want (%q, %v)", c.name, sub, pt, c.wantSub, c.wantPt)
		}
		if sizeSet != c.wantSizeSet {
			t.Errorf("%s: sizeSet=%v, want %v", c.name, sizeSet, c.wantSizeSet)
		}
		if len(skip) != len(c.wantSkip) {
			t.Errorf("%s: skip=%v, want indices %v", c.name, skip, c.wantSkip)
		}
		for _, i := range c.wantSkip {
			if !skip[i] {
				t.Errorf("%s: line %d should be skipped from the body", c.name, i)
			}
		}
	}
}

// An explicit `size: 11` must render byte-identically to the pre-T74 scale-1 path (the golden sha is
// the T73 render of this source). T76 note: this used to be the NO-directive chart, but auto-fit now
// re-sizes a directive-less chart, so the byte-stability guarantee moves to the explicit-size path —
// `size: 11` disables auto-fit and reproduces exactly the old default render (the directive line is
// header-scoped and never drawn, so the bytes are unchanged).
func TestRender_ExplicitSizeByteStable(t *testing.T) {
	src := "# Café — Live\nsize: 11\nBjörk\n\n## Verse 1\nAm E7 (x2)\nVoilà l'été\n\n## Chorus\nSing it\n"
	b, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	const want = "5ab509133439819c696da38985f811af8302e689f13b806b28dfea1f6e8dcb4c"
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
		t.Errorf("size:11 render sha = %s, want %s (scale-1 output changed vs pre-T74)", got, want)
	}
}

func TestSizeDirective_SmallerFitsMore(t *testing.T) {
	if _, err := exec.LookPath("pdftotext"); err != nil {
		t.Skip("pdftotext not available")
	}
	body := "\n\n" + strings.Repeat("## Verse\nAm\na line of lyric here\n\n", 70)
	pages := func(src string) int {
		pdf, err := Render(src)
		if err != nil {
			t.Fatal(err)
		}
		return strings.Count(pdftotext(t, pdf), "\f") + 1
	}
	small, large := pages("# T\nsize: 8"+body), pages("# T\nsize: 16"+body)
	if small >= large {
		t.Errorf("size 8 used %d pages, size 16 used %d — smaller must fit strictly more per page", small, large)
	}
	s8, _ := Render("# T\nsize: 8" + body)
	s16, _ := Render("# T\nsize: 16" + body)
	if bytes.Equal(s8, s16) {
		t.Error("size 8 and size 16 produced identical bytes — the directive had no effect")
	}
}

// T75 — a chord/section row's advance must clear the previous line's type so rows never overlap;
// scale-invariant (all advances scale together, so this holds at 8/11/16 pt alike).
func TestT75_NoRowOverlap(t *testing.T) {
	const typeMM = defaultBodyPt * 25.4 / 72 // ~3.88 mm of type at 11 pt
	checks := []struct {
		name       string
		gap, floor float64
	}{
		{"lyric line vs type", leadLyric, typeMM},
		{"chord-only line vs type", leadChord, typeMM},
		{"section label vs type", leadSection, typeMM},
		{"pair: chord clears the lyric below it", pairLyricDy, typeMM},
		{"pair height clears the lyric", leadPair, pairLyricDy + typeMM},
	}
	for _, c := range checks {
		if c.gap < c.floor {
			t.Errorf("%s: advance %.2f mm < %.2f mm — rows would overlap", c.name, c.gap, c.floor)
		}
	}
}

// T75 — compaction reclaims ≥15% of body height at the SAME font size, versus origin/main.
func TestT75_CompactionReduction(t *testing.T) {
	// origin/main heights (11 pt), measured with the pre-T75 advances (see the handoff).
	baseline := map[string]float64{
		"amazing-grace":           219.0,
		"house-of-the-rising-sun": 219.0,
		"open-road-lyrics":        393.0,
	}
	for name, old := range baseline {
		b, err := os.ReadFile(filepath.Join("../../../docs/demo-charts", name+".chart"))
		if err != nil {
			t.Fatal(err)
		}
		got := contentHeight(string(b)) // continuous height — measure() now paginates (T77)
		if got > old*0.85 {
			t.Errorf("%s: %.1f mm, want ≤ %.1f mm (≥15%% shorter than %.1f)", name, got, old*0.85, old)
		}
	}
}

// T75 — measure() is the single source of the per-row advances: for a one-page chart the pure
// measure equals the renderer's final y (drift guard).
func TestT75_MeasureMatchesRender(t *testing.T) {
	b, err := os.ReadFile("../../../docs/demo-charts/amazing-grace.chart")
	if err != nil {
		t.Fatal(err)
	}
	// Pin size: 11 so renderChart does not auto-fit (T76) — the drift guard compares the renderer and
	// the pure layout walk at the SAME size; auto-fit would render at its chosen size while measure()
	// stays at the parsed size, and they would differ for a reason that is not drift.
	src := strings.Replace(string(b), "\n", "\nsize: 11\n", 1)
	if contentHeight(src) > pageBottom {
		t.Skip("fixture no longer single-page")
	}
	_, _, finalY, err := renderChart(src, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := finalY - measure(src); diff > 0.01 || diff < -0.01 {
		t.Errorf("renderer final y %.3f != measure %.3f (drift)", finalY, measure(src))
	}
}

// --- T77: {new_page} marker + section-orphan control ---------------------

// traceOf runs the shared layout in paginated trace mode and returns each drawn element's page+y+kind.
func traceOf(src string) []placed {
	lines := chartLines(src)
	subtitle, _, bodyPt, _, _, skip := parseHeader(lines)
	scale := bodyPt / defaultBodyPt
	var tr []placed
	layout(lines, scale, skip, headerBodyStart(subtitle, scale), layoutOpts{paginate: true, trace: &tr})
	return tr
}

// pageCount renders src and returns the PDF's page count.
func pageCount(t *testing.T, src string) int {
	t.Helper()
	pdf, _, _, err := renderChart(src, false)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	return pdf.PageCount()
}

func TestT77_MarkerParsing(t *testing.T) {
	yes := []string{"{new_page}", "{np}", "{NEW_PAGE}", "{NP}", "  {np}  ", "\t{new_page}\t", "{New_Page}"}
	no := []string{"{newpage}", "{new page}", "{np} x", "new_page", "{{np}}", "np", "{ np }", "{np", "np}", "a {np}"}
	for _, s := range yes {
		if !isNewPageMarker(strings.TrimSpace(s)) {
			t.Errorf("%q should be a page-break marker", s)
		}
	}
	for _, s := range no {
		if isNewPageMarker(strings.TrimSpace(s)) {
			t.Errorf("%q should NOT be a page-break marker (content, not a marker)", s)
		}
	}
}

// A {np} directly under the title must not be lifted out as the subtitle (T70), and must not render.
func TestT77_MarkerNotSubtitle(t *testing.T) {
	src := "# Title\n{np}\n\n## Verse\nla la la\n"
	if sub, _ := subtitleOf(chartLines(src)); sub != "" {
		t.Errorf("subtitle = %q, want empty (a {np} is not a subtitle)", sub)
	}
	// leading marker before any content is a no-op → single page, no blank page.
	if n := pageCount(t, src); n != 1 {
		t.Errorf("leading marker produced %d pages, want 1", n)
	}
}

func TestT77_MarkerRendersPages(t *testing.T) {
	one := "# T\n\n## A\nla la la\n"
	two := "# T\n\n## A\nla la la\n{np}\n## B\nda da da\n"
	three := "# T\n\n## A\nx\n{np}\n## B\ny\n{new_page}\n## C\nz\n"
	if n := pageCount(t, one); n != 1 {
		t.Errorf("no marker: %d pages, want 1", n)
	}
	if n := pageCount(t, two); n != 2 {
		t.Errorf("one marker: %d pages, want 2", n)
	}
	if n := pageCount(t, three); n != 3 {
		t.Errorf("two markers: %d pages, want 3", n)
	}
}

// No blank pages: a leading marker, a trailing marker, and consecutive markers each render the same
// page count as the marker-free chart.
func TestT77_NoBlankPage(t *testing.T) {
	base := "# T\n\n## A\nla la la\nmore\n"
	want := pageCount(t, base)
	cases := map[string]string{
		"leading":     "{np}\n" + base,
		"trailing":    base + "{np}\n",
		"consecutive": "# T\n\n## A\nla la la\n{np}\n{np}\n{np}\n## B\nmore\n", // one break, not three blank pages
	}
	// the consecutive case legitimately spans 2 pages (one real break); assert it is exactly 2.
	if n := pageCount(t, cases["consecutive"]); n != 2 {
		t.Errorf("consecutive markers: %d pages, want 2 (collapse to one break)", n)
	}
	for _, name := range []string{"leading", "trailing"} {
		if n := pageCount(t, cases[name]); n != want {
			t.Errorf("%s marker: %d pages, want %d (no blank page)", name, n, want)
		}
	}
}

// Orphan control: across a scan of lengths that put a section header near the page boundary, a
// section header and its first content line always land on the SAME page (the header is never
// stranded at the foot of a page). This test fails on pre-T77 code (which reserved only the header).
func TestT77_NoOrphanHeader(t *testing.T) {
	straddled := false
	for n := 30; n <= 60; n++ {
		var b strings.Builder
		b.WriteString("# Orphan\nArtist\n\n")
		for i := 0; i < n; i++ {
			b.WriteString("filler line of ordinary length here\n")
		}
		b.WriteString("\n## Chorus\nAm       E7\nunder the chords\n")
		tr := traceOf(b.String())
		// find the Chorus section and the element immediately after it (its first content).
		for i, p := range tr {
			if p.kind == "section" && i+1 < len(tr) {
				next := tr[i+1]
				if next.page != p.page {
					t.Errorf("n=%d: section header on page %d but its first line on page %d (orphaned)", n, p.page, next.page)
				}
				if p.page > 1 {
					straddled = true // we exercised the boundary at least once
				}
			}
		}
	}
	if !straddled {
		t.Errorf("scan never pushed the header past page 1 — test isn't exercising the boundary")
	}
}

// The pair rule: over a scan of lengths straddling the boundary, no chord+lyric pair overflows its
// page (its lyric, at y+pairLyricDy, plus height, always fits) — i.e. a chord is never split from
// its lyric across a page.
func TestT77_PairNeverSplit(t *testing.T) {
	for n := 20; n <= 55; n++ {
		var b strings.Builder
		b.WriteString("# Pairs\n\n## Verse\n")
		for i := 0; i < n; i++ {
			b.WriteString("Am       E7\nline of lyric beneath the chords\n")
		}
		for _, p := range traceOf(b.String()) {
			if p.kind == "pair" && p.y+leadPair > pageBottom+0.01 {
				t.Errorf("n=%d: pair at y=%.1f overflows page (y+leadPair=%.1f > %.1f)", n, p.y, p.y+leadPair, pageBottom)
			}
		}
	}
}

// The drift guard, extended to multi-page: measure() (paginated) equals the renderer's final y even
// across explicit and automatic breaks.
func TestT77_MeasureMatchesRender_MultiPage(t *testing.T) {
	var b strings.Builder
	// size: 16 pins the size (no T76 auto-fit — the drift guard compares renderer vs layout at ONE
	// size) and, at 16 pt, the 40 filler lines overflow one page, so this still exercises an automatic
	// break inside segment A as well as the explicit {np} break into segment B.
	b.WriteString("# Multi\nsize: 16\n\n## A\n")
	for i := 0; i < 40; i++ {
		b.WriteString("a filler lyric line to force an automatic break\n")
	}
	b.WriteString("{np}\n## B\nafter an explicit break\nand one more line\n")
	src := b.String()
	if pageCount(t, src) < 2 {
		t.Fatal("fixture is not multi-page")
	}
	_, _, finalY, err := renderChart(src, false)
	if err != nil {
		t.Fatal(err)
	}
	if diff := finalY - measure(src); diff > 0.01 || diff < -0.01 {
		t.Errorf("multi-page: renderer final y %.3f != measure %.3f (drift)", finalY, measure(src))
	}
}

// --- T76: auto-fit — pick the largest size that keeps a chart on one page -----------------

// chosenSize is the size auto-fit would pick for a directive-less chart (test hook).
func chosenSize(src string) float64 {
	lines := chartLines(src)
	sub, _, _, _, _, skip := parseHeader(lines)
	return autoFitBodyPt(lines, sub, skip)
}

// A normal-length chart with no directive fits on exactly one page, and the size auto-fit picks is
// MAXIMAL — one point larger overflows. "It fits" alone would pass at 8 pt for everything, so the
// maximality half is the real assertion.
func TestT76_AutoFit_OnePageAndMaximal(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Normal Song\nArtist Name\n\n")
	for _, sec := range []string{"Verse 1", "Chorus", "Verse 2", "Bridge"} {
		b.WriteString("## " + sec + "\n")
		for i := 0; i < 4; i++ {
			b.WriteString("Am        C        G\na line of the song goes here\n")
		}
		b.WriteString("\n")
	}
	src := b.String()

	if n := pageCount(t, src); n != 1 {
		t.Fatalf("auto-fit chart used %d pages, want exactly 1", n)
	}
	chosen := chosenSize(src)
	if chosen <= minBodyPt {
		t.Errorf("normal chart auto-fit to the floor (%.0f pt) — expected a larger size", chosen)
	}
	// Maximal: one point larger must overflow to a second page (unless we are already at the ceiling).
	if int(chosen) < maxBodyPt {
		bigger := strings.Replace(src, "\n", fmt.Sprintf("\nsize: %d\n", int(chosen)+1), 1)
		if n := pageCount(t, bigger); n == 1 {
			t.Errorf("chosen size %.0f pt is not maximal — %d pt also fits one page", chosen, int(chosen)+1)
		}
	}
}

// A chart too long to fit even at the floor falls back to minBodyPt, is allowed to be multi-page, and
// loses no content (T70 body-preservation must hold at the floor too).
func TestT76_AutoFit_OverlongFallsToFloor(t *testing.T) {
	const lyric = "a line of lyric that is reasonably long here"
	var b strings.Builder
	b.WriteString("# Very Long\n\n")
	for i := 0; i < 120; i++ {
		b.WriteString("## Section\nAm\n" + lyric + "\n\n")
	}
	src := b.String()

	if got := chosenSize(src); int(got) != minBodyPt {
		t.Errorf("over-long chart chose %.0f pt, want the floor %d", got, minBodyPt)
	}
	pdf, _, _, err := renderChart(src, false)
	if err != nil {
		t.Fatalf("render over-long: %v", err)
	}
	if pdf.PageCount() < 2 {
		t.Errorf("over-long chart rendered %d page(s), want multi-page at the floor", pdf.PageCount())
	}
	if _, err := exec.LookPath("pdftotext"); err == nil {
		out, err := Render(src)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(pdftotext(t, out), lyric) {
			t.Error("over-long chart lost body content at the floor (body-preservation)")
		}
	}
}

// An explicit size: disables auto-fit — it renders at exactly that size regardless of fit, even when
// auto-fit would have chosen the ceiling.
func TestT76_ExplicitSizeDisablesAutoFit(t *testing.T) {
	short := "# Short\n\n## V\nla la la\n"
	if got := chosenSize(short); int(got) != maxBodyPt {
		t.Fatalf("a tiny chart should auto-fit to the ceiling %d, got %.0f", maxBodyPt, got)
	}
	auto, err := Render(short)
	if err != nil {
		t.Fatal(err)
	}
	forced, err := Render("# Short\nsize: 8\n\n## V\nla la la\n")
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(auto, forced) {
		t.Error("size: 8 rendered identically to auto-fit — an explicit size did not disable auto-fit")
	}
}

// Auto-fit is deterministic: the same source renders to the same bytes across runs (same input →
// same chosen size → same bytes), so charts stay byte-stable in CI and the demo bundle.
func TestT76_AutoFit_Deterministic(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Determinism\nArtist\n\n")
	for i := 0; i < 30; i++ {
		b.WriteString("C        G\nsome lyric content on this line\n")
	}
	src := b.String()
	a, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	c, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, c) {
		t.Error("auto-fit render is not deterministic across runs")
	}
}

// T76 — auto-fit must count only AUTOMATIC breaks, never explicit {new_page} markers. A chart split
// into small segments by markers must auto-fit to a LARGE size (each segment fits with room to spare)
// and still render one page per segment. If explicit breaks were miscounted as automatic, the
// fit-test would see a break at every size and slam every hand-paginated chart to the 8 pt floor.
// Teeth-check: move the `*o.autoBreaks++` from page() into newPage() (so applyBreak counts too) and
// this test reddens (chosen drops to the floor) while the T77 page-count tests stay green.
func TestT76_AutoFit_ExplicitBreaksNotCounted(t *testing.T) {
	src := "# Segmented\n\n## A\nla la la\n{np}\n## B\nda da da\n{np}\n## C\nna na na\n"
	if chosen := chosenSize(src); int(chosen) != maxBodyPt {
		t.Errorf("small marked segments should auto-fit to the ceiling %d pt, got %.0f — explicit {new_page} breaks miscounted as automatic?", maxBodyPt, chosen)
	}
	if n := pageCount(t, src); n != 3 {
		t.Errorf("two {new_page} markers rendered %d pages, want 3 (marker page count must survive auto-fit)", n)
	}
}

// T76 — pin a sha on the AUTO-FIT path, the path essentially every real (directive-less) chart takes.
// TestT76_AutoFit_Deterministic only catches in-process nondeterminism; this catches drift across
// versions. It matters because bakes are content-addressed: a silent byte change re-renders and
// re-revs every concert containing a text chart. This breaks intentionally when the layout is
// legitimately tuned — the same cost we accept on the explicit-size path (ExplicitSizeByteStable).
func TestT76_AutoFitByteStable(t *testing.T) {
	src := "# Anchor Song\nfit: page\nArtist\n\n## Verse\nAm        C\na short line of lyric here\n\n## Chorus\nF         G\nanother line to sing along\n"
	b, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	const want = "8da14f04976177ab3fe348e510f9fd459c75b5eafe0efd12fcd364d81bebbf0c"
	if got := fmt.Sprintf("%x", sha256.Sum256(b)); got != want {
		t.Errorf("auto-fit render sha = %s, want %s (auto-fit output drifted)", got, want)
	}
}

// TestRender_CP1252SupplementLetters: œ/Œ (and Š/š/Ž/ž/Ÿ) are NOT Latin-1 but ARE in cp1252, which the
// core-font translator maps — so a French chart with "cœur" renders instead of being refused (T134: a real
// band chart hit this). A genuinely-unrepresentable rune is still refused.
func TestRender_CP1252SupplementLetters(t *testing.T) {
	if _, err := Render("# Cœur\n## Refrain\nœuvre Œ Š š Ž ž Ÿ\nla la\n"); err != nil {
		t.Fatalf("cp1252-supplement letters should render: %v", err)
	}
	if _, err := Render("# T\n中文\n"); err == nil { // CJK — outside cp1252, still refused
		t.Fatal("a non-cp1252 rune must still be refused with ErrUnsupportedChar")
	}
}

package chartpdf

import (
	"bytes"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// T194 — every run in the anchor manifest maps back to its source line and the rune offset of its first
// character (Anchor.SrcLine / SrcSeg / SrcChar / SrcWrap), for every kind of run the layout draws. Synthetic
// text only.

var reNotDrawn = regexp.MustCompile(`(?i)^(size\s*:\s*\d+|fit\s*:\s*(page|auto)|columns\s*:\s*\d+|\{(new_page|np|footnote|fn|end_of_tab|eot)\}|\{(start_of_tab|sot)( [^}]*)?\})$`)

var reWS = regexp.MustCompile(`\s+`)

// norm collapses blanks and drops `**`, so a wrapped run (re-joined with single spaces, bold re-opened per
// line) compares with its source text.
func norm(s string) string {
	return strings.TrimSpace(reWS.ReplaceAllString(strings.ReplaceAll(s, "**", ""), " "))
}

// srcLocErrors checks a manifest against its source and returns every violation:
//   - SrcChar points at the run's FIRST character in its source line (exactly, when that character is not
//     a blank), and the run's text continues from there (blank-insensitively, across a footnote's lines);
//   - per source line, SrcSeg counts 0, 1, 2, … in draw order, and SrcWrap never decreases;
//   - every drawable source line has at least one run.
func srcLocErrors(src string, anchors []Anchor) []string {
	lines := chartLines(src)
	var errs []string
	byLine := map[int][]Anchor{}
	order := sourceOrder(anchors)
	for _, k := range order {
		a := anchors[k]
		if a.SrcLine == -1 {
			if a.Text != "Chart" {
				errs = append(errs, fmt.Sprintf("run %q has no source line", a.Text))
			}
			continue
		}
		if a.SrcLine < 0 || a.SrcLine >= len(lines) {
			errs = append(errs, fmt.Sprintf("run %q: SrcLine %d out of range", a.Text, a.SrcLine))
			continue
		}
		byLine[a.SrcLine] = append(byLine[a.SrcLine], a)
		line := []rune(lines[a.SrcLine])
		if a.SrcChar < 0 || a.SrcChar > len(line) {
			errs = append(errs, fmt.Sprintf("run %q: SrcChar %d out of line %q", a.Text, a.SrcChar, lines[a.SrcLine]))
			continue
		}
		if a.Text == "" {
			continue
		}
		if first := []rune(a.Text)[0]; first != ' ' && (a.SrcChar >= len(line) || line[a.SrcChar] != first) {
			errs = append(errs, fmt.Sprintf("run %q: line %d char %d is %q, not its first character", a.Text, a.SrcLine, a.SrcChar, string(line[a.SrcChar:])))
			continue
		}
		rest := string(line[a.SrcChar:]) + " " + strings.Join(lines[a.SrcLine+1:], " ")
		if !strings.HasPrefix(norm(rest), norm(a.Text)) {
			errs = append(errs, fmt.Sprintf("run %q does not continue its source at line %d char %d (%q)", a.Text, a.SrcLine, a.SrcChar, string(line[a.SrcChar:])))
		}
	}
	for ln, runs := range byLine {
		for i, a := range runs {
			if a.SrcSeg != i {
				errs = append(errs, fmt.Sprintf("line %d run %d (%q) has SrcSeg %d", ln, i, a.Text, a.SrcSeg))
			}
			if i > 0 && a.SrcWrap < runs[i-1].SrcWrap {
				errs = append(errs, fmt.Sprintf("line %d: SrcWrap goes back at %q", ln, a.Text))
			}
		}
	}
	// Coverage: every drawable line has a run. Footnote paragraphs after their first line are drawn as
	// part of the first line's runs only when they do not start a wrapped line, so they are exempt.
	inFn := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if t == "" {
			inFn = false
			continue
		}
		if isFootnoteMarker(t) {
			inFn = true
			continue
		}
		if inFn || reNotDrawn.MatchString(t) {
			continue
		}
		if len(byLine[i]) == 0 {
			errs = append(errs, fmt.Sprintf("line %d %q draws no run", i, l))
		}
	}
	return errs
}

func renderLoc(t *testing.T, src string) []Anchor {
	t.Helper()
	pdf, anchors, err := RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	// The location data never changes the PDF.
	plain, err := Render(src)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pdf, plain) {
		t.Fatal("RenderWithAnchors and Render bytes differ")
	}
	for _, e := range srcLocErrors(src, anchors) {
		t.Error(e)
	}
	return anchors
}

func runsOf(anchors []Anchor, line int) []Anchor {
	var out []Anchor
	for _, k := range sourceOrder(anchors) {
		if anchors[k].SrcLine == line {
			out = append(out, anchors[k])
		}
	}
	return out
}

func runeIdx(s, sub string) int {
	b := strings.Index(s, sub)
	if b < 0 {
		return -1
	}
	return len([]rune(s[:b]))
}

const locFixture = `# The Open Road
The Wayfarers

## Verse 1
G       C        D   (x2)
We drive the road until the evening
Some **bold** words and **more bold** here
Em      C
A short lyric line
Words with   double  spaces
Café **crème** brûlée

{sot}
e|---0---3---|
B|---1---1---|
{eot}

{fn}
Words and music by nobody in particular,
arranged for the open road.
`

func TestManifestSrcLoc_EveryRunKind(t *testing.T) {
	src := locFixture
	lines := chartLines(src)
	anchors := renderLoc(t, src)
	at := func(text string) int { return lineIndex(t, src, text) }

	title := runsOf(anchors, 0)
	if len(title) != 1 || title[0].Text != "The Open Road" || title[0].SrcChar != 2 {
		t.Errorf("title runs %+v", title)
	}
	if sub := runsOf(anchors, 1); len(sub) != 1 || sub[0].Text != "The Wayfarers" || sub[0].SrcChar != 0 {
		t.Errorf("subtitle runs %+v", sub)
	}
	if sec := runsOf(anchors, at("## Verse 1")); len(sec) != 1 || sec[0].Text != "Verse 1" || sec[0].SrcChar != 3 {
		t.Errorf("section runs %+v", sec)
	}
	// Chord row + its (x2) note: one source line, two runs; the lyric under it is the NEXT source line.
	cr := at("G       C        D   (x2)")
	chord := runsOf(anchors, cr)
	if len(chord) != 2 || chord[0].SrcChar != 0 || chord[1].Text != "(x2)" || chord[1].SrcSeg != 1 ||
		chord[1].SrcChar != runeIdx(lines[cr], "(x2)") {
		t.Errorf("chord row runs %+v", chord)
	}
	if ly := runsOf(anchors, cr+1); len(ly) != 1 || ly[0].Text != lines[cr+1] || ly[0].SrcChar != 0 {
		t.Errorf("paired lyric runs %+v", ly)
	}
	// Bold splits: one run per segment, each at its own offset, `**` counted.
	bl := at("Some **bold** words and **more bold** here")
	want := []struct {
		text string
		off  int
	}{{"Some ", 0}, {"bold", 7}, {" words and ", 13}, {"more bold", 26}, {" here", 37}}
	got := runsOf(anchors, bl)
	if len(got) != len(want) {
		t.Fatalf("bold line: %d runs, want %d: %+v", len(got), len(want), got)
	}
	for i, w := range want {
		if got[i].Text != w.text || got[i].SrcChar != w.off || got[i].SrcSeg != i {
			t.Errorf("bold run %d = %q@%d seg %d, want %q@%d seg %d", i, got[i].Text, got[i].SrcChar, got[i].SrcSeg, w.text, w.off, i)
		}
	}
	// Offsets are RUNES, not bytes: "é" before "crème" is one character.
	if r := runsOf(anchors, at("Café **crème** brûlée")); len(r) != 3 || r[1].Text != "crème" || r[1].SrcChar != 7 {
		t.Errorf("non-ASCII bold runs %+v (want crème at rune 7, byte offset is 8)", r)
	}
	// Tab lines: each its own source line, at 0.
	for _, tl := range []string{"e|---0---3---|", "B|---1---1---|"} {
		if r := runsOf(anchors, at(tl)); len(r) != 1 || r[0].Text != tl || r[0].SrcChar != 0 {
			t.Errorf("tab line %q runs %+v", tl, r)
		}
	}
	// Footnote: the paragraph's first run starts on its first source line.
	if r := runsOf(anchors, at("Words and music by nobody in particular,")); len(r) == 0 || r[0].SrcChar != 0 {
		t.Errorf("footnote runs %+v", r)
	}
}

func TestManifestSrcLoc_IndentedHeaderAndSection(t *testing.T) {
	src := "  #   The Open Road\n\n  ##  Chorus\nKeep on rolling\n"
	anchors := renderLoc(t, src)
	if r := runsOf(anchors, 0); len(r) != 1 || r[0].SrcChar != 6 {
		t.Errorf("indented title %+v, want char 6", r)
	}
	if r := runsOf(anchors, 2); len(r) != 1 || r[0].Text != "Chorus" || r[0].SrcChar != 6 {
		t.Errorf("indented section %+v, want char 6", r)
	}
	// No `# ` line: the drawn "Chart" title comes from no line.
	a2 := renderLoc(t, "just a line\n")
	if a2[0].Text != "Chart" || a2[0].SrcLine != -1 {
		t.Errorf("default title %+v", a2[0])
	}
}

// A long line at a large size wraps (T168): several runs, one source line, SrcWrap 0,1,…, each continuation
// at the offset of its first word — for prose (with bold across the wrap) and for a chord+lyric pair.
func TestManifestSrcLoc_WrappedLongLine(t *testing.T) {
	long := "We drive the open road from the morning until the **evening light comes down on the** long grey hills and the rivers run to the sea"
	chords := "G       C        D        Em       C        G        D        Em       C        G        D"
	lyric := "Oh the road is calling and the wind is blowing and the night is falling and the day is gone away"
	src := "# The Open Road\nsize: 16\n\n" + long + "\n" + chords + "\n" + lyric + "\n"
	anchors := renderLoc(t, src)
	pl := lineIndex(t, src, long)
	runs := runsOf(anchors, pl)
	wraps := map[int]bool{}
	for i, r := range runs {
		wraps[r.SrcWrap] = true
		if i > 0 && r.SrcChar <= runs[i-1].SrcChar {
			t.Errorf("prose run %d starts at %d, not after %d", i, r.SrcChar, runs[i-1].SrcChar)
		}
	}
	if len(wraps) < 2 {
		t.Fatalf("fixture: the long line did not wrap (%d visual lines): %+v", len(wraps), runs)
	}
	cl := lineIndex(t, src, chords)
	cr, lr := runsOf(anchors, cl), runsOf(anchors, cl+1)
	if len(cr) < 2 || len(lr) < 2 {
		t.Fatalf("fixture: the pair did not wrap: chords %d runs, lyric %d runs", len(cr), len(lr))
	}
	for i := range lr {
		if lr[i].SrcWrap != i || (i > 0 && lr[i].SrcChar == 0) {
			t.Errorf("lyric continuation %d: wrap %d char %d", i, lr[i].SrcWrap, lr[i].SrcChar)
		}
	}
}

// Two columns: runs in the right column map back as well as the left ones, including a line that wraps in
// the narrower column.
func TestManifestSrcLoc_TwoColumns(t *testing.T) {
	var b strings.Builder
	b.WriteString("# The Open Road\ncolumns: 2\n\n")
	for v := 1; v <= 8; v++ {
		fmt.Fprintf(&b, "## Verse %d\n", v)
		b.WriteString("G       C        D\nWe drive the road until the evening comes and the long day is over\n")
		fmt.Fprintf(&b, "Line %d of **the** song\nAnother line for verse %d\n\n", v, v)
	}
	src := b.String()
	anchors := renderLoc(t, src)
	right := 0
	for _, a := range anchors {
		if a.X0 > 0.5 && a.SrcLine >= 0 {
			right++
		}
	}
	if right == 0 {
		t.Fatal("fixture: nothing in the right column")
	}
}

// Teeth: the checker catches a run whose offset is off by one, a wrong line, and a broken seg count.
func TestManifestSrcLoc_CheckerHasTeeth(t *testing.T) {
	_, anchors, err := RenderWithAnchors(locFixture)
	if err != nil {
		t.Fatal(err)
	}
	if e := srcLocErrors(locFixture, anchors); len(e) != 0 {
		t.Fatalf("baseline not clean: %v", e)
	}
	bold := lineIndex(t, locFixture, "Some **bold** words and **more bold** here")
	for name, mut := range map[string]func(a *Anchor){
		"char+1": func(a *Anchor) { a.SrcChar++ },
		"line+1": func(a *Anchor) { a.SrcLine++ },
		"seg=0":  func(a *Anchor) { a.SrcSeg = 0 },
	} {
		cp := append([]Anchor(nil), anchors...)
		for i := range cp {
			if cp[i].SrcLine == bold && cp[i].Text == "more bold" {
				mut(&cp[i])
			}
		}
		if e := srcLocErrors(locFixture, cp); len(e) == 0 {
			t.Errorf("%s: the checker did not notice", name)
		}
	}
}

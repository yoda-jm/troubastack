package chartpdf

import (
	"bytes"
	"errors"
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
			if s == "" || strings.HasPrefix(s, "# ") {
				continue // blank or the title line
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

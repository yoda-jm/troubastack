package chartpdf

import (
	"bytes"
	"errors"
	"os/exec"
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

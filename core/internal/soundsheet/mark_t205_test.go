package soundsheet

import (
	"strings"
	"testing"

	"troubastack/core/internal/pdfmark"
	"troubastack/core/internal/pdftest"
)

// T205: the sound sheet carries the mark by default and omits it when the band hides it.
func TestRenderPDF_Mark_T205(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	m := sampleModel(2, 2)

	on, err := RenderPDF(m)
	if err != nil {
		t.Fatal(err)
	}
	txt, err := pdftest.Text(on)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("mark absent by default:\n%s", txt)
	}

	m.HidePdfMark = true
	off, err := RenderPDF(m)
	if err != nil {
		t.Fatal(err)
	}
	txt2, err := pdftest.Text(off)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(txt2, pdfmark.DefaultText) {
		t.Errorf("mark present when hidden:\n%s", txt2)
	}
}

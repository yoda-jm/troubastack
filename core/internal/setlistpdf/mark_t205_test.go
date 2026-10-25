package setlistpdf

import (
	"strings"
	"testing"

	"troubastack/core/internal/pdfmark"
	"troubastack/core/internal/pdftest"
)

// T205: the running-order sheet carries the mark by default and omits it when the band hides it; deterministic.
func TestRender_Mark_T205(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	base := Doc{
		BandName:    "B",
		SetlistName: "S",
		Main:        []Row{{Number: 1, Title: "One", Kind: KindSong}, {Number: 2, Title: "Two", Kind: KindSong}},
	}

	on, err := Render(base)
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

	hidden := base
	hidden.HidePdfMark = true
	off, err := Render(hidden)
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

	again, err := Render(base)
	if err != nil {
		t.Fatal(err)
	}
	if string(on) != string(again) {
		t.Error("not deterministic")
	}
}

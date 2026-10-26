package pdfmark_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"troubastack/core/internal/pdfmark"
	"troubastack/core/internal/pdftest"

	"github.com/go-pdf/fpdf"
)

// render a 2-page PDF with the mark wired through SetFooterFunc, as the real renderers do. Pin the dates +
// catalog sort exactly as the real exports (bake/pdf.go, setlistpdf, soundsheet, chartpdf) do — otherwise fpdf
// stamps time.Now() and TestDraw_Deterministic flakes across a second boundary (CI red on 2adf350a).
func twoPage(show bool) []byte {
	pdf := fpdf.New("P", "mm", "A4", "")
	fixed := time.Unix(1700000000, 0).UTC()
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true)
	pdf.SetFooterFunc(func() { pdfmark.Draw(pdf, show) })
	for i := 0; i < 2; i++ {
		pdf.AddPage()
		pdf.SetFont("Helvetica", "", 12)
		pdf.Cell(40, 10, "body")
	}
	var b bytes.Buffer
	_ = pdf.Output(&b)
	return b.Bytes()
}

func TestDraw_OnEveryPageWhenShown(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	pages, err := pdftest.Pages(twoPage(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(pages))
	}
	for i, p := range pages {
		if !strings.Contains(p, pdfmark.DefaultText) {
			t.Errorf("page %d missing the mark %q; got:\n%s", i+1, pdfmark.DefaultText, p)
		}
	}
}

func TestDraw_AbsentWhenHidden(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	txt, err := pdftest.Text(twoPage(false))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("mark present with show=false:\n%s", txt)
	}
}

func TestDraw_Deterministic(t *testing.T) {
	if !bytes.Equal(twoPage(true), twoPage(true)) {
		t.Error("same input produced different bytes")
	}
}

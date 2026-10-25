package bake

import (
	"context"
	"image/color"
	"strings"
	"testing"

	"troubastack/core/internal/pdfmark"
	"troubastack/core/internal/pdftest"
)

// T205: the concert print (writeA4PDF, shared with the file print) carries the mark by default and omits it when
// the band hides it. Toggling the band setting flips the print without a re-bake, because the renderer's caller
// reads the live flag (BandPdfMarkHidden), not the bundle.
func TestConcertPDF_Mark_T205(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	svc, eng, u, bandID, setlistID := seed(t)
	white := solidPNG(t, 40, 56, color.White)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: white},
		overlays: fakeOverlays{png: white},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
		t.Fatalf("bake: %v", err)
	}

	// Default (HidePdfMark zero = shown).
	pdf, err := b.ConcertPDF(setlistID, "", u.ID)
	if err != nil {
		t.Fatalf("ConcertPDF: %v", err)
	}
	if txt, _ := pdftest.Text(pdf); !strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("mark absent by default:\n%s", txt)
	}

	// Hide it — same bake, no re-bake needed.
	hide := true
	if _, err := svc.UpdateBand(u, bandID, nil, &hide); err != nil {
		t.Fatalf("UpdateBand: %v", err)
	}
	pdf2, err := b.ConcertPDF(setlistID, "", u.ID)
	if err != nil {
		t.Fatalf("ConcertPDF (hidden): %v", err)
	}
	if txt, _ := pdftest.Text(pdf2); strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("mark present when hidden:\n%s", txt)
	}
}

// T205 (Fable binding): the FILE print path is wired too. ConcertPDF and FilePDF share writeA4PDF, but each
// resolves showMark at its own call site — this exercises filepdf.go's `!BandPdfMarkHidden(bandID)` directly,
// and goes red if that showMark is forced off.
func TestFilePDF_Mark_T205(t *testing.T) {
	if !pdftest.Available() {
		t.Skip("pdftotext not available")
	}
	b, svc, u, bandID, songID, fileID := filePDFFixture(t, 1)
	all := func(string) bool { return true }

	pdf, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, all)
	if err != nil {
		t.Fatalf("FilePDF: %v", err)
	}
	if txt, _ := pdftest.Text(pdf); !strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("file print missing the mark by default:\n%s", txt)
	}

	hide := true
	if _, err := svc.UpdateBand(u, bandID, nil, &hide); err != nil {
		t.Fatalf("UpdateBand: %v", err)
	}
	pdf2, err := b.FilePDF(context.Background(), bandID, songID, fileID, u, all)
	if err != nil {
		t.Fatalf("FilePDF (hidden): %v", err)
	}
	if txt, _ := pdftest.Text(pdf2); strings.Contains(txt, pdfmark.DefaultText) {
		t.Errorf("file print carries the mark when hidden:\n%s", txt)
	}
}

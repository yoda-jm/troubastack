package setlistpdf

import (
	"bytes"
	"testing"
)

// A real, non-empty A4 PDF comes out, with accented names, a numbered order, an inline intermission and a
// bench — exercising every branch (optional header lines present, intermission label, on-call section).
func TestRender_ProducesPDF(t *testing.T) {
	pdf, err := Render(Doc{
		BandName:    "Café Fanfare", // accented → exercises the cp1252 translator
		SetlistName: "Fête de la Musique",
		Venue:       "Le Sous-sol",
		EventDate:   "2026-09-21",
		Main: []Row{
			{Number: 1, Title: "Opener", Kind: KindSong},
			{Number: 2, Title: "Deuxième Chanson", Kind: KindSong},
			{Number: 0, Title: "Intermission", Kind: KindIntermission}, // inline, unnumbered
			{Number: 3, Title: "Closer", Kind: KindSong},
		},
		OnCall: []Row{{Number: 0, Title: "Encore Maybe", Kind: KindSong}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("output is not a PDF (%d bytes)", len(pdf))
	}
	if len(pdf) < 500 {
		t.Fatalf("PDF suspiciously small: %d bytes", len(pdf))
	}
	_ = bytes.Count(pdf, []byte("/Type")) // touch to keep bytes import meaningful across branches
}

// Optional header lines omitted (no venue, no date) and no bench still renders a valid PDF — the empty
// cases the header's difficulty lives in.
func TestRender_OmitsAbsentOptionalLines(t *testing.T) {
	pdf, err := Render(Doc{
		BandName:    "Band",
		SetlistName: "Rehearsal",
		Main:        []Row{{Number: 1, Title: "Only Song", Kind: KindSong}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatal("output is not a PDF")
	}
}

// Command mkcharts generates the demo "chart" PDFs used as test/demo artifacts and
// for screenshots: a lead sheet + guitar tab, a public-domain piece, and a generic
// placeholder. Pure Go (go-pdf/fpdf), deterministic output.
//
// COPYRIGHT: everything here is either ORIGINAL (written for this project) or
// PUBLIC DOMAIN (traditional works out of copyright). No copyrighted song lyrics,
// tab, or sheet music are reproduced — that is deliberate, not an oversight.
//
//	go run ./cmd/mkcharts -out ../docs/demo-charts
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/go-pdf/fpdf"
)

func main() {
	out := flag.String("out", "", "output directory for the chart PDFs")
	flag.Parse()
	if *out == "" {
		log.Fatal("mkcharts: pass -out <dir>")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("mkcharts: %v", err)
	}
	for name, build := range map[string]func() *fpdf.Fpdf{
		"open-road-leadsheet.pdf": openRoadLeadSheet, // A: ORIGINAL lead sheet + tab
		"amazing-grace.pdf":       amazingGrace,      // B: PUBLIC DOMAIN (1779 hymn)
		"blank-chart.pdf":         blankChart,        // C: generic placeholder
	} {
		if err := write(filepath.Join(*out, name), build()); err != nil {
			log.Fatalf("mkcharts %s: %v", name, err)
		}
		fmt.Println("wrote", name)
	}
}

func write(path string, pdf *fpdf.Fpdf) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return pdf.Output(f)
}

// --- shared helpers -------------------------------------------------------

const (
	pageW  = 210.0
	margin = 18.0
	right  = pageW - margin
)

func newDoc(title string) (*fpdf.Fpdf, func(string) string) {
	pdf := fpdf.New("P", "mm", "A4", "")
	// Pin the creation/mod date so regenerating is byte-deterministic (fpdf otherwise
	// stamps "now"). Fixed epoch — the charts have no real "baked at" meaning.
	fixed := time.Unix(1700000000, 0).UTC()
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true) // sort resource dicts → deterministic byte output (fpdf maps otherwise vary)
	// We place everything with explicit SetXY, so disable auto page-break — otherwise
	// the manually-positioned footer near the page bottom spills onto a blank page.
	pdf.SetAutoPageBreak(false, 0)
	// tr: UTF-8 → cp1252 so em-dashes/accents render (same rule as cmd/seed, T16).
	tr := pdf.UnicodeTranslatorFromDescriptor("")
	pdf.SetTitle(title, true)
	return pdf, tr
}

// header draws the title block: big title, then a "key • tempo • meter" line.
func header(pdf *fpdf.Fpdf, tr func(string) string, title, sub, meta string) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetXY(margin, 16)
	pdf.Cell(0, 10, tr(title))
	if sub != "" {
		pdf.SetFont("Helvetica", "I", 12)
		pdf.SetXY(margin, 27)
		pdf.Cell(0, 7, tr(sub))
	}
	if meta != "" {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetXY(margin, 35)
		pdf.SetTextColor(90, 90, 90)
		pdf.Cell(0, 6, tr(meta))
		pdf.SetTextColor(0, 0, 0)
	}
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, 43, right, 43)
}

// chordLine prints a monospaced chord row (bold) and the lyric row beneath it —
// the classic "chords above the words" lead-sheet layout. y is the baseline of the
// chord row in mm; returns the y after the lyric row.
func chordLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, chords, lyric string) float64 {
	pdf.SetFont("Courier", "B", 11)
	pdf.SetTextColor(20, 60, 150) // chords in blue, like a real chart
	pdf.SetXY(margin, y)
	pdf.Cell(0, 5, chords)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Courier", "", 11)
	pdf.SetXY(margin, y+5)
	pdf.Cell(0, 5, tr(lyric))
	return y + 11.5
}

// sectionLabel prints a section header (e.g. "Verse 1").
func sectionLabel(pdf *fpdf.Fpdf, y float64, label string) float64 {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, y)
	pdf.Cell(0, 6, label)
	pdf.SetTextColor(0, 0, 0)
	return y + 7.5
}

// --- A: ORIGINAL lead sheet + guitar tab ("The Open Road") ----------------
//
// Lyrics, chords and the tab riff below are ORIGINAL — written for this demo.

func openRoadLeadSheet() *fpdf.Fpdf {
	pdf, tr := newDoc("The Open Road — Lead Sheet")

	// Page 1: lead sheet (chords over original lyrics).
	header(pdf, tr, "The Open Road", "original demo song", "Key: G major   •   Tempo: 92 bpm   •   4/4   •   Capo 2")
	y := 50.0
	y = sectionLabel(pdf, y, "Verse 1")
	y = chordLine(pdf, tr, y, "G            D", "Pack a little light for the road ahead,")
	y = chordLine(pdf, tr, y, "Em           C", "leave the rest of yesterday unsaid.")
	y = chordLine(pdf, tr, y, "G            D", "Every mile a page we haven't read,")
	y = chordLine(pdf, tr, y, "Em      C        G", "the open road is calling us instead.")
	y += 3
	y = sectionLabel(pdf, y, "Chorus")
	y = chordLine(pdf, tr, y, "C           G", "So drive, drive into the wide unknown,")
	y = chordLine(pdf, tr, y, "D              Em", "wherever we are going we're not going alone.")
	y = chordLine(pdf, tr, y, "C           G", "Sing loud, let the engine and the heart atone —")
	y = chordLine(pdf, tr, y, "D                 G", "the map is just a rumour once the wheels have grown.")
	y += 3
	y = sectionLabel(pdf, y, "Verse 2")
	y = chordLine(pdf, tr, y, "G            D", "Coffee going cold and the radio low,")
	y = chordLine(pdf, tr, y, "Em            C", "counting every town we'll never know.")
	y = chordLine(pdf, tr, y, "G             D", "Headlights on a hill we used to know —")
	_ = chordLine(pdf, tr, y, "Em       C         G", "the only way to stay is letting go.")
	footer(pdf, tr)

	// Page 2: the intro riff as guitar tab (ORIGINAL, monospaced).
	header(pdf, tr, "The Open Road — Intro Riff (Tab)", "original demo song", "Standard tuning (EADGBe)   •   let ring   •   ~92 bpm")
	tabLines := []string{
		"e|-----------------0-------------------0---------------|",
		"B|-------0-----------------3-------0-------------3------|",
		"G|---0-------0---------0-------0-------0-----0----------|",
		"D|-------------------------------------------------2---|",
		"A|-2---------------3-----------------2-----------------|",
		"E|-3---------------------------------3-----------------|",
	}
	yy := 54.0
	pdf.SetFont("Courier", "", 10)
	for _, ln := range tabLines {
		pdf.SetXY(margin, yy)
		pdf.Cell(0, 5, ln)
		yy += 6
	}
	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetXY(margin, yy+6)
	pdf.MultiCell(right-margin, 5, tr("Chord shapes under the riff: G (320003) · D (xx0232) · Em (022000) · C (x32010). "+
		"Play the riff twice, then land on G to start Verse 1."), "", "L", false)
	footer(pdf, tr)

	return pdf
}

// --- B: PUBLIC DOMAIN — "Amazing Grace" (John Newton, 1779) -----------------
//
// A hymn published in 1779; its text is long out of copyright (public domain).
// Only a simple demo chord accompaniment is added.

func amazingGrace() *fpdf.Fpdf {
	pdf, tr := newDoc("Amazing Grace — Lead Sheet")
	header(pdf, tr, "Amazing Grace", "words: John Newton, 1779 (public domain) · arr. demo", "Key: G major   •   Tempo: 72 bpm   •   3/4")
	y := 50.0
	y = sectionLabel(pdf, y, "Verse 1")
	y = chordLine(pdf, tr, y, "G          G7       C     G", "Amazing grace, how sweet the sound,")
	y = chordLine(pdf, tr, y, "G                    D", "That saved a wretch like me.")
	y = chordLine(pdf, tr, y, "G           G7      C      G", "I once was lost, but now am found,")
	y = chordLine(pdf, tr, y, "Em        D        G", "Was blind, but now I see.")
	y += 4
	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetXY(margin, y)
	pdf.MultiCell(right-margin, 5, tr("Text by John Newton (1779) — public domain. The chord accompaniment above is a "+
		"simple demo arrangement."), "", "L", false)
	footer(pdf, tr)
	return pdf
}

// --- C: generic placeholder chart (no real song content) -------------------

func blankChart() *fpdf.Fpdf {
	pdf, tr := newDoc("Blank Chart — Placeholder")
	header(pdf, tr, "Untitled Chart", "generic placeholder — no song content", "Key: —   •   Tempo: —   •   4/4")
	// A few empty "systems": bar lines over five staff lines each.
	pdf.SetLineWidth(0.2)
	y := 58.0
	for sys := 0; sys < 6; sys++ {
		for line := 0; line < 5; line++ {
			pdf.Line(margin, y+float64(line)*2.0, right, y+float64(line)*2.0)
		}
		// bar lines every quarter of the width
		for b := 0; b <= 4; b++ {
			x := margin + float64(b)*(right-margin)/4
			pdf.Line(x, y, x, y+8)
		}
		y += 28
	}
	// A row of blank chord boxes at the bottom.
	pdf.SetFont("Helvetica", "", 9)
	bx := margin
	for i := 0; i < 6; i++ {
		pdf.Rect(bx, 240, 22, 26, "D")
		bx += 28
	}
	footer(pdf, tr)
	return pdf
}

func footer(pdf *fpdf.Fpdf, tr func(string) string) {
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(140, 140, 140)
	pdf.SetXY(margin, 285)
	pdf.Cell(0, 6, tr("TroubaStack demo chart — original/public-domain content, free to ship."))
	pdf.SetTextColor(0, 0, 0)
}

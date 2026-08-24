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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/go-pdf/fpdf"

	"troubastack/core/internal/chartpdf"
)

// --- anchor manifest (B13) ------------------------------------------------
//
// Every text run mkcharts draws is recorded as its bounding box in [0,1]² page
// coords, emitted beside each PDF as <name>.anchors.json. The seed places demo
// annotations by looking these up (page, substring, occurrence) instead of
// hand-tuned magic numbers, so a highlight provably covers its word. Generated
// charts are exact by construction (fpdf cursor + GetStringWidth).

const (
	pageWmm = 210.0 // A4 width  (mm)
	pageHmm = 297.0 // A4 height (mm)
)

type anchor struct {
	Page int     `json:"page"` // 0-indexed
	Text string  `json:"text"`
	X0   float64 `json:"x0"`
	Y0   float64 `json:"y0"`
	X1   float64 `json:"x1"`
	Y1   float64 `json:"y1"`
}

// anchors accumulates the runs of the PDF currently being built; reset per file.
var anchors []anchor

// rec records a text run's box. (x,y) is its top-left in mm, (w,h) its size in mm.
func rec(pdf *fpdf.Fpdf, text string, x, y, w, h float64) {
	anchors = append(anchors, anchor{
		Page: pdf.PageNo() - 1,
		Text: text,
		X0:   x / pageWmm, Y0: y / pageHmm,
		X1: (x + w) / pageWmm, Y1: (y + h) / pageHmm,
	})
}

func writeAnchors(path string) error {
	// Stable order: page, then top-to-bottom, then left-to-right — deterministic JSON.
	sort.SliceStable(anchors, func(i, j int) bool {
		a, b := anchors[i], anchors[j]
		if a.Page != b.Page {
			return a.Page < b.Page
		}
		if a.Y0 != b.Y0 {
			return a.Y0 < b.Y0
		}
		return a.X0 < b.X0
	})
	b, err := json.MarshalIndent(anchors, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func main() {
	out := flag.String("out", "", "output directory for the chart PDFs")
	flag.Parse()
	if *out == "" {
		log.Fatal("mkcharts: pass -out <dir>")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("mkcharts: %v", err)
	}
	builds := map[string]func() *fpdf.Fpdf{
		"open-road-leadsheet.pdf":    openRoadLeadSheet, // A: ORIGINAL lead sheet
		"open-road-guitar.pdf":       openRoadGuitar,    // A2: ORIGINAL guitar part (intro riff)
		"blank-chart.pdf":            blankChart,        // C: generic placeholder (converges later)
		"house-rising-sun-tab.pdf":   houseTab,          // D: PUBLIC DOMAIN — guitar tab
		"house-rising-sun-drums.pdf": houseDrums,        // E: PUBLIC DOMAIN — drum groove
	}
	// T95 Stage B: charts the dialect CAN express are the .chart source of truth, rendered through the
	// productized `chartpdf` (so they inherit its auto-fit/compaction/breaks) rather than a duplicate
	// hand-drawn mkcharts builder. Their `.pdf` + `.anchors.json` are regenerated from `<base>.chart`.
	chartBuilds := map[string]string{
		"amazing-grace": "amazing-grace.chart", // B: PUBLIC DOMAIN (1779 hymn)
	}
	// Deterministic order so the run log is stable (each file's bytes are independent anyway).
	names := make([]string, 0, len(builds))
	for name := range builds {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		anchors = nil // reset the manifest for this file
		if err := write(filepath.Join(*out, name), builds[name]()); err != nil {
			log.Fatalf("mkcharts %s: %v", name, err)
		}
		anchorsPath := filepath.Join(*out, name[:len(name)-len(".pdf")]+".anchors.json")
		if err := writeAnchors(anchorsPath); err != nil {
			log.Fatalf("mkcharts %s anchors: %v", name, err)
		}
		fmt.Printf("wrote %s (+%d anchors)\n", name, len(anchors))
	}
	chartNames := make([]string, 0, len(chartBuilds))
	for base := range chartBuilds {
		chartNames = append(chartNames, base)
	}
	sort.Strings(chartNames)
	for _, base := range chartNames {
		if err := writeChartFromSource(*out, base, chartBuilds[base]); err != nil {
			log.Fatalf("mkcharts %s: %v", base, err)
		}
	}
}

// writeChartFromSource renders a committed `.chart` dialect source through chartpdf and writes
// <base>.pdf + <base>.anchors.json — the SAME renderer + anchor manifest the server produces, so a
// demo annotation placed against these anchors lands on the live-rendered chart identically (T95 §3).
func writeChartFromSource(outDir, base, chartFile string) error {
	src, err := os.ReadFile(filepath.Join(outDir, chartFile))
	if err != nil {
		return fmt.Errorf("read chart source: %w", err)
	}
	pdf, chAnchors, err := chartpdf.RenderWithAnchors(string(src))
	if err != nil {
		return fmt.Errorf("render chart: %w", err)
	}
	if err := os.WriteFile(filepath.Join(outDir, base+".pdf"), pdf, 0o644); err != nil {
		return err
	}
	// chartpdf.Anchor already carries the exact json tags of the manifest; write it verbatim + sorted.
	b, err := json.MarshalIndent(chAnchors, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(outDir, base+".anchors.json"), append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("wrote %s.pdf via chartpdf (+%d anchors)\n", base, len(chAnchors))
	return nil
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
	rec(pdf, title, margin, 16, pdf.GetStringWidth(tr(title)), 10)
	if sub != "" {
		pdf.SetFont("Helvetica", "I", 12)
		pdf.SetXY(margin, 27)
		pdf.Cell(0, 7, tr(sub))
		rec(pdf, sub, margin, 27, pdf.GetStringWidth(tr(sub)), 7)
	}
	if meta != "" {
		pdf.SetFont("Helvetica", "", 10)
		pdf.SetXY(margin, 35)
		pdf.SetTextColor(90, 90, 90)
		pdf.Cell(0, 6, tr(meta))
		rec(pdf, meta, margin, 35, pdf.GetStringWidth(tr(meta)), 6)
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
	rec(pdf, chords, margin, y, pdf.GetStringWidth(chords), 5)
	pdf.SetTextColor(0, 0, 0)
	pdf.SetFont("Courier", "", 11)
	pdf.SetXY(margin, y+5)
	pdf.Cell(0, 5, tr(lyric))
	rec(pdf, lyric, margin, y+5, pdf.GetStringWidth(tr(lyric)), 5)
	return y + 11.5
}

// sectionLabel prints a section header (e.g. "Verse 1"). Like every other text helper it
// draws through tr (UTF-8 → cp1252, T16) but records the ORIGINAL string as the anchor, so
// a label carrying an em-dash renders as one and is still found by its Go text.
func sectionLabel(pdf *fpdf.Fpdf, tr func(string) string, y float64, label string) float64 {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, y)
	pdf.Cell(0, 6, tr(label))
	rec(pdf, label, margin, y, pdf.GetStringWidth(tr(label)), 6)
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
	y = sectionLabel(pdf, tr, y, "Verse 1")
	y = chordLine(pdf, tr, y, "G            D", "Pack a little light for the road ahead,")
	y = chordLine(pdf, tr, y, "Em           C", "leave the rest of yesterday unsaid.")
	y = chordLine(pdf, tr, y, "G            D", "Every mile a page we haven't read,")
	y = chordLine(pdf, tr, y, "Em      C        G", "the open road is calling us instead.")
	y += 3
	y = sectionLabel(pdf, tr, y, "Chorus")
	y = chordLine(pdf, tr, y, "C           G", "So drive, drive into the wide unknown,")
	y = chordLine(pdf, tr, y, "D              Em", "wherever we are going we're not going alone.")
	y = chordLine(pdf, tr, y, "C           G", "Sing loud, let the engine and the heart atone —")
	y = chordLine(pdf, tr, y, "D                 G", "the map is just a rumour once the wheels have grown.")
	y += 3
	y = sectionLabel(pdf, tr, y, "Verse 2")
	y = chordLine(pdf, tr, y, "G            D", "Coffee going cold and the radio low,")
	y = chordLine(pdf, tr, y, "Em            C", "counting every town we'll never know.")
	y = chordLine(pdf, tr, y, "G             D", "Headlights on a hill we used to know —")
	_ = chordLine(pdf, tr, y, "Em       C         G", "the only way to stay is letting go.")
	footer(pdf, tr)

	// Page 2 (B13 §3b): the intro riff tab + a performance-note line (marks 12–13 target).
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 18)
	pdf.SetXY(margin, 18)
	pdf.Cell(0, 10, tr("The Open Road — Intro Riff"))
	rec(pdf, "The Open Road — Intro Riff", margin, 18, pdf.GetStringWidth(tr("The Open Road — Intro Riff")), 10)
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, 32, right, 32)
	y2 := sectionLabel(pdf, tr, 42, "Riff")
	riff := []string{
		"e|-----------------0-------------------0---------------|",
		"B|-------0-----------------3-------0-------------3------|",
		"G|---0-------0---------0-------0-------0-----0----------|",
		"A|-2---------------3-----------------2-----------------|",
		"E|-3---------------------------------3-----------------|",
	}
	pdf.SetFont("Courier", "", 9)
	for i, ln := range riff {
		pdf.SetXY(margin, y2)
		pdf.Cell(0, 5, ln)
		if i == 0 { // anchor the top tab line (mark 12 highlights its bar 1)
			rec(pdf, ln, margin, y2, pdf.GetStringWidth(ln), 5)
		}
		y2 += 5.6
	}
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, y2+8)
	note := "Riff: play 4x, build each time"
	pdf.Cell(0, 6, tr(note))
	rec(pdf, note, margin, y2+8, pdf.GetStringWidth(tr(note)), 6)
	pdf.SetTextColor(0, 0, 0)
	footer(pdf, tr)
	return pdf
}

// openRoadGuitar is the GUITARIST's part — the intro riff as tab, on its own sheet (it plays
// BEFORE the song, so it belongs with the guitar part, not after the whole lead sheet). The
// riff, chords and tuning are ORIGINAL. Seeded as a second file in The Open Road's pool.
// barChart draws `bars` as boxed measures, 4 per line, from (margin, y); each measure
// shows its chord(s). Returns the y below the chart — a "how many bars of what chord"
// guitar chart (the format VLL asked for).
func barChart(pdf *fpdf.Fpdf, tr func(string) string, y float64, bars []string) float64 {
	const perLine = 4
	const h = 13.0
	w := (right - margin) / perLine
	pdf.SetLineWidth(0.3)
	for i, ch := range bars {
		x := margin + float64(i%perLine)*w
		yy := y + float64(i/perLine)*h
		pdf.Rect(x, yy, w, h, "D")
		pdf.SetFont("Helvetica", "B", 13)
		pdf.SetXY(x, yy+3.5)
		pdf.CellFormat(w, 6, tr(ch), "", 0, "C", false, 0, "")
		rec(pdf, ch, x, yy, w, h) // the bar cell (box) — chord centered inside
	}
	rows := (len(bars) + perLine - 1) / perLine
	return y + float64(rows)*h
}

func openRoadGuitar() *fpdf.Fpdf {
	pdf, tr := newDoc("The Open Road — Guitar")
	header(pdf, tr, "The Open Road — Guitar", "original demo song · guitar chart", "Standard tuning (EADGBe)   •   Capo 2   •   ~92 bpm · 4/4")

	// A compact intro riff, then the chord chart (bars per chord) for verse + chorus.
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, 50)
	pdf.Cell(0, 6, tr("Intro riff — play twice"))
	pdf.SetTextColor(0, 0, 0)
	tabLines := []string{
		"e|-----------------0-------------------0---------------|",
		"B|-------0-----------------3-------0-------------3------|",
		"G|---0-------0---------0-------0-------0-----0----------|",
		"A|-2---------------3-----------------2-----------------|",
		"E|-3---------------------------------3-----------------|",
	}
	yy := 60.0
	pdf.SetFont("Courier", "", 9)
	for _, ln := range tabLines {
		pdf.SetXY(margin, yy)
		pdf.Cell(0, 5, ln)
		yy += 5.2
	}

	// Verse chord chart — 8 bars.
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, yy+8)
	pdf.Cell(0, 6, tr("Verse — 8 bars"))
	pdf.SetTextColor(0, 0, 0)
	yy = barChart(pdf, tr, yy+16, []string{"G", "D", "Em", "C", "G", "D", "Em  C", "G"})

	// Chorus chord chart — 8 bars.
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, yy+8)
	pdf.Cell(0, 6, tr("Chorus — 8 bars"))
	pdf.SetTextColor(0, 0, 0)
	yy = barChart(pdf, tr, yy+16, []string{"C", "G", "D", "Em", "C", "G", "D", "G"})

	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetXY(margin, yy+8)
	pdf.MultiCell(right-margin, 5, tr("Chord shapes: G (320003) · D (xx0232) · Em (022000) · C (x32010). "+
		"Capo 2 (written pitch). Play the riff twice, then Verse."), "", "L", false)
	footer(pdf, tr)
	return pdf
}

// --- D: PUBLIC DOMAIN — "House of the Rising Sun" guitar tab ----------------
//
// A traditional American folk song (public domain, no known author). The
// arpeggiated 6/8 fingerpicking pattern below is a plain, generic demo
// arrangement of the standard Am–C–D–F–Am–E changes — no copyrighted
// transcription is reproduced.

func houseTab() *fpdf.Fpdf {
	pdf, tr := newDoc("House of the Rising Sun — Guitar Tab")
	header(pdf, tr, "House of the Rising Sun — Guitar", "traditional (public domain) · arr. demo",
		"Key: A minor   •   Tempo: 72 bpm   •   6/8   •   Standard tuning (EADGBe)")

	// One verse of the arpeggiated 6/8 pattern over the standard changes. Each block is
	// one chord's broken arpeggio (bass note + rolled treble), the classic picking feel.
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, 50)
	pdf.Cell(0, 6, tr("Verse — arpeggio pattern"))
	pdf.SetTextColor(0, 0, 0)

	tab := []string{
		"      Am        C         D         F         Am        E",
		"e|----0-----|----0-----|----2-----|----1-----|----0-----|----0-----|",
		"B|--1---1---|--1---1---|--3---3---|--1---1---|--1---1---|--0---0---|",
		"G|-2-----2--|-0-----0--|-2-----2--|-2-----2--|-2-----2--|-1-----1--|",
		"D|2---------|2---------|0---------|3---------|2---------|2---------|",
		"A|0---------|3---------|----------|----------|0---------|2---------|",
		"E|----------|----------|----------|1---------|----------|0---------|",
	}
	yy := 60.0
	pdf.SetFont("Courier", "", 9)
	for i, ln := range tab {
		if i == 0 {
			pdf.SetTextColor(20, 60, 150)
		} else {
			pdf.SetTextColor(0, 0, 0)
		}
		pdf.SetXY(margin, yy)
		pdf.Cell(0, 5, ln)
		if i == 0 { // anchor the chord-name row (F barre, E turnaround live here)
			rec(pdf, ln, margin, yy, pdf.GetStringWidth(ln), 5)
		}
		yy += 6
	}
	pdf.SetTextColor(0, 0, 0)

	// B13 §3b — a Chorus arpeggio variation (D→F adjacent for the "quick change" mark) + an Outro.
	yy = sectionLabel(pdf, tr, yy+6, "Chorus — arpeggio variation")
	pdf.SetFont("Courier", "", 9)
	pdf.SetTextColor(20, 60, 150)
	chorusRow := "      Am        C         D    F         Am        E"
	pdf.SetXY(margin, yy)
	pdf.Cell(0, 5, chorusRow)
	rec(pdf, chorusRow, margin, yy, pdf.GetStringWidth(chorusRow), 5)
	pdf.SetTextColor(0, 0, 0)
	yy += 6
	chorusTab := []string{
		"e|----0-----|----0-----|----2-----|----1-----|----0-----|----0-----|",
		"B|--1---1---|--1---1---|--3--1----|--1---1---|--1---1---|--0---0---|",
		"G|-2-----2--|-0-----0--|-2--2-----|-2-----2--|-2-----2--|-1-----1--|",
		"D|2---------|2---------|0--3------|----------|2---------|2---------|",
	}
	for _, ln := range chorusTab {
		pdf.SetXY(margin, yy)
		pdf.Cell(0, 5, ln)
		yy += 6
	}
	yy = sectionLabel(pdf, tr, yy+4, "Outro: let the last Am ring — fermata")

	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetXY(margin, yy+4)
	pdf.MultiCell(right-margin, 5, tr("Chord shapes: Am (x02210) · C (x32010) · D (xx0232) · F (133211) · E (022100). "+
		"Let each arpeggio ring; keep the 6/8 lilt. Repeat for each verse."), "", "L", false)
	footer(pdf, tr)
	return pdf
}

// --- E: PUBLIC DOMAIN — "House of the Rising Sun" drum groove ---------------
//
// A generic 6/8 groove box for the same traditional song — a common way drummers
// notate a feel (hi-hat / snare / kick grid). Original demo notation.

func houseDrums() *fpdf.Fpdf {
	pdf, tr := newDoc("House of the Rising Sun — Drum Groove")
	header(pdf, tr, "House of the Rising Sun — Drums", "traditional (public domain) · arr. demo",
		"Key: A minor   •   Tempo: 72 bpm   •   6/8   •   swung eighths")

	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, 50)
	pdf.Cell(0, 6, tr("Groove — one bar of 6/8 (count: 1 2 3 4 5 6)"))
	pdf.SetTextColor(0, 0, 0)

	// A monospace groove grid: x = hit, . = rest. Six eighth-note slots.
	grid := []string{
		"        1   2   3   4   5   6",
		"Hi-hat  x   x   x   x   x   x",
		"Snare   .   .   .   x   .   .",
		"Kick    x   .   .   .   .   x",
	}
	yy := 62.0
	pdf.SetFont("Courier", "", 12)
	for _, ln := range grid {
		pdf.SetXY(margin, yy)
		pdf.Cell(0, 6, ln)
		rec(pdf, ln, margin, yy, pdf.GetStringWidth(ln), 6) // anchor each groove-grid row
		yy += 8
	}
	pdf.SetFont("Helvetica", "I", 10)
	pdf.SetXY(margin, yy+6)
	pdf.MultiCell(right-margin, 5, tr("Ride the 6/8 with steady eighths on the hi-hat, snare on beat 4, kick on 1 and 6. "+
		"Open the hi-hat on the last '6' into each verse. Keep it soft under the vocal."), "", "L", false)
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

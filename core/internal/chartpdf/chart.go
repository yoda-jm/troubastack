// Package chartpdf renders a tiny plain-text "chart dialect" to a PDF — the T19
// productization of the demo chart renderer that lived in cmd/mkcharts. A member
// types lyrics/chords/sections in Studio; the server renders them to a PDF that
// enters the song's file pool, after which everything downstream (viewing,
// annotations, my-files, bake, Stage) works unchanged.
//
// Dialect (deliberately tiny — NOT Markdown):
//
//	# Title            document title (first wins; also the PDF metadata title)
//	## Section         a section header (Verse / Chorus / Bridge / …)
//	<chords>           a line whose tokens are ALL chords renders monospace-bold…
//	<lyric>            …above the next line as the classic "chords over words"
//	**bold**           inline bold within a normal text line
//	(blank line)       paragraph gap
//	anything else      literal text
//
// Input is ASCII + Latin-1 (the PDF core fonts are cp1252). A few common
// typographic runes (en/em dash, curly quotes, ellipsis, bullet) are allowed and
// mapped; anything else is rejected with ErrUnsupportedChar rather than rendered
// as mojibake.
package chartpdf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
)

// ErrUnsupportedChar is returned by Render when the source contains a rune the
// cp1252 renderer cannot represent. The error message names the offending rune.
var ErrUnsupportedChar = errors.New("chartpdf: unsupported character")

const (
	pageW      = 210.0 // A4 width (mm)
	pageH      = 297.0
	margin     = 18.0
	right      = pageW - margin
	bodyTop    = 50.0  // first body line, just below the title rule
	pageBottom = 282.0 // add a page once a line would cross this
)

// chordToken matches a single chord like G, Em, C#m7, Dsus4, A7sus4, F/G, N.C.
var chordToken = regexp.MustCompile(`^(N\.C\.|[A-G](#|b)?([0-9]|maj|min|m|dim|aug|sus|add)*(/[A-G](#|b)?)?)$`)

// extraRunes are non-Latin-1 runes we still accept (and the cp1252 translator
// maps): en/em dash, curly single/double quotes, ellipsis, bullet.
var extraRunes = map[rune]bool{
	'–': true, '—': true, // – —
	'‘': true, '’': true, // ‘ ’
	'“': true, '”': true, // “ ”
	'…': true, '•': true, // … •
}

// Render parses the chart dialect in source and returns a deterministic A4 PDF.
func Render(source string) ([]byte, error) {
	if err := validateChars(source); err != nil {
		return nil, err
	}
	lines := strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")

	title := firstTitle(lines)
	pdf, tr := newDoc(title)
	header(pdf, tr, title)

	y := bodyTop
	page := func(need float64) {
		if y+need > pageBottom {
			pdf.AddPage()
			y = margin + 4
		}
	}

	for i := 0; i < len(lines); i++ {
		raw := lines[i]
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)

		switch {
		case strings.HasPrefix(trimmed, "# "):
			// Title line — already rendered in the header; skip.
			continue
		case strings.HasPrefix(trimmed, "## "):
			page(9)
			y = sectionLabel(pdf, y, strings.TrimSpace(trimmed[3:]))
		case trimmed == "":
			y += 4 // paragraph gap
		case isChordRow(trimmed) && i+1 < len(lines) && isLyric(lines[i+1]):
			page(12)
			y = chordLine(pdf, tr, y, line, strings.TrimRight(lines[i+1], " \t"))
			i++ // consumed the lyric line
		case isChordRow(trimmed):
			page(6)
			y = chordLine(pdf, tr, y, line, "")
		default:
			page(6)
			y = textLine(pdf, tr, y, line)
		}
	}

	return output(pdf)
}

// validateChars rejects any rune the cp1252 renderer can't represent (outside
// Latin-1 and not in the small typographic allowlist), naming the first one.
func validateChars(s string) error {
	for _, r := range s {
		if r == '\n' || r == '\t' || r < 0x100 || extraRunes[r] {
			continue
		}
		return fmt.Errorf("%w: %q (U+%04X) — charts are ASCII/Latin-1 only", ErrUnsupportedChar, r, r)
	}
	return nil
}

// Title returns the chart's title — the first `# Title` line, or "Chart" if none.
// Used to name the generated pool file.
func Title(source string) string {
	return firstTitle(strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n"))
}

// firstTitle returns the first `# Title` text, or "Chart" if none.
func firstTitle(lines []string) string {
	for _, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "# ") {
			return strings.TrimSpace(t[2:])
		}
	}
	return "Chart"
}

// isChordRow reports whether every whitespace-separated token on the line is a
// chord (so the line is a chord row, not lyrics).
func isChordRow(s string) bool {
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return false
	}
	for _, f := range fields {
		if !chordToken.MatchString(f) {
			return false
		}
	}
	return true
}

// isLyric reports whether a line can serve as the lyric under a chord row: it is
// non-blank, not a directive (#/##), and not itself a chord row.
func isLyric(raw string) bool {
	t := strings.TrimSpace(raw)
	if t == "" || strings.HasPrefix(t, "#") {
		return false
	}
	return !isChordRow(t)
}

// --- renderer primitives (extracted from cmd/mkcharts) --------------------

func newDoc(title string) (*fpdf.Fpdf, func(string) string) {
	pdf := fpdf.New("P", "mm", "A4", "")
	// Deterministic output: pin the date, sort resource dicts, and paginate
	// manually (no auto page-break spilling a footer onto a blank page — T19/seed fix).
	fixed := time.Unix(1700000000, 0).UTC()
	pdf.SetCreationDate(fixed)
	pdf.SetModificationDate(fixed)
	pdf.SetCatalogSort(true)
	pdf.SetAutoPageBreak(false, 0)
	tr := pdf.UnicodeTranslatorFromDescriptor("") // UTF-8 → cp1252 (em-dash etc.)
	pdf.SetTitle(title, true)
	return pdf, tr
}

func header(pdf *fpdf.Fpdf, tr func(string) string, title string) {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetXY(margin, 16)
	pdf.Cell(0, 10, tr(title))
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, 30, right, 30)
}

// sectionLabel prints a section header (e.g. "Verse 1") and returns the next y.
func sectionLabel(pdf *fpdf.Fpdf, y float64, label string) float64 {
	pdf.SetFont("Helvetica", "B", 11)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, y)
	pdf.Cell(0, 6, label)
	pdf.SetTextColor(0, 0, 0)
	return y + 8
}

// chordLine prints a monospaced blue chord row and the lyric beneath it.
func chordLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, chords, lyric string) float64 {
	pdf.SetFont("Courier", "B", 11)
	pdf.SetTextColor(20, 60, 150)
	pdf.SetXY(margin, y)
	pdf.Cell(0, 5, tr(chords))
	pdf.SetTextColor(0, 0, 0)
	if lyric != "" {
		pdf.SetFont("Courier", "", 11)
		pdf.SetXY(margin, y+5)
		pdf.Cell(0, 5, tr(lyric))
		return y + 11.5
	}
	return y + 6
}

// textLine renders a normal paragraph line in Helvetica, honoring inline **bold**.
func textLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, line string) float64 {
	pdf.SetXY(margin, y)
	bold := false
	for _, seg := range strings.Split(line, "**") {
		if seg != "" {
			if bold {
				pdf.SetFont("Helvetica", "B", 11)
			} else {
				pdf.SetFont("Helvetica", "", 11)
			}
			pdf.CellFormat(pdf.GetStringWidth(tr(seg)), 6, tr(seg), "", 0, "L", false, 0, "")
		}
		bold = !bold
	}
	return y + 6.5
}

func output(pdf *fpdf.Fpdf) ([]byte, error) {
	var buf sink
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.b, nil
}

type sink struct{ b []byte }

func (w *sink) Write(p []byte) (int, error) { w.b = append(w.b, p...); return len(p), nil }

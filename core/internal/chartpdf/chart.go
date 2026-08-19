// Package chartpdf renders a tiny plain-text "chart dialect" to a PDF — the T19
// productization of the demo chart renderer that lived in cmd/mkcharts. A member
// types lyrics/chords/sections in Studio; the server renders them to a PDF that
// enters the song's file pool, after which everything downstream (viewing,
// annotations, my-files, bake, Stage) works unchanged.
//
// Dialect (deliberately tiny — NOT Markdown):
//
//	# Title            document title (first wins; also the PDF metadata title)
//	<subtitle>         the line DIRECTLY under `# Title` (no blank between) is the
//	                   artist/subtitle — shown in the header, not the body. A blank
//	                   line after the title means the body has started (no subtitle).
//	## Section         a section header (Verse / Chorus / Bridge / …)
//	<chords>           a line whose tokens are ALL chords renders monospace-bold…
//	<lyric>            …above the next line as the classic "chords over words"
//	**bold**           inline bold within a normal text line
//	(blank line)       paragraph gap
//	anything else      literal text
//
// Subtitle examples — with an artist, and without:
//
//	# My Song            # My Song
//	The Artist
//	                     ## Verse 1
//	## Verse 1           …
//	…
//
// The left chart's "The Artist" is adjacent to the title → header subtitle. The
// right chart has a blank after the title → no subtitle, body starts at Verse 1.
//
// Header-block directives (T74): on the lines immediately after `# Title`, before the
// first blank line or `## section`, a line `size: N` sets the chart's font size (8–16 pt;
// out of range → default). Everything (header, sections, chords, lyrics, spacing) scales
// from it. It is the ONLY directive; any other `key: value` line is NOT a directive — it
// stays the subtitle/body — so an artist "Foo: Bar" is unaffected. The directive may sit
// before or after the artist line and is never printed in the body:
//
//	# My Song            # My Song
//	size: 13             The Artist
//	The Artist           size: 13
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
	"strconv"
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
	margin     = 12.0 // T75: 18→12 all round (still a safe binder edge) — ~12mm of height + wider column
	topMargin  = 12.0
	right      = pageW - margin
	pageBottom = pageH - margin // add a page once a line would cross this

	// T75 per-row advances at scale 1 (11 pt). Leading pulled toward typographic norms (was
	// 1.5–1.7×): lyric 1.35×, chord 1.28× of the ~3.9 mm type. The chord row is tighter (short,
	// mostly descender-free). measure() and the renderer share these — one source of truth.
	leadLyric     = 5.3 // lyric-only line (was 6.5)
	leadChord     = 5.0 // chord-only line (was 6.0)
	leadPair      = 9.6 // chord+lyric pair (was 11.5)
	pairLyricDy   = 4.3 // lyric baseline offset below the chord within a pair
	leadSection   = 6.5 // section label row (was 8.0)
	sectionTopGap = 3.5 // air ABOVE a non-first section, so it reads as a header, not a line
	paraGap       = 2.5 // stanza break (was 4.0); collapsed so consecutive blanks = one gap
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
	pdf, _, err := renderChart(source)
	if err != nil {
		return nil, err
	}
	return output(pdf)
}

// chartLines splits a source into lines (CRLF-normalized).
func chartLines(source string) []string {
	return strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
}

// renderChart draws the chart and returns the pdf + the final body y. The y is the drift guard:
// it must equal measure(source) for a single-page chart (both walk the same rows with the same
// advances). T75: consecutive blank lines collapse to one stanza gap, and there is no gap before
// the first content / a section / at the end.
func renderChart(source string) (*fpdf.Fpdf, float64, error) {
	lines := chartLines(source)
	subtitle, _, bodyPt, skip := parseHeader(lines)
	scale := bodyPt / defaultBodyPt
	pdf, tr := newDoc(firstTitle(lines))
	y := header(pdf, tr, firstTitle(lines), subtitle, scale)
	page := func(need float64) {
		if y+need > pageBottom {
			pdf.AddPage()
			y = topMargin
		}
	}
	drew, pendingGap := false, false
	gap := func() {
		if drew && pendingGap {
			y += paraGap * scale
		}
		pendingGap = false
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case skip[i], strings.HasPrefix(trimmed, "# "):
			// subtitle / directive (header) or the title — never body.
		case trimmed == "":
			pendingGap = true
		case strings.HasPrefix(trimmed, "## "):
			pendingGap = false // the section's own top-gap replaces any stanza gap before it
			if drew {
				y += sectionTopGap * scale // air above a non-first section
			}
			page(leadSection * scale)
			y = sectionLabel(pdf, tr, y, strings.TrimSpace(trimmed[3:]), scale)
			drew = true
		case isChordRow(trimmed) && i+1 < len(lines) && isLyric(lines[i+1]):
			gap()
			page(leadPair * scale)
			ch, an, _ := chordRowParts(line)
			y = chordLine(pdf, tr, y, ch, an, strings.TrimRight(lines[i+1], " \t"), scale)
			drew = true
			i++ // consumed the lyric line
		case isChordRow(trimmed):
			gap()
			page(leadChord * scale)
			ch, an, _ := chordRowParts(line)
			y = chordLine(pdf, tr, y, ch, an, "", scale)
			drew = true
		default:
			gap()
			page(leadLyric * scale)
			y = textLine(pdf, tr, y, line, scale)
			drew = true
		}
	}
	return pdf, y, nil
}

// measure returns the body's end-y at the source's size WITHOUT pagination — i.e. the height it
// occupies on one continuous page, so `measure(source) <= pageBottom` iff the chart fits one page.
// It shares every advance and the header start with renderChart (T76 auto-fit consumes it).
func measure(source string) float64 {
	lines := chartLines(source)
	subtitle, _, bodyPt, skip := parseHeader(lines)
	scale := bodyPt / defaultBodyPt
	y := headerBodyStart(subtitle, scale)
	drew, pendingGap := false, false
	gap := func() {
		if drew && pendingGap {
			y += paraGap * scale
		}
		pendingGap = false
	}
	for i := 0; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		switch {
		case skip[i], strings.HasPrefix(trimmed, "# "):
		case trimmed == "":
			pendingGap = true
		case strings.HasPrefix(trimmed, "## "):
			pendingGap = false
			if drew {
				y += sectionTopGap * scale
			}
			y += leadSection * scale
			drew = true
		case isChordRow(trimmed) && i+1 < len(lines) && isLyric(lines[i+1]):
			gap()
			y += leadPair * scale
			drew = true
			i++
		case isChordRow(trimmed):
			gap()
			y += leadChord * scale
			drew = true
		default:
			gap()
			y += leadLyric * scale
			drew = true
		}
	}
	return y
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

// isChordRow reports whether a line is a chord row (all leading tokens are chords, with an
// optional trailing "(…)" performance note).
func isChordRow(s string) bool {
	_, _, ok := chordRowParts(s)
	return ok
}

// chordRowParts splits a chord row into its chord portion (original spacing preserved, for
// chord-over-word alignment) and an optional terminal "(…)" annotation. ok is true iff the tokens
// BEFORE the first "(" are non-empty and all chords, and — when a "(" is present — the line ends
// with ")". So `Am E7 (x2)` and `Am E7 G (2x, 1x Arpèges)` are chord rows, while `A (very) long
// day` is not (it doesn't end in ")") and `(x2)` alone is not (no chords precede it).
func chordRowParts(s string) (chords, annot string, ok bool) {
	t := strings.TrimRight(s, " \t")
	if strings.TrimSpace(t) == "" {
		return "", "", false
	}
	idx := strings.IndexByte(t, '(')
	if idx < 0 {
		for _, f := range strings.Fields(t) {
			if !chordToken.MatchString(f) {
				return "", "", false
			}
		}
		return t, "", true
	}
	if !strings.HasSuffix(t, ")") {
		return "", "", false
	}
	chords = strings.TrimRight(t[:idx], " \t")
	cf := strings.Fields(chords)
	if len(cf) == 0 {
		return "", "", false
	}
	for _, f := range cf {
		if !chordToken.MatchString(f) {
			return "", "", false
		}
	}
	return chords, strings.TrimSpace(t[idx:]), true
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

// header renders the title (and optional subtitle/artist) plus the rule, and returns the y at
// which the body should start (so callers don't hardcode a body top that ignores the subtitle).
// headerBodyStart is the y where the body begins — the single source shared by header() (its
// return) and measure(). T75: title 22→16, subtitle 12→11, geometry tightened (~8–10 mm reclaimed).
func headerBodyStart(subtitle string, scale float64) float64 {
	ruleY := topMargin + 9*scale
	if subtitle != "" {
		ruleY = topMargin + 14*scale
	}
	return ruleY + 3*scale
}

func header(pdf *fpdf.Fpdf, tr func(string) string, title, subtitle string, scale float64) float64 {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16*scale)
	pdf.SetXY(margin, topMargin)
	pdf.Cell(0, 8*scale, tr(title))
	ruleY := topMargin + 9*scale
	if subtitle != "" {
		pdf.SetFont("Helvetica", "I", 11*scale)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(margin, topMargin+8*scale)
		pdf.Cell(0, 5*scale, tr(subtitle))
		pdf.SetTextColor(0, 0, 0)
		ruleY = topMargin + 14*scale
	}
	pdf.SetLineWidth(0.3)
	pdf.Line(margin, ruleY, right, ruleY)
	return headerBodyStart(subtitle, scale)
}

// subtitleOf returns the chart's subtitle (artist) and its line index, or ("", -1). Rule:
// ADJACENCY — the line at exactly titleIndex+1 (no blank between), when it is not itself a
// section/`#` line or a chord row, and the line after it is blank / a `##` section / EOF. A blank
// line after the title means the body has started, so nothing is lifted out of it — this makes it
// impossible to swallow a body lyric separated from the title by a blank line.
const defaultBodyPt = 11.0

// reSizeDirective matches a `size: N` header-block directive (case-insensitive key). Any other
// `key: value` line is NOT a directive — it stays subtitle/body — so an artist like "Foo: Bar"
// is unaffected. Adding a second key later is a deliberate decision (T74).
var reSizeDirective = regexp.MustCompile(`(?i)^size\s*:\s*(\d+)$`)

// parseHeader scans the header block — the contiguous non-blank lines after `# Title`, before the
// first blank line or `## section` — for the `size` directive and the subtitle. It returns the
// subtitle text + line index (or -1), the body point size (default when the directive is
// absent/out of range), and the set of line indices to skip in the body (directive lines + the
// subtitle). Body lines are never scanned, so a lyric `size: 13` renders as a lyric.
//
// Subtitle rule (superset of T70): the subtitle is the sole non-directive header-block line, when
// it is not a chord row. Zero or ≥2 non-directive lines → no subtitle (a title running straight
// into body is not a header). Both `size`/artist orders work; `size: 99`/`size: abc` don't set a
// size but the numeric one is still consumed.
func parseHeader(lines []string) (subtitle string, subIdx int, bodyPt float64, skip map[int]bool) {
	bodyPt, subIdx, skip = defaultBodyPt, -1, map[int]bool{}
	t := -1
	for i, l := range lines {
		if strings.HasPrefix(strings.TrimSpace(l), "# ") {
			t = i
			break
		}
	}
	if t < 0 {
		return
	}
	var nonDirective []int
	for i := t + 1; i < len(lines); i++ {
		s := strings.TrimSpace(lines[i])
		if s == "" || strings.HasPrefix(s, "##") {
			break // end of the header block
		}
		if m := reSizeDirective.FindStringSubmatch(s); m != nil {
			if n, _ := strconv.Atoi(m[1]); n >= 8 && n <= 16 {
				bodyPt = float64(n)
			}
			skip[i] = true // consumed whether or not the value was in range
			continue
		}
		nonDirective = append(nonDirective, i)
	}
	if len(nonDirective) == 1 {
		i := nonDirective[0]
		if s := strings.TrimSpace(lines[i]); !strings.HasPrefix(s, "#") && !isChordRow(s) {
			subtitle, subIdx = s, i
			skip[i] = true
		}
	}
	return
}

// subtitleOf is retained for the T70 tests: the subtitle text + index only.
func subtitleOf(lines []string) (string, int) {
	s, i, _, _ := parseHeader(lines)
	return s, i
}

// sectionLabel prints a section header (e.g. "Verse 1") and returns the next y.
func sectionLabel(pdf *fpdf.Fpdf, tr func(string) string, y float64, label string, scale float64) float64 {
	pdf.SetFont("Helvetica", "B", 11*scale)
	pdf.SetTextColor(150, 90, 30)
	pdf.SetXY(margin, y)
	pdf.Cell(0, 6*scale, tr(label)) // T73: through tr() like every other string — an accented section name must not mojibake
	pdf.SetTextColor(0, 0, 0)
	return y + leadSection*scale
}

// chordLine prints a monospaced blue chord row and the lyric beneath it.
func chordLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, chords, annot, lyric string, scale float64) float64 {
	pdf.SetFont("Courier", "B", 11*scale)
	pdf.SetTextColor(20, 60, 150)
	pdf.SetXY(margin, y)
	pdf.CellFormat(pdf.GetStringWidth(tr(chords)), 5*scale, tr(chords), "", 0, "L", false, 0, "")
	if annot != "" {
		// a performance note ("(x2)", "(2x, 1x Arpèges)") — an instruction, not something to
		// play, so render it muted + non-chord, on the same line after the chords.
		pdf.SetFont("Helvetica", "I", 10*scale)
		pdf.SetTextColor(120, 120, 120)
		pdf.CellFormat(0, 5*scale, "  "+tr(annot), "", 0, "L", false, 0, "")
	}
	pdf.SetTextColor(0, 0, 0)
	if lyric != "" {
		pdf.SetFont("Courier", "", 11*scale)
		pdf.SetXY(margin, y+pairLyricDy*scale)
		pdf.Cell(0, 5*scale, tr(lyric))
		return y + leadPair*scale
	}
	return y + leadChord*scale
}

// textLine renders a normal paragraph line in Helvetica, honoring inline **bold**.
func textLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, line string, scale float64) float64 {
	pdf.SetXY(margin, y)
	bold := false
	for _, seg := range strings.Split(line, "**") {
		if seg != "" {
			if bold {
				pdf.SetFont("Helvetica", "B", 11*scale)
			} else {
				pdf.SetFont("Helvetica", "", 11*scale)
			}
			pdf.CellFormat(pdf.GetStringWidth(tr(seg)), 6*scale, tr(seg), "", 0, "L", false, 0, "")
		}
		bold = !bold
	}
	return y + leadLyric*scale
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

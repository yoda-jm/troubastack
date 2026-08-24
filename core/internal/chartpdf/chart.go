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
//	{new_page}         force a page break here (alias {np}) — see "Page breaks" below
//	{footnote}         open a footnote/attribution block (alias {fn}) — see "Footnotes" below
//	(blank line)       paragraph gap
//	anything else      literal text
//
// Page breaks (T77): a line containing only `{new_page}` — or the short alias `{np}`,
// case-insensitive, surrounding whitespace ignored — forces the following content to the top of the
// next page. Put it where a player has a free hand (a section end, a rest, an instrumental), not
// merely where the page happens to fill. It is never printed, and it never makes a blank page: a
// marker before any content, after all content, or repeated collapses to nothing. Automatic breaks
// likewise never strand a section header — a heading always travels to the same page as its first
// line.
//
// Footnotes (T95): a line containing only `{footnote}` — or the alias `{fn}`, case-insensitive,
// surrounding whitespace ignored (same discipline as `{new_page}`; `{fn} x` and `{{fn}}` stay literal
// body text) — opens a footnote/attribution block for public-domain credits, "capo" / chord-shape
// notes, and the like. The following lines up to the NEXT blank line (or a `#`/`##`/other marker/EOF)
// are one paragraph, wrapped to the body column and rendered in a smaller muted italic, visually
// distinct from lyrics. It is drawn IN PLACE (write it last for "after the body") and is just another
// block: it counts toward T76 auto-fit and paginates under T77 like any content. `**bold**` is NOT
// interpreted inside a footnote — the asterisks are literal.
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
	"sort"
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

	// T95 footnote block ({footnote}/{fn}): wrapped prose, smaller + muted italic, visually distinct
	// from lyrics. leadFootnote is one wrapped line's advance; footnoteTopGap is the air above the block.
	footnotePt     = 9.0 // point size at scale 1 (body is 11)
	leadFootnote   = 4.2 // one wrapped footnote line's advance
	footnoteTopGap = 3.5 // air above the footnote block
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

// reNewPage matches a page-break flow marker: exactly `{new_page}` or the alias `{np}`,
// case-insensitive, surrounding whitespace ignored. Braces (not a bare `key:`) mark it as a
// positional flow directive — distinct from the header-scoped `size:` metadata — and match ChordPro.
var reNewPage = regexp.MustCompile(`(?i)^\{(new_page|np)\}$`)

// isNewPageMarker reports whether a whole line is a page-break marker. `{newpage}`, `{new page}`,
// `{np} x`, `new_page` and `{{np}}` are NOT markers — they are content and render as body text.
func isNewPageMarker(trimmed string) bool { return reNewPage.MatchString(trimmed) }

// reFootnote matches a footnote-block opener: exactly `{footnote}` or the alias `{fn}`,
// case-insensitive, surrounding whitespace ignored — same brace-directive discipline as reNewPage
// (T95, Fable's ruling). `{fn} x` and `{{fn}}` are NOT markers; they stay literal body text.
var reFootnote = regexp.MustCompile(`(?i)^\{(footnote|fn)\}$`)

// isFootnoteMarker reports whether a whole line opens a footnote block.
func isFootnoteMarker(trimmed string) bool { return reFootnote.MatchString(trimmed) }

// Render parses the chart dialect in source and returns a deterministic A4 PDF.
func Render(source string) ([]byte, error) {
	if err := validateChars(source); err != nil {
		return nil, err
	}
	pdf, _, _, err := renderChart(source, false)
	if err != nil {
		return nil, err
	}
	return output(pdf)
}

// Anchor is one drawn text run's bounding box in [0,1]² page coordinates, plus the run's ORIGINAL
// (pre-cp1252) text and 0-indexed page. It mirrors mkcharts' `<name>.anchors.json` shape (B13)
// EXACTLY, so the demo seed's page+substring+occurrence lookup places a highlight over a chartpdf
// chart's word identically. Anchors come from the SAME layout walk that draws the ink
// (RenderWithAnchors), so a box cannot disagree with what is on the page (T95 §3/§5.1).
type Anchor struct {
	Page int     `json:"page"` // 0-indexed
	Text string  `json:"text"`
	X0   float64 `json:"x0"`
	Y0   float64 `json:"y0"`
	X1   float64 `json:"x1"`
	Y1   float64 `json:"y1"`
}

// recFn records one drawn text run: (x,y) its top-left in mm, (w,h) its size in mm. nil in the
// measure/contentHeight passes (no drawing, so no anchors) and in a plain Render (no manifest asked).
type recFn func(text string, x, y, w, h float64)

// RenderWithAnchors renders the chart to a deterministic A4 PDF AND returns the anchor manifest for
// every text run it drew, in mkcharts' shape and order (page, then top-to-bottom, then left-to-right).
// `Render` is exactly this minus the manifest — same bytes.
func RenderWithAnchors(source string) ([]byte, []Anchor, error) {
	if err := validateChars(source); err != nil {
		return nil, nil, err
	}
	pdf, anchors, _, err := renderChart(source, true)
	if err != nil {
		return nil, nil, err
	}
	out, err := output(pdf)
	if err != nil {
		return nil, nil, err
	}
	sortAnchors(anchors)
	return out, anchors, nil
}

// sortAnchors orders the manifest deterministically: page, then top-to-bottom, then left-to-right —
// identical to mkcharts' writeAnchors, so the two backends' manifests are directly comparable.
func sortAnchors(a []Anchor) {
	sort.SliceStable(a, func(i, j int) bool {
		if a[i].Page != a[j].Page {
			return a[i].Page < a[j].Page
		}
		if a[i].Y0 != a[j].Y0 {
			return a[i].Y0 < a[j].Y0
		}
		return a[i].X0 < a[j].X0
	})
}

// chartLines splits a source into lines (CRLF-normalized).
func chartLines(source string) []string {
	return strings.Split(strings.ReplaceAll(source, "\r\n", "\n"), "\n")
}

// renderChart draws the chart and returns the pdf + the final body y. That y is the drift guard: it
// must equal measure(source) — both walk the identical body via layout(), the single source of the
// per-row advances AND of pagination, so they cannot drift, on one page or many.
func renderChart(source string, collect bool) (*fpdf.Fpdf, []Anchor, float64, error) {
	lines := chartLines(source)
	subtitle, _, bodyPt, sizeSet, skip := parseHeader(lines)
	if !sizeSet {
		bodyPt = autoFitBodyPt(lines, subtitle, skip) // T76: largest size that keeps every segment on its page
	}
	scale := bodyPt / defaultBodyPt
	pdf, tr := newDoc(firstTitle(lines))
	var anchors []Anchor
	var rec recFn
	if collect {
		// The anchor box is in [0,1]² of the A4 page; the run's page is fpdf's current page (0-idx),
		// so a run recorded straight after its Cell/CellFormat lands on the page it drew onto.
		rec = func(text string, x, y, w, h float64) {
			anchors = append(anchors, Anchor{
				Page: pdf.PageNo() - 1, Text: text,
				X0: x / pageW, Y0: y / pageH, X1: (x + w) / pageW, Y1: (y + h) / pageH,
			})
		}
	}
	y := header(pdf, tr, firstTitle(lines), subtitle, scale, rec)
	y = layout(lines, scale, skip, y, layoutOpts{pdf: pdf, tr: tr, paginate: true, rec: rec})
	return pdf, anchors, y, nil
}

// measure returns the renderer's final body y for the source at its size — the PAGINATED end, so it
// equals renderChart's returned y across page breaks (the T77 drift guard). Because pagination resets
// to the top margin, do not read measure() as a height; for "how tall / does it fit" use
// contentHeight().
func measure(source string) float64 {
	lines := chartLines(source)
	subtitle, _, bodyPt, _, skip := parseHeader(lines)
	scale := bodyPt / defaultBodyPt
	return layout(lines, scale, skip, headerBodyStart(subtitle, scale), layoutOpts{paginate: true})
}

// contentHeight is the continuous body height the chart occupies on one unbounded page (pagination
// disabled): contentHeight(source) <= pageBottom iff the whole chart fits a single page. T75's
// compaction guard and T76's auto-fit fit-test consume this — the honest "how tall is it", distinct
// from measure()'s paginated end-y.
func contentHeight(source string) float64 {
	lines := chartLines(source)
	subtitle, _, bodyPt, _, skip := parseHeader(lines)
	scale := bodyPt / defaultBodyPt
	return layout(lines, scale, skip, headerBodyStart(subtitle, scale), layoutOpts{paginate: false})
}

// autoFitBodyPt returns the largest integer point size in [minBodyPt, maxBodyPt] at which the chart
// needs no automatic page break — i.e. every {new_page}-delimited segment fits its own page (T76).
// It searches ceiling-down and returns the first that fits, so the result is deterministic (same
// input → same size → same bytes). If even the floor overflows, it returns the floor: overflow to a
// second page is allowed rather than printing type too small to read on a stand ("most songs, not
// all"). Used only when no manual `size:` was given — a manual size disables auto-fit (parseHeader
// sizeSet).
func autoFitBodyPt(lines []string, subtitle string, skip map[int]bool) float64 {
	for pt := maxBodyPt; pt > minBodyPt; pt-- {
		if fitsAt(lines, subtitle, skip, float64(pt)) {
			return float64(pt)
		}
	}
	return minBodyPt
}

// fitsAt reports whether the chart body fits at bodyPt with NO automatic page break. It runs the same
// paginated layout the renderer uses (starting the first segment below the header, later segments at
// the top margin) and counts automatic breaks: zero means every segment fit its own page. Using the
// real layout — not a raw height compare — means orphan control and the never-split-a-pair rule are
// honoured exactly as they will be drawn, so a size fits here iff it renders on its segments' pages.
func fitsAt(lines []string, subtitle string, skip map[int]bool, bodyPt float64) bool {
	scale := bodyPt / defaultBodyPt
	auto := 0
	layout(lines, scale, skip, headerBodyStart(subtitle, scale), layoutOpts{paginate: true, autoBreaks: &auto})
	return auto == 0
}

// placed records one drawn element's page and top y — a test hook (layoutOpts.trace) for asserting
// pagination: orphan control (a section header is never a page's last element) and the pair rule (a
// chord+lyric pair never straddles a page).
type placed struct {
	page int
	y    float64
	kind string // "section" | "pair" | "chord" | "text"
}

// layoutOpts selects layout()'s mode: draw into pdf (with tr) or not; paginate (reset to the top
// margin at each break) or accumulate continuously; and optionally record a trace.
type layoutOpts struct {
	pdf      *fpdf.Fpdf
	tr       func(string) string
	paginate bool
	rec      recFn // T95: when set (draw+anchor pass), each primitive records its text runs' boxes
	trace    *[]placed
	// autoBreaks, when non-nil, counts AUTOMATIC page breaks (content crossing pageBottom) — NOT
	// explicit {new_page} breaks. T76 auto-fit's fit-test: a size fits iff it forces zero automatic
	// breaks, i.e. every {new_page}-delimited segment fits its own page (segment 1 under the header).
	autoBreaks *int
}

// layout walks the body once and returns the final y. It is the SINGLE source of the per-row advances
// AND of pagination, shared by renderChart (draws), measure (paginated end-y) and contentHeight
// (continuous). Three break triggers are honored identically:
//   - an explicit {new_page} marker, applied lazily so a leading / trailing / repeated marker never
//     makes a blank page;
//   - an automatic break when the next unit would cross pageBottom;
//   - section-orphan control: a `## header` reserves room for itself AND its first content line, so a
//     heading is never stranded at the foot of a page (automatic breaks only — an explicit marker
//     right after a header is obeyed as written).
func layout(lines []string, scale float64, skip map[int]bool, y float64, o layoutOpts) float64 {
	pg := 1
	newPage := func() {
		if o.paginate {
			if o.pdf != nil {
				o.pdf.AddPage()
			}
			y = topMargin
			pg++
		}
	}
	page := func(need float64) {
		if o.paginate && y+need > pageBottom {
			if o.autoBreaks != nil {
				*o.autoBreaks++ // an automatic break: this size did not fit the segment (T76)
			}
			newPage()
		}
	}
	drew, pendingGap, pendingBreak := false, false, false
	gap := func() {
		if drew && pendingGap {
			y += paraGap * scale
		}
		pendingGap = false
	}
	// applyBreak turns a pending {new_page} into an actual break, lazily — a trailing marker, a
	// leading marker (drew==false, so never set) and consecutive markers all collapse to no blank
	// page. Returns whether it broke (the caller then drops the pre-content gap: after a break the
	// body starts flush at the top margin, per T75).
	applyBreak := func() bool {
		if !pendingBreak {
			return false
		}
		pendingBreak, pendingGap = false, false
		newPage()
		return true
	}
	note := func(kind string) {
		if o.trace != nil {
			*o.trace = append(*o.trace, placed{page: pg, y: y, kind: kind})
		}
	}
	// getMeasurer returns an fpdf to size footnote wrapping with. In the draw pass the real doc IS the
	// measurer (identical metrics); in a measure/fit pass (o.pdf == nil) a throwaway is created lazily —
	// only if a footnote is actually reached, so a footnote-free chart pays nothing. Core-font metrics
	// are instance-independent, so the wrap is identical to what the draw pass produces (the drift guard).
	var measurer *fpdf.Fpdf
	var measureTr func(string) string
	getMeasurer := func() (*fpdf.Fpdf, func(string) string) {
		if o.pdf != nil {
			return o.pdf, o.tr // draw pass: measure with the real doc + its translator
		}
		if measurer == nil {
			measurer = newMeasurer()
			measureTr = measurer.UnicodeTranslatorFromDescriptor("") // same cp1252 mapping ⇒ same widths
		}
		return measurer, measureTr
	}
	for i := 0; i < len(lines); i++ {
		line := strings.TrimRight(lines[i], " \t")
		trimmed := strings.TrimSpace(line)
		switch {
		case skip[i], strings.HasPrefix(trimmed, "# "):
			// subtitle / directive (header) or the title — never body.
		case trimmed == "":
			pendingGap = true
		case isNewPageMarker(trimmed):
			// consumed, never drawn; break BEFORE the next content (lazily, via applyBreak).
			if drew {
				pendingBreak, pendingGap = true, false
			}
		case isFootnoteMarker(trimmed):
			// {footnote}/{fn}: the following non-blank lines up to a blank / #/## / another marker / EOF
			// are ONE paragraph (a blank line ends it), rendered as wrapped muted-italic prose IN PLACE.
			var para []string
			j := i + 1
			for ; j < len(lines); j++ {
				t := strings.TrimSpace(lines[j])
				if t == "" || strings.HasPrefix(t, "#") || isNewPageMarker(t) || isFootnoteMarker(t) {
					break
				}
				para = append(para, t)
			}
			i = j - 1 // consume the paragraph (the loop's i++ steps past the last gathered line)
			if len(para) == 0 {
				break // a bodyless {footnote}: consumed, draws nothing (like a trailing {new_page})
			}
			applyBreak()
			gap()
			if drew {
				y += footnoteTopGap * scale // air above the block (unless it opens a fresh page)
			}
			// **bold** is LITERAL inside a footnote (documented): the asterisks render as text, so a
			// wrapped attribution can't accidentally trip Part 1's per-bold-word anchoring.
			mpdf, mtr := getMeasurer()
			for _, wl := range footnoteLines(mpdf, mtr, strings.Join(para, " "), scale) {
				page(leadFootnote * scale)
				note("footnote")
				y = drawFootnoteLine(o.pdf, o.tr, y, wl, scale, o.rec)
			}
			drew = true
		case strings.HasPrefix(trimmed, "## "):
			broke := applyBreak()
			pendingGap = false
			if drew && !broke {
				y += sectionTopGap * scale // air above a non-first section (not at a fresh page top)
			}
			// orphan control: keep the header with its first content line on one page.
			page(leadSection*scale + firstUnitLead(lines, i, skip)*scale)
			note("section")
			y = sectionLabel(o.pdf, o.tr, y, strings.TrimSpace(trimmed[3:]), scale, o.rec)
			drew = true
		case isChordRow(trimmed) && i+1 < len(lines) && isLyric(lines[i+1]):
			applyBreak()
			gap()
			page(leadPair * scale) // the pair is one unit — never split a chord from its lyric
			note("pair")
			ch, an, _ := chordRowParts(line)
			y = chordLine(o.pdf, o.tr, y, ch, an, strings.TrimRight(lines[i+1], " \t"), scale, o.rec)
			drew = true
			i++ // consumed the lyric line
		case isChordRow(trimmed):
			applyBreak()
			gap()
			page(leadChord * scale)
			note("chord")
			ch, an, _ := chordRowParts(line)
			y = chordLine(o.pdf, o.tr, y, ch, an, "", scale, o.rec)
			drew = true
		default:
			applyBreak()
			gap()
			page(leadLyric * scale)
			note("text")
			y = textLine(o.pdf, o.tr, y, line, scale, o.rec)
			drew = true
		}
	}
	return y
}

// firstUnitLead returns the advance of a section's first content line — the unit orphan control keeps
// with the header. 0 when the section has no content to be stranded from on this page: it is empty,
// runs straight into another section, hits EOF, or the author placed an explicit {new_page} right
// after it (an explicit break is obeyed as written, not orphan-protected).
func firstUnitLead(lines []string, headerIdx int, skip map[int]bool) float64 {
	for j := headerIdx + 1; j < len(lines); j++ {
		if skip[j] {
			continue
		}
		t := strings.TrimSpace(lines[j])
		switch {
		case t == "":
			continue
		case strings.HasPrefix(t, "# "), strings.HasPrefix(t, "## "), isNewPageMarker(t):
			return 0
		case isChordRow(t) && j+1 < len(lines) && isLyric(lines[j+1]):
			return leadPair
		case isChordRow(t):
			return leadChord
		default:
			return leadLyric
		}
	}
	return 0
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

func header(pdf *fpdf.Fpdf, tr func(string) string, title, subtitle string, scale float64, rec recFn) float64 {
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16*scale)
	pdf.SetXY(margin, topMargin)
	pdf.Cell(0, 8*scale, tr(title))
	if rec != nil {
		rec(title, margin, topMargin, pdf.GetStringWidth(tr(title)), 8*scale)
	}
	ruleY := topMargin + 9*scale
	if subtitle != "" {
		pdf.SetFont("Helvetica", "I", 11*scale)
		pdf.SetTextColor(90, 90, 90)
		pdf.SetXY(margin, topMargin+8*scale)
		pdf.Cell(0, 5*scale, tr(subtitle))
		if rec != nil {
			rec(subtitle, margin, topMargin+8*scale, pdf.GetStringWidth(tr(subtitle)), 5*scale)
		}
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

// T74/T76 body-size range: the manual `size:` bounds AND auto-fit's search range — one bounded range
// for the dialect, so a manual `size:` and an auto-fit size can never disagree about what is legal.
const (
	minBodyPt = 8  // readability floor; below it we accept overflow rather than print unreadable type
	maxBodyPt = 16 // ceiling; auto-fit never exceeds the manual maximum
)

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
func parseHeader(lines []string) (subtitle string, subIdx int, bodyPt float64, sizeSet bool, skip map[int]bool) {
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
			// The author opted into a manual size — auto-fit (T76) defers to that intent whether or
			// not the value is in range, so a `size:` chart renders byte-identically to its T74/T75
			// output rather than being silently re-sized.
			sizeSet = true
			if n, _ := strconv.Atoi(m[1]); n >= minBodyPt && n <= maxBodyPt {
				bodyPt = float64(n)
			}
			skip[i] = true // consumed whether or not the value was in range
			continue
		}
		if isNewPageMarker(s) {
			skip[i] = true // a leading {new_page} in the header block: consumed, never a subtitle
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
	s, i, _, _, _ := parseHeader(lines)
	return s, i
}

// sectionLabel prints a section header (e.g. "Verse 1") and returns the next y.
func sectionLabel(pdf *fpdf.Fpdf, tr func(string) string, y float64, label string, scale float64, rec recFn) float64 {
	if pdf != nil { // nil in measure/contentHeight mode: advance only, draw nothing
		pdf.SetFont("Helvetica", "B", 11*scale)
		pdf.SetTextColor(150, 90, 30)
		pdf.SetXY(margin, y)
		pdf.Cell(0, 6*scale, tr(label)) // T73: through tr() like every other string — an accented section name must not mojibake
		if rec != nil {
			rec(label, margin, y, pdf.GetStringWidth(tr(label)), 6*scale)
		}
		pdf.SetTextColor(0, 0, 0)
	}
	return y + leadSection*scale
}

// chordLine prints a monospaced blue chord row and the lyric beneath it.
func chordLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, chords, annot, lyric string, scale float64, rec recFn) float64 {
	if pdf != nil { // nil in measure/contentHeight mode: advance only, draw nothing
		pdf.SetFont("Courier", "B", 11*scale)
		pdf.SetTextColor(20, 60, 150)
		pdf.SetXY(margin, y)
		pdf.CellFormat(pdf.GetStringWidth(tr(chords)), 5*scale, tr(chords), "", 0, "L", false, 0, "")
		if rec != nil {
			rec(chords, margin, y, pdf.GetStringWidth(tr(chords)), 5*scale)
		}
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
			if rec != nil {
				rec(lyric, margin, y+pairLyricDy*scale, pdf.GetStringWidth(tr(lyric)), 5*scale)
			}
		}
	}
	if lyric != "" {
		return y + leadPair*scale
	}
	return y + leadChord*scale
}

// textLine renders a normal paragraph line in Helvetica, honoring inline **bold**.
func textLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, line string, scale float64, rec recFn) float64 {
	if pdf != nil { // nil in measure/contentHeight mode: advance only, draw nothing
		pdf.SetXY(margin, y)
		bold := false
		x := margin // track the cursor so each **bold** segment gets its own box at the right x
		for _, seg := range strings.Split(line, "**") {
			if seg != "" {
				if bold {
					pdf.SetFont("Helvetica", "B", 11*scale)
				} else {
					pdf.SetFont("Helvetica", "", 11*scale)
				}
				w := pdf.GetStringWidth(tr(seg))
				pdf.CellFormat(w, 6*scale, tr(seg), "", 0, "L", false, 0, "")
				if rec != nil {
					rec(seg, x, y, w, 6*scale)
				}
				x += w
			}
			bold = !bold
		}
	}
	return y + leadLyric*scale
}

// newMeasurer returns a throwaway A4 fpdf used only to size footnote wrapping in the draw-free
// measure/fit passes. Core-font metrics are instance-independent, so its GetStringWidth matches the
// real document's — the wrap it computes is the wrap that will be drawn.
func newMeasurer() *fpdf.Fpdf {
	m := fpdf.New("P", "mm", "A4", "")
	m.AddPage()
	return m
}

// footnoteLines wraps a footnote paragraph to the body column at the footnote font, returning the
// ORIGINAL-text lines (widths measured through tr(), so cp1252 metrics are exact and the lines match
// what drawFootnoteLine renders). A single word wider than the column overflows on its own line rather
// than looping. `m` is the measurer (the real doc when drawing, a throwaway otherwise).
func footnoteLines(m *fpdf.Fpdf, tr func(string) string, text string, scale float64) []string {
	m.SetFont("Helvetica", "I", footnotePt*scale)
	colW := right - margin
	var lines []string
	cur := ""
	for _, w := range strings.Fields(text) {
		try := w
		if cur != "" {
			try = cur + " " + w
		}
		if cur == "" || m.GetStringWidth(tr(try)) <= colW {
			cur = try
		} else {
			lines = append(lines, cur)
			cur = w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return lines
}

// drawFootnoteLine draws one already-wrapped footnote line in muted italic and records its anchor;
// nil pdf = advance only (measure mode). Every wrapped line advances by leadFootnote in BOTH modes, so
// measure() and the renderer agree on the block's height (the T77 drift guard). **bold** is not parsed
// here — inside a footnote the asterisks are literal (documented in the dialect header).
func drawFootnoteLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, line string, scale float64, rec recFn) float64 {
	if pdf != nil {
		pdf.SetFont("Helvetica", "I", footnotePt*scale)
		pdf.SetTextColor(100, 100, 100)
		pdf.SetXY(margin, y)
		pdf.Cell(0, leadFootnote*scale, tr(line))
		if rec != nil {
			rec(line, margin, y, pdf.GetStringWidth(tr(line)), leadFootnote*scale)
		}
		pdf.SetTextColor(0, 0, 0)
	}
	return y + leadFootnote*scale
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

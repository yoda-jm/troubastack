package chartpdf

// Tab blocks in the chart dialect (T135). A tab chart is a normal chart whose source contains a
// fenced block; the whole downstream stack (chart-source, Generated, bake, Stage, folders, .tband)
// is unchanged because the output is still one generated PDF. Only the SYNTAX is new.
//
//	{start_of_tab}     open a block (alias {sot}); whole line, case-insensitive (T77 discipline)
//	…                  every line inside is content, drawn VERBATIM in monospace
//	(blank line)       separates two staves; a stave is one layout unit, never split across a page
//	{end_of_tab}       close a block (alias {eot}); an unclosed block runs to EOF
//
// Inside a block nothing but the closer is a marker: `#`, `##`, `{np}`, `{fn}`, `**bold**` are all
// literal. A line that is a chord row (chord names over the strings) is drawn bold in the chord colour
// at the SAME monospace size so its columns stay over the frets. Tab lines are NEVER transposed
// (transpose.go skips the whole block) and NEVER wrapped or clipped: the tab size is the smaller of the
// proportional size and the size at which the longest stave line fits the column; below the 7 pt floor
// the save is refused (ErrTabTooWide). One size for all blocks in the chart.

import (
	"fmt"
	"math"
	"regexp"
	"strings"

	"github.com/go-pdf/fpdf"
)

// ErrTabTooWide is returned by Render when a tab line is longer than fits the body column even at the
// readability floor. Silent clipping of frets is the one failure a stage cannot tolerate, so the save
// is refused with the offending line named.
var ErrTabTooWide = fmt.Errorf("chartpdf: tab line too wide")

const (
	tabBasePt  = 9.0 // tab point size under the default 11 pt body (matches the demo tab PDFs)
	tabFloorPt = 7.0 // readability floor; a line that needs less than this to fit is refused
	leadTab    = 4.0 // per string-line advance at tabBasePt (1.25× the type — tight, reads as one grid)
	tabGap     = 2.5 // air between two staves in a block
	tabTopGap  = 2.0 // air above a block that follows other content
)

// reTabStart / reTabEnd match the block markers with the same whole-line, case-insensitive, ChordPro
// discipline as {new_page}/{footnote}: `{sot} x`, `{{sot}}`, `sot`, `{tab}` are NOT markers.
//
// An opener may carry attributes, but ONLY in key=value form (Fable's ruling): `{sot original=G}` names
// the key the frets are written in, drawn as a zero-height marker when the chart is baked transposed so
// the reader is not misled (tab blocks are never transposed). A bare trailing token — `{sot} x` — is NOT
// an opener and stays text; that boundary is pinned by the near-miss tests. `original=` is author-
// writable (it is their claim about their document) and is also injected by the transpose step.
var reTabStart = regexp.MustCompile(`(?i)^\{(start_of_tab|sot)(\s+original=([^\s}]+))?\}$`)
var reTabEnd = regexp.MustCompile(`(?i)^\{(end_of_tab|eot)\}$`)

func isTabStart(trimmed string) bool { return reTabStart.MatchString(trimmed) }
func isTabEnd(trimmed string) bool   { return reTabEnd.MatchString(trimmed) }

// tabOpenerOriginalKey returns the `original=` value on an opener line, or "" if absent / not an opener.
func tabOpenerOriginalKey(trimmed string) string {
	m := reTabStart.FindStringSubmatch(trimmed)
	if m == nil {
		return ""
	}
	return m[3]
}

// drawTabMarker draws the "tab in original key (G)" note in the page's bottom margin — a zero-height,
// non-anchored footer, so it CANNOT move any annotation below it (bake renders transposed without
// re-anchoring, so geometry must be byte-identical: chart.go Part A invariant). The margin always
// clears the stave, even at the width floor where the stave fills the 186 mm column, and sits below
// pageBottom so it cannot collide with an in-body {footnote}. Never recorded as an anchor.
func drawTabMarker(pdf *fpdf.Fpdf, tr func(string) string, originalKey string) {
	if pdf == nil || originalKey == "" {
		return
	}
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(120, 120, 120)
	pdf.SetXY(leftMargin, pageBottom+3)
	pdf.CellFormat(tabColW, 4, tr("tab in original key ("+originalKey+")"), "", 0, "R", false, 0, "")
	pdf.SetTextColor(0, 0, 0)
}

// tabContentSet returns, for each source line, whether it sits INSIDE a tab block (between an opener
// and its closer, or EOF for an unclosed block) — the opener and closer lines themselves are false.
// It is the single predicate the renderer, the width check and transpose.go share, so what is drawn
// verbatim, what is width-checked and what is left un-transposed can never disagree.
func tabContentSet(lines []string) []bool {
	in := make([]bool, len(lines))
	block := false
	for i, l := range lines {
		t := strings.TrimSpace(l)
		switch {
		case !block && isTabStart(t):
			block = true
		case block && isTabEnd(t):
			block = false
		case block:
			in[i] = true
		}
	}
	return in
}

// hasTabBlock reports whether the source opens any tab block.
func hasTabBlock(lines []string) bool {
	for _, l := range lines {
		if isTabStart(strings.TrimSpace(l)) {
			return true
		}
	}
	return false
}

// longestTabLine returns the widest content line across all blocks (by rune count — Courier is
// monospace, so rune count is an exact proxy for width) and whether any block exists.
func longestTabLine(lines []string) (string, bool) {
	in := tabContentSet(lines)
	longest, found := "", false
	for i := range lines {
		if !in[i] {
			continue
		}
		found = true
		body := strings.TrimRight(lines[i], " \t")
		if len([]rune(body)) > len([]rune(longest)) {
			longest = body
		}
	}
	return longest, found
}

// tabColW is the body column width the tab must fit (the same width the rest of the body uses — T146
// widened it by moving the left edge to leftMargin).
const tabColW = right - leftMargin

// widthFitPt returns the largest point size at which s fits the body column in Courier, measured
// through tr so cp1252 metrics are exact. +Inf for an empty string (no constraint). Scale-independent:
// it is a property of the line and the column, not the body size.
func widthFitPt(m *fpdf.Fpdf, tr func(string) string, s string) float64 {
	if s == "" {
		return math.Inf(1)
	}
	m.SetFont("Courier", "", 10)
	w := m.GetStringWidth(tr(s))
	if w <= 0 {
		return math.Inf(1)
	}
	return 10.0 * tabColW / w // width scales linearly with point size
}

// validateTabWidth refuses a source whose longest tab line cannot fit the column even at the 7 pt
// floor — naming the line and the limit (the ErrUnsupportedChar pattern: a render error refuses the
// save). No block, or every line fits: nil.
func validateTabWidth(lines []string) error {
	longest, has := longestTabLine(lines)
	if !has {
		return nil
	}
	m := newMeasurer()
	tr := m.UnicodeTranslatorFromDescriptor("")
	if fit := widthFitPt(m, tr, longest); fit < tabFloorPt {
		return fmt.Errorf("%w: %q (%d characters) needs %.1f pt to fit the %.0f mm column but the floor is %.0f pt — shorten the line",
			ErrTabTooWide, longest, len([]rune(longest)), fit, tabColW, tabFloorPt)
	}
	return nil
}

// tabPtFor returns the tab point size for the whole chart at this body scale: the smaller of the
// proportional size (tabBasePt × scale) and the width-fit size for the longest line. One size for
// every block (the longest line governs). Callers width-validate first, so this never returns a size
// that clips. 0 when there is no block.
func tabPtFor(m *fpdf.Fpdf, tr func(string) string, lines []string, scale float64) float64 {
	longest, has := longestTabLine(lines)
	if !has {
		return 0
	}
	pt := tabBasePt * scale
	if fit := widthFitPt(m, tr, longest); fit < pt {
		pt = fit
	}
	return pt
}

// gatherStaves splits a block's content lines (between opener and closer) into staves — runs of
// non-blank lines; a blank line separates staves and consecutive blanks collapse. Returns the staves
// and the index of the closer (or len(lines) for an unclosed block, so the caller resumes past EOF).
func gatherStaves(lines []string, openerIdx int) (staves [][]string, endIdx int) {
	var cur []string
	i := openerIdx + 1
	for ; i < len(lines); i++ {
		t := strings.TrimSpace(lines[i])
		if isTabEnd(t) {
			break
		}
		if t == "" {
			if len(cur) > 0 {
				staves = append(staves, cur)
				cur = nil
			}
			continue
		}
		cur = append(cur, strings.TrimRight(lines[i], " \t"))
	}
	if len(cur) > 0 {
		staves = append(staves, cur)
	}
	return staves, i // i is the closer index, or len(lines) if unclosed
}

// staveHeight is a stave's drawn height at tabPt: one leadTab advance (scaled to the tab size) per
// line. The ratio to tabBasePt scales leadTab/tabGap/tabTopGap "with the tab size actually used".
func staveHeight(nLines int, tabPt float64) float64 {
	return float64(nLines) * leadTab * (tabPt / tabBasePt)
}

// drawTabLine draws (or, with a nil pdf, just advances past) one stave line in Courier at tabPt: a
// chord row over the strings is bold in the chord colour, a string line is plain black — both at the
// same monospace size so the columns line up. Records the run's anchor like every other primitive.
func drawTabLine(pdf *fpdf.Fpdf, tr func(string) string, y float64, text string, tabPt float64, rec recFn) float64 {
	lead := leadTab * (tabPt / tabBasePt)
	if pdf != nil {
		if isChordRow(text) {
			pdf.SetFont("Courier", "B", tabPt)
			pdf.SetTextColor(20, 60, 150)
		} else {
			pdf.SetFont("Courier", "", tabPt)
			pdf.SetTextColor(0, 0, 0)
		}
		pdf.SetXY(leftMargin, y)
		pdf.Cell(0, lead, tr(text))
		if rec != nil {
			rec(text, leftMargin, y, pdf.GetStringWidth(tr(text)), lead)
		}
		pdf.SetTextColor(0, 0, 0)
	}
	return y + lead
}

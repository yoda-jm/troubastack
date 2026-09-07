package chartpdf

import (
	"strings"

	"github.com/go-pdf/fpdf"
)

// T168 — wrapping a body line to its column.
//
// A chart line was never wrapped: in ONE column that was safe, because the author wrote the line to fit the
// page. Two columns halve the width and the same line simply kept going — over the gutter, through the
// right column and off the paper (measured: 33 runs out of their column, 17 off the sheet, worst right edge
// 2.04 × the page). VLL's ⟨D1⟩: wrap it, with the overflowing word going to the next line.
//
// The wrap lives here and is called from layout(), which is also what fitsAt() runs — so auto-fit and
// pagination measure the chart AFTER wrapping. Measuring before would trade a sideways overflow for a
// downward one and look fixed.

// monoColChars is how many Courier characters fit a column of width colW at this scale. BOTH rows of a
// chord/lyric pair are Courier at 11*scale (chords bold, lyric regular — same metrics), so this is the one
// character grid the pair shares, and splitting both rows at the same index keeps every chord over its word.
func monoColChars(m *fpdf.Fpdf, scale, colW float64) int {
	m.SetFont("Courier", "", 11*scale)
	w := m.GetStringWidth("0")
	if w <= 0 {
		return 1 << 20 // degenerate metrics: never wrap rather than wrap to nothing
	}
	n := int(colW / w)
	if n < 1 {
		n = 1
	}
	return n
}

// pairSeg is one drawn line of a wrapped pair: a chord row and the lyric beneath it.
type pairSeg struct{ ch, ly string }

// runeAt reports the rune at i, or 0 past the end — so a short row never blocks a cut point.
func runeAt(r []rune, i int) rune {
	if i < 0 || i >= len(r) {
		return 0
	}
	return r[i]
}

// cutPoint picks where to split a pair at or before `limit`. It prefers an index that splits NEITHER a word
// nor a chord (a blank in both rows, or past the end of one), then a word boundary in the lyric alone, and
// only then cuts hard — a line with no blank in it has nowhere better to go.
func cutPoint(cr, lr []rune, limit int) int {
	if limit < 1 {
		return 1
	}
	for i := limit; i > 0; i-- {
		cOK := runeAt(cr, i) == ' ' || i >= len(cr)
		lOK := runeAt(lr, i) == ' ' || i >= len(lr)
		if cOK && lOK {
			return i
		}
	}
	for i := limit; i > 0; i-- {
		if runeAt(lr, i) == ' ' || i >= len(lr) {
			return i
		}
	}
	return limit
}

// leadingBlanks counts the spaces a continuation starts with, so the SAME number can be dropped from both
// rows — dropping them independently would slide the chords off their words.
func leadingBlanks(r []rune) int {
	n := 0
	for n < len(r) && r[n] == ' ' {
		n++
	}
	return n
}

// wrapPair splits a chord row and its lyric into continuation pairs, each fitting maxChars of the shared
// monospace grid. An empty lyric (a chord-only row) wraps just as well.
func wrapPair(chords, lyric string, maxChars int) []pairSeg {
	cr, lr := []rune(chords), []rune(lyric)
	var out []pairSeg
	for {
		if len(cr) <= maxChars && len(lr) <= maxChars {
			// Nothing to wrap: hand back the rows EXACTLY as given. Trimming or normalising here would
			// change the bytes of every chart that already fitted — and single-column output being
			// byte-identical is the property T146 stage 2 rests on.
			return append(out, pairSeg{string(cr), string(lr)})
		}
		cut := cutPoint(cr, lr, maxChars)
		head := pairSeg{
			ch: strings.TrimRight(string(cr[:min(cut, len(cr))]), " "),
			ly: strings.TrimRight(string(lr[:min(cut, len(lr))]), " "),
		}
		out = append(out, head)
		cr, lr = cr[min(cut, len(cr)):], lr[min(cut, len(lr)):]
		// Drop the same leading run from both rows so the continuation starts at the column edge with its
		// chords still over its words.
		drop := leadingBlanks(lr)
		if len(lr) == 0 {
			drop = leadingBlanks(cr)
		}
		if drop > 0 {
			cr, lr = cr[min(drop, len(cr)):], lr[min(drop, len(lr)):]
		}
		if len(cr) == 0 && len(lr) == 0 {
			return out
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// proseToken is one word of a body line plus whether it is inside a **bold** run.
type proseToken struct {
	word string
	bold bool
}

// wrapProse splits a normal body line (Helvetica, honouring inline **bold**) into lines that each fit colW.
// Bold runs are reconstructed per output line, so a wrap inside a bold phrase keeps both halves bold.
func wrapProse(m *fpdf.Fpdf, tr func(string) string, line string, scale, colW float64) []string {
	var toks []proseToken
	bold := false
	for _, seg := range strings.Split(line, "**") {
		for _, w := range strings.Fields(seg) {
			toks = append(toks, proseToken{w, bold})
		}
		bold = !bold
	}
	if len(toks) == 0 {
		return []string{line}
	}
	// If the line already fits, return it VERBATIM. Re-joining tokens would collapse the author's own
	// spacing (two spaces, an indent) and change the bytes of charts that never needed wrapping.
	if proseWidth(m, tr, line, scale) <= colW {
		return []string{line}
	}
	width := func(s string, b bool) float64 {
		if b {
			m.SetFont("Helvetica", "B", 11*scale)
		} else {
			m.SetFont("Helvetica", "", 11*scale)
		}
		return m.GetStringWidth(tr(s))
	}
	space := width(" ", false)
	var out []string
	var cur []proseToken
	var w float64
	flush := func() {
		if len(cur) > 0 {
			out = append(out, renderProse(cur))
			cur, w = nil, 0
		}
	}
	for _, t := range toks {
		tw := width(t.word, t.bold)
		next := tw
		if len(cur) > 0 {
			next = w + space + tw
		}
		if len(cur) > 0 && next > colW {
			flush()
			next = tw
		}
		cur = append(cur, t)
		w = next
	}
	flush()
	return out
}

// renderProse rebuilds one output line's source text, re-opening `**` wherever the bold run continues.
func renderProse(toks []proseToken) string {
	var b strings.Builder
	bold := false
	for i, t := range toks {
		if i > 0 {
			b.WriteString(" ")
		}
		if t.bold != bold {
			b.WriteString("**")
			bold = t.bold
		}
		b.WriteString(t.word)
	}
	if bold {
		b.WriteString("**")
	}
	return b.String()
}

// proseWidth measures a body line as it will be DRAWN — each **bold** run in its own face, concatenated —
// so "does it fit?" is asked of the same glyphs textLine will put on the page.
func proseWidth(m *fpdf.Fpdf, tr func(string) string, line string, scale float64) float64 {
	total, bold := 0.0, false
	for _, seg := range strings.Split(line, "**") {
		if seg != "" {
			if bold {
				m.SetFont("Helvetica", "B", 11*scale)
			} else {
				m.SetFont("Helvetica", "", 11*scale)
			}
			total += m.GetStringWidth(tr(seg))
		}
		bold = !bold
	}
	return total
}

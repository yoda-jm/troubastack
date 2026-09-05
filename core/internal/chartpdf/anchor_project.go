package chartpdf

import "math"

// T145 — a SOURCE-SCOPED text anchor for annotations. A mark used to store `(page, fractional x/y)` of ONE
// render, so any reflow (a renderer change, an auto-fit change, one added lyric line) moved the words out
// from under it while the mark stayed put. A SourceAnchor instead names WHAT text the mark sits on, in
// terms of the source alone:
//
//   - RunText     the drawn run's text (a chord row, a lyric line, a bold segment, a section label);
//   - Occurrence  the 1-based Nth run with that text IN THE SOURCE (document-wide, NOT per page — a page
//     index is a render property, so counting per page would re-break on reflow, ⛔BLOCKER 1);
//   - CharStart/CharEnd  the [start,end) rune span within the run the mark covers (a highlight over part
//     of a line).
//
// Nothing here is derived from a render: the same anchor resolves against ANY render of the source, so a
// reflow relocates the mark to the same words instead of orphaning it. `Project` turns it back into render
// coordinates at draw/bake time (Studio has the live render; the bake re-projects server-side before
// rasterizing overlays). RunText also lets a near-match relocate a mark after a routine edit; when the run
// is gone entirely, the caller must flag it (never silently re-anchor to different words).
type SourceAnchor struct {
	RunText    string `json:"runText"`
	Occurrence int    `json:"occurrence"` // 1-based, document-wide (source order)
	CharStart  int    `json:"charStart"`  // rune index within the run
	CharEnd    int    `json:"charEnd"`
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// AnchorAt builds a SourceAnchor for a mark drawn at the [0,1] box (mx0,my0)-(mx1,my1) on `page`, given a
// render's anchors (from RenderWithAnchors, which are already in source order: page, then top, then left).
// It picks the run under the mark's centre and records its document-wide occurrence + the rune span the
// mark's x-range covers. ok is false when the mark is over no text run (e.g. whitespace) — the caller then
// keeps the raw coordinates and flags the mark as un-anchorable, never guesses.
func AnchorAt(anchors []Anchor, page int, mx0, my0, mx1, my1 float64) (SourceAnchor, bool) {
	cx, cy := (mx0+mx1)/2, (my0+my1)/2
	best := -1
	for i, a := range anchors {
		if a.Page != page || a.Text == "" {
			continue
		}
		if cx >= a.X0 && cx <= a.X1 && cy >= a.Y0 && cy <= a.Y1 {
			best = i
			break
		}
	}
	if best < 0 {
		return SourceAnchor{}, false
	}
	a := anchors[best]
	occ := 0 // document-wide occurrence: count same-text runs up to and including this one, in source order
	for i := 0; i <= best; i++ {
		if anchors[i].Text == a.Text {
			occ++
		}
	}
	n := len([]rune(a.Text))
	cs, ce := 0, n
	if w := a.X1 - a.X0; w > 0 && n > 0 {
		cs = clampInt(int(math.Round((mx0-a.X0)/w*float64(n))), 0, n)
		ce = clampInt(int(math.Round((mx1-a.X0)/w*float64(n))), cs, n)
	}
	return SourceAnchor{RunText: a.Text, Occurrence: occ, CharStart: cs, CharEnd: ce}, true
}

// Project resolves a SourceAnchor against a render's anchors, returning the mark's box in [0,1] on the
// page the run now occupies. ok is false when the run is no longer present (the source text was edited
// away) — the caller flags the mark rather than moving it to unrelated words. Because Occurrence is
// document-wide, a run that reflowed onto a different page still resolves to the right text.
func (sa SourceAnchor) Project(anchors []Anchor) (page int, x0, y0, x1, y1 float64, ok bool) {
	seen := 0
	for _, a := range anchors {
		if a.Text != sa.RunText {
			continue
		}
		seen++
		if seen != sa.Occurrence {
			continue
		}
		n := len([]rune(a.Text))
		w := a.X1 - a.X0
		f0, f1 := 0.0, 1.0
		if n > 0 {
			f0 = float64(clampInt(sa.CharStart, 0, n)) / float64(n)
			f1 = float64(clampInt(sa.CharEnd, 0, n)) / float64(n)
		}
		return a.Page, a.X0 + f0*w, a.Y0, a.X0 + f1*w, a.Y1, true
	}
	return 0, 0, 0, 0, 0, false
}

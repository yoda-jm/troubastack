package chartpdf

import (
	"math"

	"troubastack/core/internal/domain"
)

// T145 — projection between a mark's render coordinates and its SOURCE-scoped anchor (domain.SourceAnchor).
// A mark used to store (page, fractional x/y) of ONE render, so any reflow moved the words out from under
// it. domain.SourceAnchor names WHAT text the mark sits on in source terms alone (RunText + document-wide
// Occurrence + rune span), so it resolves against any render of the source. AnchorAt builds one from a
// drawn mark; Project resolves it back to a box on whatever page the run now occupies. Both work off the
// T95 RenderWithAnchors manifest. The type lives in domain (a data-model field on Object); the projection
// lives here because it needs the renderer.

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// AnchorAt builds a domain.SourceAnchor for a mark drawn at the [0,1] box (mx0,my0)-(mx1,my1) on `page`,
// given a render's anchors (from RenderWithAnchors — already in source order: page, then top, then left).
// It picks the run under the mark's centre and records its DOCUMENT-WIDE occurrence + the rune span the
// mark's x-range covers. ok is false when the mark is over no text run (e.g. whitespace) — the caller then
// keeps the raw coordinates and flags the mark as un-anchorable, never guesses.
func AnchorAt(anchors []Anchor, page int, mx0, my0, mx1, my1 float64) (domain.SourceAnchor, bool) {
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
		return domain.SourceAnchor{}, false
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
	return domain.SourceAnchor{RunText: a.Text, Occurrence: occ, CharStart: cs, CharEnd: ce}, true
}

// Project resolves a domain.SourceAnchor against a render's anchors, returning the mark's box in [0,1] on
// the page the run now occupies. ok is false when the run is no longer present (the source text was edited
// away) — the caller flags the mark rather than moving it to unrelated words. Because Occurrence is
// document-wide, a run that reflowed onto a different page still resolves to the right text.
func Project(sa domain.SourceAnchor, anchors []Anchor) (page int, x0, y0, x1, y1 float64, ok bool) {
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

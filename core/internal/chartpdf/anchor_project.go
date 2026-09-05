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

// AnchorObject attaches a source anchor to a mark being CREATED on a generated chart: anchors is the
// current render's manifest and renderHash its content identity. It sets Anchor from the run under the
// mark + stamps PointsRenderHash (so the projected Points cache is later known current). A mark over no
// run (whitespace) or one already anchored is returned unchanged — for an uploaded PDF (no source) the
// caller simply never calls this, and the frozen coordinates stand. This is the T145 forward fix's
// create-time half: a mark drawn now records WHAT WORDS it is on, so a later size/render change moves it
// with them.
func AnchorObject(o domain.Object, anchors []Anchor, renderHash string) domain.Object {
	if o.Anchor != nil {
		return o
	}
	x0, y0, x1, y1 := boundsOf(o.Points)
	if sa, ok := AnchorAt(anchors, o.Page, x0, y0, x1, y1); ok {
		o.Anchor = &sa
		o.PointsRenderHash = renderHash
	}
	return o
}

// Reproject re-projects any mark whose cached Points are STALE (has an Anchor and PointsRenderHash != the
// current renderHash) onto the current render, and restamps the hash. Used at SERVE and BAKE so a mark
// follows its words after a reflow. Marks with no anchor, or already current, pass through untouched; a
// mark whose run is gone (Project fails) is left on its frozen coordinates (its stale hash remains, so a
// warn layer can still flag it) — never silently moved to different words. Returns the objects and the
// count re-projected.
func Reproject(objs []domain.Object, anchors []Anchor, renderHash string) ([]domain.Object, int) {
	out := make([]domain.Object, len(objs))
	changed := 0
	for i, o := range objs {
		out[i] = o.Clone()
		if o.Anchor == nil || o.PointsRenderHash == renderHash {
			continue
		}
		pg, nx0, ny0, nx1, ny1, ok := Project(*o.Anchor, anchors)
		if !ok {
			continue
		}
		out[i].Page = pg
		out[i].Points = remap(o.Points, nx0, ny0, nx1, ny1)
		out[i].PointsRenderHash = renderHash
		changed++
	}
	return out, changed
}

// remap fits a mark's points into the new [0,1] box (nx0,ny0)-(nx1,ny1). A box-like mark (≤2 points:
// rect/line/highlight/text/icon) becomes exactly that box. A freehand PATH is translated + scaled from its
// old bounding box into the new one, preserving its shape as it follows the words to their new size.
func remap(pts []domain.Point, nx0, ny0, nx1, ny1 float64) []domain.Point {
	if len(pts) <= 2 {
		return []domain.Point{{X: nx0, Y: ny0}, {X: nx1, Y: ny1}}
	}
	ox0, oy0, ox1, oy1 := boundsOf(pts)
	sx, sy := 1.0, 1.0
	if ow := ox1 - ox0; ow > 1e-9 {
		sx = (nx1 - nx0) / ow
	}
	if oh := oy1 - oy0; oh > 1e-9 {
		sy = (ny1 - ny0) / oh
	}
	out := make([]domain.Point, len(pts))
	for i, p := range pts {
		out[i] = domain.Point{X: nx0 + (p.X-ox0)*sx, Y: ny0 + (p.Y-oy0)*sy, Pressure: p.Pressure}
	}
	return out
}

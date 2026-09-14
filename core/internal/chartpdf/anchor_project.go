package chartpdf

import (
	"math"
	"sort"

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

// sourceOrder returns the anchors' indices in SOURCE order — the order the runs were drawn, which is the
// order they appear in the chart source, because the layout walks the source once and fills one column
// before starting the next.
//
// This is NOT the order of the slice. The manifest is sorted for presentation (page, then top, then left),
// and in a two-column chart both columns share a Y range, so that sort interleaves them row by row. Counting
// Occurrence along the slice therefore gave the same source line a different number depending on how the
// page happened to be laid out, and a mark re-projected across a re-layout landed on another line (T146).
//
// A hand-built manifest (tests, fixtures) leaves every Seq at 0; the sort is stable, so it keeps slice
// order and behaves exactly as it did before this existed.
func sourceOrder(anchors []Anchor) []int {
	idx := make([]int, len(anchors))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return anchors[idx[i]].Seq < anchors[idx[j]].Seq })
	return idx
}

// AnchorAt builds a domain.SourceAnchor for a mark drawn at the [0,1] box (mx0,my0)-(mx1,my1) on `page`,
// given a render's anchors (from RenderWithAnchors). The occurrence it records is counted in SOURCE order
// (sourceOrder), so it does not depend on the layout the mark happened to be made on.
// It picks the run under the mark's centre and records its DOCUMENT-WIDE occurrence + the rune span the
// mark's x-range covers. ok is false when the mark is over no text run (e.g. whitespace) — the caller then
// keeps the raw coordinates and flags the mark as un-anchorable, never guesses.
// maxOffsetRunHeights bounds T172's nearest-run search: beyond it a mark has no defensible referent and
// stays un-anchorable (R3's refusal, kept).
//
// 3.0, measured rather than chosen. Across the real unanchored marks on VLL's library the distance to the
// nearest run was 1.09 run-heights at the median, 2.65 at the 90th percentile, and one outlier at 6.49 —
// an icon six line-heights above any text, which is exactly the "middle of a blank half-page" the refusal
// exists for. 2.65 would fit the sample; 3.0 fits the phenomenon with room to be wrong in (Fable). The
// rule self-limits: on a dense chart nothing is ever 3 run-heights from all text, so the radius only bites
// on sparse pages, which is where refusing belongs.
const maxOffsetRunHeights = 3.0

// nearestRunBelowAbove finds the run a mark is vertically ADJACENT to: the closest run, in run-heights,
// whose horizontal extent overlaps the mark's.
//
// The horizontal-overlap requirement is the line between adjacency we can defend and a guess. A mark with
// no overlap at all is beside a BLOCK, not a run — anchoring it to whichever run is nearest and offsetting
// sideways would record a relationship we cannot justify, and it would drift silently on the next
// re-render. Those stay un-anchorable on purpose (Fable, T172 rulings).
func nearestRunBelowAbove(anchors []Anchor, page int, mx0, my0, mx1, my1 float64) (int, bool) {
	best, bestD := -1, math.MaxFloat64
	for i, a := range anchors {
		if a.Page != page || a.Text == "" {
			continue
		}
		h := a.Y1 - a.Y0
		if h <= 0 {
			continue
		}
		if mx1 < a.X0 || mx0 > a.X1 { // no horizontal overlap: beside a block, not this run
			continue
		}
		var gap float64
		switch {
		case my0 > a.Y1: // the mark is below the run
			gap = my0 - a.Y1
		case my1 < a.Y0: // above it
			gap = a.Y0 - my1
		default:
			gap = 0 // vertically overlapping already
		}
		if d := gap / h; d < bestD {
			best, bestD = i, d
		}
	}
	if best < 0 || bestD > maxOffsetRunHeights {
		return -1, false
	}
	return best, true
}

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
	// T172: the centre test answers "is the mark ON a run". Most real marks are not — an underline sits
	// below its line, a bracket above its section — and the relationship is adjacency, which nothing used
	// to record. Fall back to the nearest run the mark overlaps horizontally, and carry the offset so
	// re-projection puts the mark back BESIDE the run rather than on top of it.
	offset := false
	if best < 0 {
		if i, ok := nearestRunBelowAbove(anchors, page, mx0, my0, mx1, my1); ok {
			best, offset = i, true
		}
	}
	if best < 0 {
		return domain.SourceAnchor{}, false
	}
	a := anchors[best]
	// Document-wide occurrence: count same-text runs up to and including this one, walking SOURCE order —
	// never the slice, which is in presentation order (see sourceOrder).
	occ, found := 0, false
	for _, i := range sourceOrder(anchors) {
		if anchors[i].Text != a.Text {
			continue
		}
		occ++
		if i == best {
			found = true
			break
		}
	}
	if !found {
		return domain.SourceAnchor{}, false // unreachable: best has this text
	}
	n := len([]rune(a.Text))
	cs, ce := 0, n
	if w := a.X1 - a.X0; w > 0 && n > 0 {
		cs = clampInt(int(math.Round((mx0-a.X0)/w*float64(n))), 0, n)
		ce = clampInt(int(math.Round((mx1-a.X0)/w*float64(n))), cs, n)
	}
	sa := domain.SourceAnchor{RunText: a.Text, Occurrence: occ, CharStart: cs, CharEnd: ce}
	if offset {
		// The mark's vertical extent in the RUN'S OWN heights, from the run's top edge. Absent when the
		// mark is on its run, so every pre-T172 anchor and every on-run mark keeps today's behaviour
		// exactly — presence is the discriminator, and it cannot disagree with the payload.
		if h := a.Y1 - a.Y0; h > 0 {
			sa.Offset = &domain.AnchorOffset{RelY0: (my0 - a.Y0) / h, RelY1: (my1 - a.Y0) / h}
		}
	}
	return sa, true
}

// Project resolves a domain.SourceAnchor against a render's anchors, returning the mark's box in [0,1] on
// the page the run now occupies. ok is false when the run is no longer present (the source text was edited
// away) — the caller flags the mark rather than moving it to unrelated words. Because Occurrence is
// document-wide AND counted in source order, a run that reflowed onto a different page — or into another
// column — still resolves to the right text (T146).
func Project(sa domain.SourceAnchor, anchors []Anchor) (page int, x0, y0, x1, y1 float64, ok bool) {
	a, found := runAt(anchors, sa.RunText, sa.Occurrence)
	if !found {
		return 0, 0, 0, 0, 0, false
	}
	n := len([]rune(a.Text))
	w := a.X1 - a.X0
	f0, f1 := 0.0, 1.0
	if n > 0 {
		f0 = float64(clampInt(sa.CharStart, 0, n)) / float64(n)
		f1 = float64(clampInt(sa.CharEnd, 0, n)) / float64(n)
	}
	x0, x1 = a.X0+f0*w, a.X0+f1*w
	y0, y1 = a.Y0, a.Y1

	// T172 R2 — a span covers runs N..M: take the vertical extent across both ends, and the horizontal
	// extent too, since a bracket over two lines is as wide as the wider of them. A span whose far end has
	// been edited away, or has reflowed to another page, falls back to the first run rather than stretching
	// a mark across a page break: the near end is still a defensible referent, a cross-page box is not.
	if sa.Span != nil {
		if b, ok2 := runAt(anchors, sa.Span.RunText, sa.Span.Occurrence); ok2 && b.Page == a.Page {
			y0, y1 = math.Min(a.Y0, b.Y0), math.Max(a.Y1, b.Y1)
			x0, x1 = math.Min(x0, b.X0), math.Max(x1, b.X1)
		}
	}

	// T172 R1 — the mark sat BESIDE the run, so put it back beside it. Without this an underline would
	// project onto the words it underlines, which is worse than leaving it where it was: the feature would
	// be moving marks on top of the text it is meant to keep them clear of.
	if sa.Offset != nil {
		h := y1 - y0
		if h > 0 {
			top := y0
			y0, y1 = top+sa.Offset.RelY0*h, top+sa.Offset.RelY1*h
		}
	}
	return a.Page, x0, y0, x1, y1, true
}

// runAt resolves the (text, document-wide occurrence) pair to a run, counting in SOURCE order so a
// re-layout cannot renumber it (T146). ok is false when the run is gone — the source text was edited away.
func runAt(anchors []Anchor, text string, occurrence int) (Anchor, bool) {
	seen := 0
	for _, i := range sourceOrder(anchors) {
		a := anchors[i]
		if a.Text != text {
			continue
		}
		seen++
		if seen == occurrence {
			return a, true
		}
	}
	return Anchor{}, false
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

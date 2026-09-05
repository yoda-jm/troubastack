package chartpdf

import "troubastack/core/internal/domain"

// T145 migration — give a legacy mark (page + frozen [0,1] Points, no Anchor) a SOURCE-scoped anchor by
// reverse-looking-up the text it sat on IN THE RENDER WHERE IT WAS STILL CORRECT.
//
// ⛔ Fable BLOCKER 2: that render is the FROZEN 08-22 one (troubastack-demo/data.preseed-20260904-191837),
// NOT the current post-reflow render. Anchoring against the current render would pin every mark to whatever
// text now happens to sit under it — silently canonicalising the corruption and destroying the last chance
// to recover the real intent. So the correct render's anchor manifest is passed IN; this package never
// reaches for the live render itself.
//
// Where the mark sits over no text run in that render, it is left on its frozen coordinates and COUNTED —
// never guessed, never silently dropped.

// MigrateReport summarises a batch reverse-anchoring.
type MigrateReport struct {
	Migrated        int // gained a source anchor
	AlreadyAnchored int // had one already; left untouched
	Unmigratable    int // over no text run in the correct render — frozen coords kept, flagged here
}

// boundsOf is the [0,1] bounding box of a mark's points (min/max over the path/corners).
func boundsOf(pts []domain.Point) (x0, y0, x1, y1 float64) {
	if len(pts) == 0 {
		return 0, 0, 0, 0
	}
	x0, y0 = pts[0].X, pts[0].Y
	x1, y1 = x0, y0
	for _, p := range pts[1:] {
		if p.X < x0 {
			x0 = p.X
		}
		if p.X > x1 {
			x1 = p.X
		}
		if p.Y < y0 {
			y0 = p.Y
		}
		if p.Y > y1 {
			y1 = p.Y
		}
	}
	return
}

// MigrateObject reverse-anchors ONE mark from correctAnchors (the anchor manifest of the render the mark
// was correct on). It sets Anchor from the run under the mark's bounding box and stamps PointsRenderHash so
// the projected cache self-invalidates. ok=false when the mark is over no run — the caller keeps the frozen
// Points and counts it. A mark that already has an Anchor is returned unchanged (ok=true).
func MigrateObject(o domain.Object, correctAnchors []Anchor, renderHash string) (domain.Object, bool) {
	if o.Anchor != nil {
		return o, true
	}
	x0, y0, x1, y1 := boundsOf(o.Points)
	sa, ok := AnchorAt(correctAnchors, o.Page, x0, y0, x1, y1)
	if !ok {
		return o, false
	}
	o.Anchor = &sa
	o.PointsRenderHash = renderHash
	return o, true
}

// MigrateObjects reverse-anchors a batch and returns the migrated objects (order preserved) + a report.
// Objects that could not be anchored keep their frozen coordinates unchanged and are counted, never dropped.
func MigrateObjects(objs []domain.Object, correctAnchors []Anchor, renderHash string) ([]domain.Object, MigrateReport) {
	out := make([]domain.Object, len(objs))
	var r MigrateReport
	for i, o := range objs {
		switch {
		case o.Anchor != nil:
			out[i] = o
			r.AlreadyAnchored++
		default:
			m, ok := MigrateObject(o, correctAnchors, renderHash)
			out[i] = m
			if ok {
				r.Migrated++
			} else {
				r.Unmigratable++
			}
		}
	}
	return out, r
}

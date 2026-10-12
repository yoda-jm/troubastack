package main

import (
	"math"
	"sort"

	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
)

// The OLD (T145/T172) projection of an old-style anchor, ported verbatim from chartpdf/anchor_project.go as
// it was before T195 deleted it (19a5b656: Project, runAt, sourceOrder). It is used for ONE thing: the
// dashed orange box on a review crop, which shows VLL where the old link wanted the mark, so he can judge
// whether the mark's current (stored) position is the right one to re-link from. Nothing here is ever
// written: the proposal and the apply use chartpdf.LinkMark alone.
//
// For a mark of any shape the old Reproject put its points into exactly the box Project returned (remap:
// a ≤2-point mark becomes the box, a path is fitted into it), so the box is the whole answer.

// oldSourceOrder is the anchors' indices in SOURCE (draw) order, by Anchor.Seq.
func oldSourceOrder(anchors []chartpdf.Anchor) []int {
	idx := make([]int, len(anchors))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(i, j int) bool { return anchors[idx[i]].Seq < anchors[idx[j]].Seq })
	return idx
}

// oldRunAt resolves (text, document-wide occurrence) to a run, counting in source order.
func oldRunAt(anchors []chartpdf.Anchor, text string, occurrence int) (chartpdf.Anchor, bool) {
	seen := 0
	for _, i := range oldSourceOrder(anchors) {
		a := anchors[i]
		if a.Text != text {
			continue
		}
		seen++
		if seen == occurrence {
			return a, true
		}
	}
	return chartpdf.Anchor{}, false
}

func oldClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// oldProject is the pre-T195 chartpdf.Project: the box (page, x0, y0, x1, y1) an old-style anchor resolved
// to on a render. ok=false when its run is gone.
func oldProject(sa domain.SourceAnchor, anchors []chartpdf.Anchor) (page int, x0, y0, x1, y1 float64, ok bool) {
	a, found := oldRunAt(anchors, sa.RunText, sa.Occurrence)
	if !found {
		return 0, 0, 0, 0, 0, false
	}
	n := len([]rune(a.Text))
	w := a.X1 - a.X0
	f0, f1 := 0.0, 1.0
	if n > 0 {
		f0 = float64(oldClamp(sa.CharStart, 0, n)) / float64(n)
		f1 = float64(oldClamp(sa.CharEnd, 0, n)) / float64(n)
	}
	x0, x1 = a.X0+f0*w, a.X0+f1*w
	y0, y1 = a.Y0, a.Y1
	if sa.Span != nil {
		if b, ok2 := oldRunAt(anchors, sa.Span.RunText, sa.Span.Occurrence); ok2 && b.Page == a.Page {
			y0, y1 = math.Min(a.Y0, b.Y0), math.Max(a.Y1, b.Y1)
			x0, x1 = math.Min(x0, b.X0), math.Max(x1, b.X1)
		}
	}
	if sa.Offset != nil {
		if h := y1 - y0; h > 0 {
			top := y0
			y0, y1 = top+sa.Offset.RelY0*h, top+sa.Offset.RelY1*h
		}
	}
	return a.Page, x0, y0, x1, y1, true
}

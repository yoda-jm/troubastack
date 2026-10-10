package chartpdf

import (
	"testing"

	"troubastack/core/internal/domain"
)

// line is one manifest run (0.6 wide at x=0.2) at a given source Seq and vertical position.
func line(seq int, text string, top, h float64) Anchor {
	return Anchor{Page: 0, Text: text, X0: 0.2, X1: 0.8, Y0: top, Y1: top + h, Seq: seq}
}

// box is a small mark rectangle centred at (cx, cy), the shape AnchorAt/boundsOf read.
func box(cx, cy float64) []domain.Point {
	return []domain.Point{{X: cx - 0.05, Y: cy - 0.01}, {X: cx + 0.05, Y: cy + 0.01}}
}

// TestMovedMarkStaysWhereYouPutItWhenTextChanges is the reported failure, end to end (T191 §5): a mark is
// created low, then dragged up beside an upper line; later a section is inserted BELOW the upper line, which
// pushes the original line down. Before the fix the move never touched the anchor, so Reproject rebuilt the
// position from the create-time anchor alone and the mark snapped back down to the line it was dragged away
// from. After the fix the move re-anchors to the upper line, so Reproject keeps it there.
//
// Two renders by hand (deterministic): render1 is title/alpha/beta/gamma/delta; render2 inserts two lines
// between beta and gamma, so gamma+delta drop by 0.20 while title..beta stay put.
func TestMovedMarkStaysWhereYouPutItWhenTextChanges(t *testing.T) {
	const h = 0.04
	render1 := []Anchor{
		line(0, "title", 0.10, h),
		line(1, "alpha", 0.20, h),
		line(2, "beta", 0.30, h),
		line(3, "gamma", 0.40, h),
		line(4, "delta", 0.50, h),
	}
	render2 := []Anchor{
		line(0, "title", 0.10, h),
		line(1, "alpha", 0.20, h), // unchanged — it is above the insertion
		line(2, "beta", 0.30, h),
		line(3, "ins1", 0.40, h),
		line(4, "ins2", 0.50, h),
		line(5, "gamma", 0.60, h), // pushed down by two lines
		line(6, "delta", 0.70, h), // pushed down by two lines
	}
	const hash1, hash2 = "render-1", "render-2"

	// 1. Create a mark on "delta" (the low line): it anchors there.
	o := domain.Object{Page: 0, Points: box(0.5, 0.52)}
	o = AnchorObject(o, render1, hash1)
	if o.Anchor == nil || o.Anchor.RunText != "delta" {
		t.Fatalf("create anchored to %+v, want delta", o.Anchor)
	}

	// 2. Drag it up beside "alpha" and re-anchor against the render it was moved on.
	o.Points = box(0.5, 0.22)
	o = Reanchor(o, render1, hash1)
	if o.Anchor == nil || o.Anchor.RunText != "alpha" {
		t.Fatalf("after the move the anchor is %+v, want alpha", o.Anchor)
	}

	// 3. The text changes (two lines inserted below alpha) and the chart re-renders. Reproject onto render2.
	out, n := Reproject([]domain.Object{o}, render2, hash2)
	if n != 1 {
		t.Fatalf("Reproject re-projected %d marks, want 1", n)
	}
	gotY := out[0].Points[0].Y
	// The mark must be at alpha's row (~0.20), NOT at delta's new, lower row (~0.70).
	if gotY > 0.30 {
		t.Fatalf("the mark followed the text down to y=%.3f; it should have stayed beside alpha (~0.20)", gotY)
	}

	// Discriminator (this is what the move re-anchor buys): had the move NOT re-anchored — keeping the
	// create-time delta anchor — Reproject would put the mark on delta's NEW low position. If these two were
	// the same, the test would pass even with the fix removed, so prove they differ.
	stale := domain.Object{Page: 0, Points: box(0.5, 0.22),
		Anchor:           &domain.SourceAnchor{RunText: "delta", Occurrence: 1},
		PointsRenderHash: hash1}
	staleOut, _ := Reproject([]domain.Object{stale}, render2, hash2)
	if staleY := staleOut[0].Points[0].Y; staleY < 0.60 {
		t.Fatalf("expected the un-re-anchored mark to follow delta down to ~0.70, got y=%.3f — the vector does not discriminate", staleY)
	}
}

// TestMoveIntoEmptySpaceClearsTheAnchor is T191's refusal, applied to an explicit placement: a mark dragged
// into empty space (more than 3 run-heights from any overlapping run) loses its anchor, so it keeps the
// coordinates the user chose and a later re-render never drags it back (§5).
func TestMoveIntoEmptySpaceClearsTheAnchor(t *testing.T) {
	const h = 0.04
	render := []Anchor{
		line(0, "alpha", 0.20, h),
		line(1, "beta", 0.24, h),
	}
	// Anchored first (on alpha), then moved far below everything (> 3 run-heights under beta, which ends at
	// 0.28): y ≈ 0.80 is ~13 run-heights down.
	o := domain.Object{Page: 0, Points: box(0.5, 0.21)}
	o = AnchorObject(o, render, "r1")
	if o.Anchor == nil {
		t.Fatal("precondition: the create should have anchored on alpha")
	}
	o.Points = box(0.5, 0.80)
	o = Reanchor(o, render, "r1")
	if o.Anchor != nil {
		t.Fatalf("a move into empty space must CLEAR the anchor, got %+v", o.Anchor)
	}
	// A later re-render leaves it exactly in place (Reproject skips nil-anchor marks).
	out, n := Reproject([]domain.Object{o}, render, "r2")
	if n != 0 {
		t.Fatalf("a cleared-anchor mark must not re-project, got %d", n)
	}
	if out[0].Points[0].Y != o.Points[0].Y {
		t.Fatalf("the mark moved: y %.3f -> %.3f", o.Points[0].Y, out[0].Points[0].Y)
	}
}

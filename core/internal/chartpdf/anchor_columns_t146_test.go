package chartpdf

import (
	"fmt"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// T146 stage 2 — the assertion that was missing when the two-column layout landed, and that I should have
// demanded when I made stage 2 conditional on "a mark re-projects across the re-layout".
//
// A mark is anchored by its run text plus a DOCUMENT-WIDE occurrence. That occurrence used to be counted by
// walking the anchor manifest, which is sorted for PRESENTATION (page, then top, then left). In one column
// that order happens to equal source order; in two columns both columns share a Y range, so the sort
// interleaves them and the same source line answers to a different occurrence number. A mark made on the
// one-column render then re-projected onto the two-column render landed on a DIFFERENT line, in the other
// column, with ok == true and a plausible box — silently.
//
// The test is deliberately EXTERNAL to the mechanism: it never mentions draw order or sequence numbers. The
// repeated run is a CHORD line and each copy carries a UNIQUE lyric beneath it — a chord/lyric pair never
// splits across a column (the T146 pair rule), so the unique lyric is a landmark that is guaranteed to sit
// with its chord line. The assertion is then the one a musician would make: the mark is still on its line.
const repeatedChords = "G     D     Am    C"

// Short enough to fit a HALF-width column: T168 wraps a line that does not, which would split this
// landmark in two and make the test about wrapping instead of about occurrence stability.
func waypointLyric(k int) string { return fmt.Sprintf("waypoint %d here", k) }

// columnsFixture: a body long enough to need two columns, carrying `pairs` copies of the SAME chord line,
// each over its own unique lyric, spread so that at least one pair falls on each side of the column break —
// which is where the interleaving bites. Invented words only, no band data (T146 ⟨R1⟩).
func columnsFixture(twoCol bool, pairs int) string {
	var b strings.Builder
	b.WriteString("# An Invented Chart\n")
	if twoCol {
		b.WriteString("columns: 2\n")
	}
	b.WriteString("\n## Verse\n")
	const every = 8
	for i := 0; i < pairs*every; i++ {
		if i%every == 0 {
			fmt.Fprintf(&b, "%s\n%s\n", repeatedChords, waypointLyric(i/every+1))
			continue
		}
		fmt.Fprintf(&b, "filler %02d short\n", i)
	}
	return b.String()
}

// findUnique locates a landmark by text, failing unless it is unique — so the test never leans on
// occurrence, the very thing under test.
func findUnique(t *testing.T, anchors []Anchor, text string) Anchor {
	t.Helper()
	var found []Anchor
	for _, a := range anchors {
		if a.Text == text {
			found = append(found, a)
		}
	}
	if len(found) != 1 {
		t.Fatalf("landmark %q appears %d times, want exactly 1", text, len(found))
	}
	return found[0]
}

// chordAboveWaypoint returns the repeated chord run belonging to waypoint k — the copy directly above it in
// the same column. Pairs never split, so "same page, same left edge, nearest above" identifies it exactly.
func chordAboveWaypoint(t *testing.T, anchors []Anchor, k int) Anchor {
	t.Helper()
	wp := findUnique(t, anchors, waypointLyric(k))
	best := Anchor{}
	for _, a := range anchors {
		if a.Text != repeatedChords || a.Page != wp.Page || a.Y0 >= wp.Y0 {
			continue
		}
		if diff := a.X0 - wp.X0; diff > 1e-9 || diff < -1e-9 {
			continue // another column
		}
		if best.Text == "" || a.Y0 > best.Y0 {
			best = a
		}
	}
	if best.Text == "" {
		t.Fatalf("k=%d: no chord line found above waypoint %d", k, k)
	}
	return best
}

// waypointBelow reports which waypoint sits immediately under a box — the landmark test for "which line did
// this mark end up on".
func waypointBelow(anchors []Anchor, page int, x0, y0 float64) string {
	best, bestDy := "", 0.0
	for _, a := range anchors {
		if a.Page != page || !strings.HasPrefix(a.Text, "waypoint ") {
			continue
		}
		if (a.X0 > 0.5) != (x0 > 0.5) {
			continue // other column
		}
		dy := a.Y0 - y0
		if dy < 0 {
			continue
		}
		if best == "" || dy < bestDy {
			best, bestDy = a.Text, dy
		}
	}
	return best
}

func TestAnchors_MarkKeepsItsLineAcrossTheColumnRelayout_T146(t *testing.T) {
	const pairs = 6
	_, oneCol, err := RenderWithAnchors(columnsFixture(false, pairs))
	if err != nil {
		t.Fatalf("one-column render: %v", err)
	}
	_, twoCol, err := RenderWithAnchors(columnsFixture(true, pairs))
	if err != nil {
		t.Fatalf("two-column render: %v", err)
	}

	// The fixture must actually straddle the column break, or this test proves nothing.
	var left, right int
	for _, a := range twoCol {
		if a.Text == repeatedChords {
			if a.X0 > 0.5 {
				right++
			} else {
				left++
			}
		}
	}
	if left == 0 || right == 0 {
		t.Fatalf("fixture does not straddle the column break (left=%d right=%d) — the test would be vacuous", left, right)
	}

	for k := 1; k <= pairs; k++ {
		// A mark made on the ONE-column render, over the chord line of pair k.
		src := chordAboveWaypoint(t, oneCol, k)
		sa, ok := AnchorAt(oneCol, src.Page, src.X0, src.Y0, src.X1, src.Y1)
		if !ok {
			t.Fatalf("k=%d: AnchorAt found no run under the mark", k)
		}

		// Re-projected onto the TWO-column render, it must still be on ITS line — the one whose lyric is
		// waypoint k.
		page, x0, y0, _, _, ok := Project(sa, twoCol)
		if !ok {
			t.Fatalf("k=%d: the mark no longer resolves at all after the re-layout", k)
		}
		if got := waypointBelow(twoCol, page, x0, y0); got != waypointLyric(k) {
			t.Errorf("k=%d (occurrence %d): the mark re-projected onto the line above %q, but it belongs above %q — it moved to another line",
				k, sa.Occurrence, got, waypointLyric(k))
		}
	}
}

// The invariant behind the test above, stated directly: the SAME source line yields the same Occurrence
// whether the chart is set in one column or two. If this holds, no re-layout can renumber a mark.
func TestAnchors_OccurrenceIsLayoutIndependent_T146(t *testing.T) {
	const pairs = 6
	_, oneCol, err := RenderWithAnchors(columnsFixture(false, pairs))
	if err != nil {
		t.Fatalf("one-column render: %v", err)
	}
	_, twoCol, err := RenderWithAnchors(columnsFixture(true, pairs))
	if err != nil {
		t.Fatalf("two-column render: %v", err)
	}
	occurrenceOf := func(anchors []Anchor, k int) int {
		a := chordAboveWaypoint(t, anchors, k)
		sa, ok := AnchorAt(anchors, a.Page, a.X0, a.Y0, a.X1, a.Y1)
		if !ok {
			t.Fatalf("k=%d: AnchorAt found nothing", k)
		}
		return sa.Occurrence
	}
	for k := 1; k <= pairs; k++ {
		one, two := occurrenceOf(oneCol, k), occurrenceOf(twoCol, k)
		if one != two {
			t.Errorf("waypoint %d: the same source line is occurrence %d in one column but %d in two — a re-layout renumbers the mark", k, one, two)
		}
	}
}

// The single-column path is untouched: occurrence is still 1,2,3… down the page.
func TestAnchors_SingleColumnOccurrenceUnchanged_T146(t *testing.T) {
	const pairs = 4
	_, anchors, err := RenderWithAnchors(columnsFixture(false, pairs))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for k := 1; k <= pairs; k++ {
		a := chordAboveWaypoint(t, anchors, k)
		sa, ok := AnchorAt(anchors, a.Page, a.X0, a.Y0, a.X1, a.Y1)
		if !ok {
			t.Fatalf("k=%d: AnchorAt found nothing", k)
		}
		if sa.Occurrence != k {
			t.Errorf("the %d-th copy down the page is occurrence %d, want %d", k, sa.Occurrence, k)
		}
	}
}

// Project must still refuse when the run is gone, rather than resolving to a neighbour.
func TestAnchors_ProjectRefusesWhenTheRunIsGone_T146(t *testing.T) {
	_, anchors, err := RenderWithAnchors(columnsFixture(true, 3))
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	sa := domain.SourceAnchor{RunText: "a line that was edited away", Occurrence: 1, CharStart: 0, CharEnd: 5}
	if _, _, _, _, _, ok := Project(sa, anchors); ok {
		t.Error("Project resolved a run that is not in the render; it must refuse so the caller can flag the mark")
	}
}

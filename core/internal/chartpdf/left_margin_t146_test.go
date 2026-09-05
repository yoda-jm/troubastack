package chartpdf

import (
	"math"
	"testing"
)

// TestLeftMargin_T146 pins the reduced left margin (T146): the leftmost drawn glyph sits at leftMargin, not
// the old 12mm all-round margin. RED first against the pre-T146 code, where every left-anchored run started
// at margin (12mm → X0 0.0571) — the expected 8mm value (0.0381) differs, so the assertion has teeth.
// Fixtures use invented lyrics only (no band data).
func TestLeftMargin_T146(t *testing.T) {
	_, anchors, err := RenderWithAnchors("# Invented Title\nInvented Artist\n\n## Verse\nAm        C\nla la la a made-up line\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(anchors) == 0 {
		t.Fatal("no anchors rendered")
	}
	// The leftmost run's left edge is the drawn left margin.
	min := 1.0
	for _, a := range anchors {
		if a.X0 < min {
			min = a.X0
		}
	}
	want := leftMargin / pageW
	if math.Abs(min-want) > 0.0005 {
		t.Fatalf("leftmost glyph X0 = %.4f, want %.4f (leftMargin %.1fmm / pageW %.0fmm)", min, want, leftMargin, pageW)
	}
	// Guard the reduction itself: leftMargin must be strictly less than the all-round margin (else this
	// task did nothing).
	if leftMargin >= margin {
		t.Fatalf("leftMargin %.1f is not less than margin %.1f — the left margin was not reduced", leftMargin, margin)
	}
}

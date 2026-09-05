package app_test

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/chartpdf"
)

// TestChartAnchorsIfCurrent is the guard on the T145 precondition (Fable's blocker): the anchor manifest is
// returned ONLY when a fresh render reproduces the stored blob byte-for-byte. A render that diverges from
// the stored blob's geometry — which every layout change (e.g. T146's margin) creates — must yield ok=false
// so no caller stamps the blob hash onto coordinates the served pixels do not have.
func TestChartAnchorsIfCurrent(t *testing.T) {
	src := "# Riverside Waltz\nThe Riverside Trio\n\n## Verse\nAm\nrolling downstream slow\n"

	// The stored blob is the current renderer's output — the normal case (charts are re-rendered by the
	// running binary), where the guard is a no-op and anchoring proceeds.
	pdf, err := chartpdf.Render(src)
	if err != nil {
		t.Fatal(err)
	}
	hash := blob.HashOf(pdf)

	anchors, ok := app.ChartAnchorsIfCurrent(src, hash)
	if !ok {
		t.Fatal("render reproduces the stored blob, but the guard withheld the manifest")
	}
	if len(anchors) == 0 {
		t.Fatal("expected a non-empty anchor manifest for a chart with text")
	}

	// A stored blob from a DIFFERENT render (any other hash) → the fresh render diverges → withhold. This is
	// the case a layout change activates; teeth: replacing the guard with an unconditional true reddens here.
	if _, ok := app.ChartAnchorsIfCurrent(src, "0000000000000000000000000000000000000000000000000000000000000000"); ok {
		t.Fatal("a divergent stored blob must yield ok=false, got ok=true (the latent-lie defect)")
	}

	// Unrenderable source → ok=false (no panic, no manifest).
	if _, ok := app.ChartAnchorsIfCurrent("\x00\x01 not a chart", "whatever"); ok {
		t.Fatal("unrenderable source must yield ok=false")
	}
}

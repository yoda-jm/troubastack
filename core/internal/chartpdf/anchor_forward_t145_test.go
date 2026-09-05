package chartpdf

import (
	"strings"
	"testing"

	"troubastack/core/internal/domain"
)

// TestAnchorObject_And_Reproject (T145 forward fix): a mark anchored when CREATED on one render follows
// its words after the chart reflows — Reproject moves it onto the run's new page/box — while a mark that
// is already current, or has no anchor, is left alone.
func TestAnchorObject_And_Reproject(t *testing.T) {
	var body strings.Builder
	words := []string{"alpha", "bravo", "charlie", "delta", "echo", "foxtrot", "golf", "hotel",
		"india", "juliet", "kilo", "lima", "mike", "november", "oscar", "romeo"}
	for _, w := range words {
		body.WriteString("C       G\nthe verse line called " + w + "\n\n")
	}
	src := body.String()
	rA := anchorsOf(t, "# Song\nsize: 8\n\n"+src)  // small → the run sits high on page 0
	rB := anchorsOf(t, "# Song\nsize: 16\n\n"+src) // large → reflows onto a later page

	target := "the verse line called romeo"
	a, _ := findAnchor(rA, target)
	b, okb := findAnchor(rB, target)
	if !okb {
		t.Fatal("target run missing in render B")
	}

	// A mark drawn over the target run in render A, anchored at create time.
	mark := domain.Object{UUID: "m", Type: domain.TypeHighlight, Page: a.Page,
		Points: []domain.Point{{X: a.X0, Y: a.Y0}, {X: a.X1, Y: a.Y1}}}
	mark = AnchorObject(mark, rA, "hashA")
	if mark.Anchor == nil || mark.Anchor.RunText != target {
		t.Fatalf("create-time anchor not set to the target run: %+v", mark.Anchor)
	}
	if mark.PointsRenderHash != "hashA" {
		t.Fatalf("create render hash not stamped: %q", mark.PointsRenderHash)
	}

	// Reflow to render B: Reproject must move the mark onto the run's NEW page/box.
	notMark := domain.Object{UUID: "n", Points: []domain.Point{{X: 0.1, Y: 0.1}, {X: 0.2, Y: 0.2}}} // no anchor
	current := mark
	current.PointsRenderHash = "hashB" // pretend already current — must be untouched
	out, changed := Reproject([]domain.Object{mark, notMark, current}, rB, "hashB")
	if changed != 1 {
		t.Fatalf("Reproject changed %d marks, want 1 (only the stale anchored one)", changed)
	}
	got := out[0]
	if got.Page != b.Page {
		t.Fatalf("re-projected page = %d, want the run's new page %d", got.Page, b.Page)
	}
	const eps = 0.01
	if abs(got.Points[0].X-b.X0) > eps || abs(got.Points[1].X-b.X1) > eps {
		t.Fatalf("re-projected box x = [%.3f,%.3f], want ~[%.3f,%.3f] (the run in render B)",
			got.Points[0].X, got.Points[1].X, b.X0, b.X1)
	}
	if got.PointsRenderHash != "hashB" {
		t.Fatalf("re-projected mark did not restamp the render hash: %q", got.PointsRenderHash)
	}
	if out[1].Anchor != nil || len(out[1].Points) != 2 || out[1].Points[0].X != 0.1 {
		t.Fatal("a mark with no anchor must pass through untouched")
	}
	if out[2].Page != current.Page || out[2].Points[0].X != current.Points[0].X {
		t.Fatal("an already-current mark must not be re-projected")
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

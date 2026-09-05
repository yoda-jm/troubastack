package chartpdf

import (
	"testing"

	"troubastack/core/internal/domain"
)

// TestMigrateObjects_ReverseAnchorsFromCorrectRender (T145 migration): a legacy mark gains a source anchor
// by reverse-looking-up the text it sat on in the CORRECT (here: stand-in for the frozen 08-22) render; a
// mark over no text keeps its frozen coordinates and is COUNTED, not guessed; an already-anchored mark is
// left untouched.
func TestMigrateObjects_ReverseAnchorsFromCorrectRender(t *testing.T) {
	src := "# Mig Test\n\n## Verse\nC       G\nthe one true lyric line\n\n## Chorus\nF       C\nanother sung line here\n"
	anchors := anchorsOf(t, src) // stands in for the frozen 08-22 render's manifest
	run, ok := findAnchor(anchors, "the one true lyric line")
	if !ok {
		t.Fatal("fixture missing the target run")
	}

	mark := domain.Object{ // drawn over the middle of that run
		UUID: "m1", Page: run.Page,
		Points: []domain.Point{{X: run.X0 + 0.02, Y: (run.Y0 + run.Y1) / 2}, {X: run.X1 - 0.02, Y: (run.Y0 + run.Y1) / 2}},
	}
	empty := domain.Object{ // over blank space near the page bottom — no run
		UUID: "m2", Page: run.Page,
		Points: []domain.Point{{X: 0.85, Y: 0.965}, {X: 0.92, Y: 0.985}},
	}
	pre := domain.Object{ // already anchored — must be left alone
		UUID: "m3", Anchor: &domain.SourceAnchor{RunText: "unchanged", Occurrence: 1},
	}

	out, rep := MigrateObjects([]domain.Object{mark, empty, pre}, anchors, "hash-0822")

	if rep.Migrated != 1 || rep.Unmigratable != 1 || rep.AlreadyAnchored != 1 {
		t.Fatalf("report = %+v, want Migrated 1 / Unmigratable 1 / AlreadyAnchored 1", rep)
	}
	if out[0].Anchor == nil || out[0].Anchor.RunText != "the one true lyric line" {
		t.Fatalf("mark not anchored to its run: %+v", out[0].Anchor)
	}
	if out[0].PointsRenderHash != "hash-0822" {
		t.Fatalf("render hash not stamped for cache self-invalidation: %q", out[0].PointsRenderHash)
	}
	if out[1].Anchor != nil {
		t.Fatalf("a mark over no text must stay un-anchored, got %+v", out[1].Anchor)
	}
	if len(out[1].Points) != 2 {
		t.Fatal("an un-migratable mark's frozen coordinates must be preserved, not dropped")
	}
	if out[2].Anchor == nil || out[2].Anchor.RunText != "unchanged" {
		t.Fatalf("an already-anchored mark must be untouched: %+v", out[2].Anchor)
	}
}

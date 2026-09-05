package bake

import (
	"testing"

	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
)

// TestSnapshotToDoc_ReprojectsStaleMark (T145 forward fix, bake side): a mark whose cached coordinates
// predate the chart's current render is re-projected onto its words before baking — so a reflowed chart
// bakes the mark on its line, not orphaned. A mark already current, or on an uploaded file (no anchors),
// is left as-is.
func TestSnapshotToDoc_ReprojectsStaleMark(t *testing.T) {
	src := "# Song\n\n## Verse\nC       G\nthe one true lyric line\n\n## Chorus\nF       C\nanother sung line here\n"
	_, anchors, err := chartpdf.RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	var run chartpdf.Anchor
	for _, a := range anchors {
		if a.Text == "the one true lyric line" {
			run = a
		}
	}
	if run.Text == "" {
		t.Fatal("fixture missing target run")
	}

	const fileID = "f1"
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "L1", FileID: fileID, Name: "Marks"}},
		Objects: []domain.Object{{
			UUID: "stale", LayerID: "L1", Type: domain.TypeHighlight,
			Page:             5,                                                      // an orphaning page (predates a shortened render)
			Points:           []domain.Point{{X: 0.90, Y: 0.90}, {X: 0.95, Y: 0.95}}, // stale coords
			Anchor:           &domain.SourceAnchor{RunText: run.Text, Occurrence: 1, CharStart: 0, CharEnd: len([]rune(run.Text))},
			PointsRenderHash: "old-render",
		}},
	}

	// current render hash differs from the mark's → it must be re-projected onto the run.
	doc := snapshotToDoc(snap, fileID, anchors, "current-render")
	if len(doc.Objects) != 1 {
		t.Fatalf("got %d objects, want 1", len(doc.Objects))
	}
	o := doc.Objects[0]
	if o.Page != run.Page {
		t.Fatalf("re-projected page = %d, want the run's page %d (mark was orphaned on page 5)", o.Page, run.Page)
	}
	const eps = 0.01
	if abs(o.Points[0].X-run.X0) > eps || abs(o.Points[1].X-run.X1) > eps {
		t.Fatalf("re-projected box x = [%.3f,%.3f], want ~[%.3f,%.3f] (the run)", o.Points[0].X, o.Points[1].X, run.X0, run.X1)
	}

	// Same mark, but already current (hash matches) → NOT re-projected: it keeps the stale coords.
	snap.Objects[0].PointsRenderHash = "current-render"
	doc2 := snapshotToDoc(snap, fileID, anchors, "current-render")
	if doc2.Objects[0].Page != 5 || doc2.Objects[0].Points[0].X != 0.90 {
		t.Fatal("an already-current mark must not be re-projected")
	}

	// No anchors (uploaded file) → no re-projection, even with a render hash.
	snap.Objects[0].PointsRenderHash = "old-render"
	doc3 := snapshotToDoc(snap, fileID, nil, "current-render")
	if doc3.Objects[0].Page != 5 {
		t.Fatal("without anchors (uploaded file) a mark must be left on its frozen coords")
	}
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}

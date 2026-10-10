package httpapi

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// reanchorFixture builds a song with a generated chart, a layer pointing at it, and ONE object stored in
// HEAD anchored on run A. It returns the adapter, the song id, the chart's current render hash, and two
// distinct text runs A and B to move between. It centres the stored mark on A.
func reanchorFixture(t *testing.T) (anc *chartAnchorer, songID, curHash string, runA, runB chartpdf.Anchor) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("marie", "Marie", "password123", "marie@x.com")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Riverside Waltz", "The Riverside Trio")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := svc.CreateTextChart(admin, band.ID, song.ID,
		"# Riverside Waltz\n\n## Verse\nrolling downstream slow\nbeneath a paper moon\nand morning finds the room\n")
	if err != nil {
		t.Fatal(err)
	}

	_, src, err := svc.ChartSourceForFile(chart.ID)
	if err != nil {
		t.Fatal(err)
	}
	_, anchors, err := chartpdf.RenderWithAnchors(src)
	if err != nil {
		t.Fatal(err)
	}
	// Two distinct text runs on the same page.
	var got []chartpdf.Anchor
	for _, a := range anchors {
		if a.Text != "" && (len(got) == 0 || a.Page == got[0].Page && a.Text != got[0].Text) {
			got = append(got, a)
		}
		if len(got) == 2 {
			break
		}
	}
	if len(got) < 2 {
		t.Fatal("need two distinct text runs in the fixture chart")
	}
	runA, runB = got[0], got[1]

	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", FileID: chart.ID, Zone: domain.ZonePersonal, OwnerID: admin.ID, Access: domain.AccessRW},
		AuthorID: admin.ID,
		Summary:  "layer",
	}); err != nil {
		t.Fatal(err)
	}

	anc = newChartAnchorer(svc, eng)
	mark := domain.Object{
		UUID: "o1", LayerID: "L1", Type: domain.TypeHighlight, Version: 1,
		Page:   runA.Page,
		Points: centreOn(runA),
	}
	mark = anc.AnchorMark(song.ID, mark) // stores the A anchor + the current render hash
	if mark.Anchor == nil || mark.Anchor.RunText != runA.Text {
		t.Fatalf("fixture: stored mark did not anchor on run A (%q): %+v", runA.Text, mark.Anchor)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind: domain.KindCreate, UUID: "o1", Object: &mark, AuthorID: admin.ID, Summary: "create",
	}); err != nil {
		t.Fatal(err)
	}
	return anc, song.ID, chart.BlobHash, runA, runB
}

func centreOn(a chartpdf.Anchor) []domain.Point {
	cx, cy := (a.X0+a.X1)/2, (a.Y0+a.Y1)/2
	return []domain.Point{{X: cx - 0.01, Y: cy - 0.005}, {X: cx + 0.01, Y: cy + 0.005}}
}

// TestReanchorMoved_ReanchorsFromNewPointsOnCurrentRender: a move onto run B, made against the current
// render, re-anchors the mark to B and stamps the current hash — the mark now tracks B's words (T191 ⟨D1⟩).
func TestReanchorMoved_ReanchorsFromNewPointsOnCurrentRender(t *testing.T) {
	anc, songID, curHash, runA, runB := reanchorFixture(t)

	moved := domain.Object{UUID: "o1", LayerID: "L1", Page: runB.Page, Points: centreOn(runB), PointsRenderHash: curHash}
	out := anc.ReanchorMoved(songID, moved)

	if out.Anchor == nil || out.Anchor.RunText != runB.Text {
		t.Fatalf("move re-anchored to %+v, want run B %q (was A %q)", out.Anchor, runB.Text, runA.Text)
	}
	if out.PointsRenderHash != curHash {
		t.Errorf("PointsRenderHash = %q, want the current render hash %q", out.PointsRenderHash, curHash)
	}
}

// TestReanchorMoved_StaleRenderKeepsOldAnchor: a move carrying a PointsRenderHash that is not the current
// render (a stale tab) must NOT re-anchor against a render the user never saw — the stored A anchor stands.
func TestReanchorMoved_StaleRenderKeepsOldAnchor(t *testing.T) {
	anc, songID, _, runA, runB := reanchorFixture(t)

	moved := domain.Object{UUID: "o1", LayerID: "L1", Page: runB.Page, Points: centreOn(runB), PointsRenderHash: "an-old-render"}
	out := anc.ReanchorMoved(songID, moved)

	if out.Anchor == nil || out.Anchor.RunText != runA.Text {
		t.Fatalf("a stale-render move changed the anchor to %+v, want the stored run A %q", out.Anchor, runA.Text)
	}
}

// TestReanchorMoved_IgnoresClientSentAnchor: the server decides the anchor. A move onto run B that also
// carries a client-crafted Anchor pointing elsewhere stores the SERVER-computed B anchor, not the client's.
func TestReanchorMoved_IgnoresClientSentAnchor(t *testing.T) {
	anc, songID, curHash, _, runB := reanchorFixture(t)

	moved := domain.Object{
		UUID: "o1", LayerID: "L1", Page: runB.Page, Points: centreOn(runB), PointsRenderHash: curHash,
		Anchor: &domain.SourceAnchor{RunText: "a line the client made up", Occurrence: 7},
	}
	out := anc.ReanchorMoved(songID, moved)

	if out.Anchor == nil || out.Anchor.RunText != runB.Text {
		t.Fatalf("client anchor leaked: stored %+v, want the server-computed run B %q", out.Anchor, runB.Text)
	}
}

// TestReanchorMoved_UnknownObjectIsLeftAsIs: a move for an object not in HEAD returns the client object
// unchanged (best-effort; the engine handles the staleness).
func TestReanchorMoved_UnknownObjectIsLeftAsIs(t *testing.T) {
	anc, songID, curHash, _, runB := reanchorFixture(t)
	moved := domain.Object{UUID: "ghost", LayerID: "L1", Page: runB.Page, Points: centreOn(runB), PointsRenderHash: curHash}
	out := anc.ReanchorMoved(songID, moved)
	if out.Anchor != nil {
		t.Fatalf("unknown object got an anchor: %+v", out.Anchor)
	}
}

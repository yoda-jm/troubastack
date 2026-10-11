package bake

import (
	"context"
	"math"
	"reflect"
	"sync"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
	"troubastack/core/internal/testutil"
)

// T195 (anchoring stage 2) — the bake half. It replaces T193's TestAnchorFreezeBake (which pinned "every
// mark bakes at its stored coordinates") and T145's TestSnapshotToDoc_ReprojectsStaleMark (which pinned
// the old text+occurrence projection, a mark re-fitted to its run's box). Stage 2 splits the population:
//   - a mark with a stage-2 link bakes where serve shows it: projected rigidly onto the chart's current
//     render by the same chartpdf.ProjectMarks (P33);
//   - an old-style anchored mark is still FROZEN at its stored coordinates (stage 4 re-links it);
//   - a chart with no current render (an uploaded file, a divergent render) bakes every mark as stored.

// docCapture is an OverlayRenderer that records the overlay docs the bake hands the renderer — the marks'
// BAKED coordinates — and draws every layer on page 0.
type docCapture struct {
	png  []byte
	mu   sync.Mutex
	docs []annotationsDoc
}

func (d *docCapture) RenderBatch(ctx context.Context, songs []overlaySong) (map[string][]renderedOverlay, error) {
	d.mu.Lock()
	for _, s := range songs {
		d.docs = append(d.docs, s.Doc)
	}
	d.mu.Unlock()
	return fakeOverlays{png: d.png}.RenderBatch(ctx, songs)
}

// TestAnchorStage2Bake/<edit>/<population>/<shape>: marks of both populations on one generated chart, the
// chart source edited (new blob hash), then a REAL Bake. Linked marks bake at their projection onto the new
// render, and that projection moved them with their line; old-style marks bake at their stored
// coordinates exactly.
func TestAnchorStage2Bake(t *testing.T) {
	for _, edit := range []string{"insert_line_above", "size_change"} {
		edit := edit
		t.Run(edit, func(t *testing.T) {
			svc := app.NewService(memrepo.New())
			svc.WithBlobStore(blob.NewMem())
			eng := engine.New(memstore.New().(store.HistoryAware))
			u, err := svc.Register("admin", "Admin", "password123", "")
			if err != nil {
				t.Fatal(err)
			}
			band, err := svc.CreateBand(u, "Demo Band")
			if err != nil {
				t.Fatal(err)
			}
			song, err := svc.CreateSong(u, band.ID, "The Open Road", "")
			if err != nil {
				t.Fatal(err)
			}
			chart, err := svc.CreateTextChart(u, band.ID, song.ID, testutil.FreezeChart)
			if err != nil {
				t.Fatal(err)
			}
			before, err := svc.ChartRenderForFile(chart.ID)
			if err != nil {
				t.Fatal(err)
			}
			var tg chartpdf.Anchor
			for _, a := range before.Anchors {
				if a.Text == testutil.FreezeTarget {
					tg = a
				}
			}
			if tg.Text == "" {
				t.Fatal("fixture: target line not in the render")
			}
			if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: u.ID, Layer: &domain.Layer{
				ID: "L1", FileID: chart.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
			}}); err != nil {
				t.Fatal(err)
			}
			stored := map[string]domain.Object{}
			pop := map[string]string{} // uuid → population
			for _, p := range []string{"linked", "oldstyle"} {
				marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
				for _, shape := range testutil.FreezeShapes {
					for _, o := range marks[shape] {
						o.OwnerID = u.ID
						o.UUID = p + "-" + o.UUID
						if o.JumpTo != "" {
							o.JumpTo = p + "-" + o.JumpTo
						}
						if p == "linked" {
							o = chartpdf.LinkMark(o, before)
							if o.Link == nil || o.Link.State != domain.LinkLinked {
								t.Fatalf("fixture: %s did not link (the cell would prove nothing)", o.UUID)
							}
						} else {
							o.Anchor = &domain.SourceAnchor{RunText: tg.Text, Occurrence: 1, CharStart: 0, CharEnd: 9}
							o.PointsRenderHash = chart.BlobHash
						}
						if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: u.ID, Object: &o}); err != nil {
							t.Fatal(err)
						}
						stored[o.UUID], pop[o.UUID] = o, p
					}
				}
			}
			saved, err := svc.SaveChartSource(u, band.ID, song.ID, chart.ID, chart.Revision, testutil.FreezeEdits[edit])
			if err != nil {
				t.Fatal(err)
			}
			if saved.BlobHash == chart.BlobHash {
				t.Fatal("fixture: the edit did not change the render")
			}
			after, err := svc.ChartRenderForFile(chart.ID)
			if err != nil {
				t.Fatal(err)
			}
			sl, err := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
				t.Fatal(err)
			}
			png := tinyPNG(t, 40, 56)
			capt := &docCapture{png: png}
			b := &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 2, png: png}, overlays: capt,
				bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
			if _, _, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil, ""); err != nil {
				t.Fatalf("bake: %v", err)
			}
			baked := map[string]docObject{}
			for _, d := range capt.docs {
				for _, o := range d.Objects {
					baked[o.UUID] = o
				}
			}
			for uuid, st := range stored {
				uuid, st := uuid, st
				t.Run(pop[uuid]+"/"+uuid, func(t *testing.T) {
					g, ok := baked[uuid]
					if !ok {
						t.Fatalf("%s was not baked", uuid)
					}
					pts := make([]domain.Point, len(g.Points))
					for i, p := range g.Points {
						pts[i] = domain.Point{X: p.X, Y: p.Y, Pressure: p.Pressure}
					}
					want := st
					if pop[uuid] == "linked" {
						want = chartpdf.ProjectMark(st, after) // P33: exactly what serve shows
						if want.Link.State == domain.LinkLost || math.Abs(want.Points[0].Y-st.Points[0].Y) < 1e-6 {
							t.Fatalf("fixture: the projection did not move %s (state %s) — the cell would prove nothing", uuid, want.Link.State)
						}
					}
					if g.Page != want.Page || !reflect.DeepEqual(pts, want.Points) {
						t.Errorf("%s: baked page %d points %v, want page %d points %v", uuid, g.Page, pts, want.Page, want.Points)
					}
				})
			}
		})
	}
}

// TestSnapshotToDoc_ProjectsLinkedMarks: the doc builder's own contract, without a bake — a stage-2 link is
// projected, an old-style anchor and an unlinked mark are not, and a nil render projects nothing.
func TestSnapshotToDoc_ProjectsLinkedMarks(t *testing.T) {
	src := "# Song\n\n## Verse\nC       G\nthe one true lyric line\n\n## Chorus\nF       C\nanother sung line here\n"
	moved := "# Song\n\n## Intro\nG  D\n\n## Verse\nC       G\nthe one true lyric line\n\n## Chorus\nF       C\nanother sung line here\n"
	render := func(s string) ([]chartpdf.Anchor, string) {
		_, a, err := chartpdf.RenderWithAnchors(s)
		if err != nil {
			t.Fatal(err)
		}
		return a, chartpdf.SourceHash(s)
	}
	n := 0
	newID := func() string { n++; return string(rune('a' + n)) }
	a0, h0 := render(src)
	li := chartpdf.NewLineIdentity(src, newID)
	r0 := chartpdf.ChartRender{Source: src, Lines: li, Anchors: a0, Hash: h0}
	a1, h1 := render(moved)
	r1 := chartpdf.ChartRender{Source: moved, Anchors: a1, Hash: h1,
		Lines: chartpdf.CarryLineIdentity(li, src, moved, newID, chartpdf.CarryOptions{FromRender: h0, ToRender: h1})}
	var run chartpdf.Anchor
	for _, a := range a0 {
		if a.Text == "the one true lyric line" {
			run = a
		}
	}
	box := []domain.Point{{X: run.X0, Y: run.Y0}, {X: run.X1, Y: run.Y1}}
	linked := chartpdf.LinkMark(domain.Object{UUID: "linked", LayerID: "L1", Type: domain.TypeHighlight, Points: box}, r0)
	old := domain.Object{UUID: "old", LayerID: "L1", Type: domain.TypeHighlight, Points: box,
		Anchor: &domain.SourceAnchor{RunText: run.Text, Occurrence: 1}, PointsRenderHash: "old-render"}
	snap := domain.Snapshot{Layers: []domain.Layer{{ID: "L1", FileID: "f1"}}, Objects: []domain.Object{linked, old}}

	doc := snapshotToDoc(snap, "f1", &r1)
	byID := map[string]docObject{}
	for _, o := range doc.Objects {
		byID[o.UUID] = o
	}
	if byID["linked"].Points[0].Y <= box[0].Y {
		t.Errorf("the linked mark did not follow its line down (y %.4f, was %.4f)", byID["linked"].Points[0].Y, box[0].Y)
	}
	if byID["old"].Points[0].Y != box[0].Y || byID["old"].Points[1].X != box[1].X {
		t.Errorf("an old-style anchored mark moved at bake: %v", byID["old"].Points)
	}
	none := snapshotToDoc(snap, "f1", nil)
	for _, o := range none.Objects {
		if o.Points[0].Y != box[0].Y {
			t.Errorf("%s moved with no current render (P32)", o.UUID)
		}
	}
}

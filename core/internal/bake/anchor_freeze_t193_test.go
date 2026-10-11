package bake

import (
	"context"
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

// TestAnchorFreezeBake/<edit>/<shape> (T193, not gated): the bake half of the freeze cell. Marks linked
// before the freeze sit on a generated chart; the chart source is then edited (new blob hash, so every mark
// is stale). Frozen, a real Bake hands the overlay renderer each mark's stored Points and Page exactly.
func TestAnchorFreezeBake(t *testing.T) {
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
			_, anchors, err := chartpdf.RenderWithAnchors(testutil.FreezeChart)
			if err != nil {
				t.Fatal(err)
			}
			var tg chartpdf.Anchor
			for _, a := range anchors {
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
			marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
			stored := map[string]domain.Object{}
			for _, shape := range testutil.FreezeShapes {
				for _, o := range marks[shape] {
					o.OwnerID = u.ID
					o = chartpdf.AnchorObject(o, anchors, chart.BlobHash) // the OLD create path: pre-freeze data
					if o.Anchor == nil {
						t.Fatalf("fixture: %s did not link (the cell would prove nothing)", o.UUID)
					}
					if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: u.ID, Object: &o}); err != nil {
						t.Fatal(err)
					}
					stored[o.UUID] = o
				}
			}
			saved, err := svc.SaveChartSource(u, band.ID, song.ID, chart.ID, chart.Revision, testutil.FreezeEdits[edit])
			if err != nil {
				t.Fatal(err)
			}
			if saved.BlobHash == chart.BlobHash {
				t.Fatal("fixture: the edit did not change the render")
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
			for _, shape := range testutil.FreezeShapes {
				shape := shape
				t.Run(shape, func(t *testing.T) {
					for _, m := range marks[shape] {
						st := stored[m.UUID]
						g, ok := baked[m.UUID]
						if !ok {
							t.Fatalf("%s was not baked", m.UUID)
						}
						pts := make([]domain.Point, len(g.Points))
						for i, p := range g.Points {
							pts[i] = domain.Point{X: p.X, Y: p.Y, Pressure: p.Pressure}
						}
						if g.Page != st.Page || !reflect.DeepEqual(pts, st.Points) {
							t.Errorf("%s: baked page %d points %v, stored page %d points %v — a frozen mark must bake where it is stored",
								m.UUID, g.Page, pts, st.Page, st.Points)
						}
					}
				})
			}
		})
	}
}

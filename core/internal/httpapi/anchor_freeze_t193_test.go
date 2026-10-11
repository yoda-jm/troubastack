package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

// T193 (anchoring stage 0) — the serve and create halves of the freeze, through the real annotation routes
// (GET /annotations, POST /annotations/import, mounted on a mux with the caller injected in place of the
// session check) and the real realtime create-time anchorer (what sync/apply.go KindCreate calls).
//
// NOT gated: these assert the freeze itself, so they run in CI and must stay green while it is on.

type freezeFixture struct {
	svc    *app.Service
	eng    *engine.Engine
	admin  app.User
	band   string
	song   string
	chart  app.SongFile
	target chartpdf.Anchor
	mux    *http.ServeMux
}

func newFreezeFixture(t *testing.T) *freezeFixture {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, err := svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Demo Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "The Open Road", "Demo")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := svc.CreateTextChart(admin, band.ID, song.ID, testutil.FreezeChart)
	if err != nil {
		t.Fatal(err)
	}
	_, anchors, err := chartpdf.RenderWithAnchors(testutil.FreezeChart)
	if err != nil {
		t.Fatal(err)
	}
	var target chartpdf.Anchor
	for _, a := range anchors {
		if a.Text == testutil.FreezeTarget {
			target = a
		}
	}
	if target.Text == "" {
		t.Fatal("fixture: target line not in the render")
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: chart.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	NewAnnotationsAPI(svc, eng).Mount(mux, func(h authedHandler) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) { h(w, r, admin) }
	})
	return &freezeFixture{svc: svc, eng: eng, admin: admin, band: band.ID, song: song.ID, chart: chart, target: target, mux: mux}
}

func (f *freezeFixture) path(suffix string) string {
	return "/api/bands/" + f.band + "/songs/" + f.song + "/annotations" + suffix
}

// served GETs the annotations exactly as the editor does, keyed by uuid.
func (f *freezeFixture) served(t *testing.T) map[string]domain.Object {
	t.Helper()
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, f.path(""), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET annotations: %d %s", rec.Code, rec.Body)
	}
	var doc annotationsJSON
	if err := json.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	out := map[string]domain.Object{}
	for _, oj := range doc.Objects {
		out[oj.UUID] = objectFromJSON(oj)
	}
	return out
}

// TestAnchorFreezeServe/<edit>/<shape>: a mark linked BEFORE the freeze (the stored population: an Anchor
// and the render hash of the chart it was drawn on), then a source edit that re-renders the chart (new
// blob hash, so every such mark is stale). Frozen, GET serves each mark's stored Points and Page exactly,
// and its stored Anchor is still there (the data is kept for stage 4).
func TestAnchorFreezeServe(t *testing.T) {
	for _, edit := range []string{"insert_line_above", "size_change"} {
		edit := edit
		t.Run(edit, func(t *testing.T) {
			f := newFreezeFixture(t)
			_, anchors, err := chartpdf.RenderWithAnchors(testutil.FreezeChart)
			if err != nil {
				t.Fatal(err)
			}
			tg := f.target
			marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
			for _, shape := range testutil.FreezeShapes {
				for _, o := range marks[shape] {
					o.OwnerID = f.admin.ID
					o = chartpdf.AnchorObject(o, anchors, f.chart.BlobHash) // the OLD create path: pre-freeze data
					if o.Anchor == nil {
						t.Fatalf("fixture: %s did not link (the cell would prove nothing)", o.UUID)
					}
					if _, err := f.eng.Apply(f.song, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: f.admin.ID, Object: &o}); err != nil {
						t.Fatal(err)
					}
				}
			}
			saved, err := f.svc.SaveChartSource(f.admin, f.band, f.song, f.chart.ID, f.chart.Revision, testutil.FreezeEdits[edit])
			if err != nil {
				t.Fatal(err)
			}
			if saved.BlobHash == f.chart.BlobHash {
				t.Fatal("fixture: the edit did not change the render")
			}
			head, err := f.eng.Head(f.song)
			if err != nil {
				t.Fatal(err)
			}
			got := f.served(t)
			stored := map[string]domain.Object{}
			for _, o := range head.LiveObjects() {
				stored[o.UUID] = o
			}
			for _, shape := range testutil.FreezeShapes {
				shape := shape
				t.Run(shape, func(t *testing.T) {
					for _, m := range marks[shape] {
						st, g := stored[m.UUID], got[m.UUID]
						if g.Page != st.Page || !reflect.DeepEqual(g.Points, st.Points) {
							t.Errorf("%s: served page %d points %v, stored page %d points %v — a frozen mark must be served where it is stored",
								m.UUID, g.Page, g.Points, st.Page, st.Points)
						}
						if !reflect.DeepEqual(g.Anchor, st.Anchor) || g.PointsRenderHash != st.PointsRenderHash {
							t.Errorf("%s: served link %+v/%q, stored %+v/%q — the stored data must be served as is",
								m.UUID, g.Anchor, g.PointsRenderHash, st.Anchor, st.PointsRenderHash)
						}
					}
				})
			}
		})
	}
}

// TestAnchorFreezeCreateNoAnchor/<path>/<shape>: a mark created while frozen, ON the target line of a
// generated chart (where the old mechanism would link it), is stored with no Anchor and no
// PointsRenderHash — through the realtime create-time anchorer and through the HTTP import. An imported
// mark that already carries a link (a band folder that stored one) keeps it unchanged.
func TestAnchorFreezeCreateNoAnchor(t *testing.T) {
	t.Run("realtime", func(t *testing.T) {
		f := newFreezeFixture(t)
		tg := f.target
		marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
		anc := newChartAnchorer(f.svc, f.eng)
		for _, shape := range testutil.FreezeShapes {
			shape := shape
			t.Run(shape, func(t *testing.T) {
				for _, m := range marks[shape] {
					out := anc.AnchorMark(f.song, m)
					if out.Anchor != nil || out.PointsRenderHash != "" {
						t.Errorf("%s: created with link %+v / hash %q while frozen, want none", m.UUID, out.Anchor, out.PointsRenderHash)
					}
					if !reflect.DeepEqual(out.Points, m.Points) || out.Page != m.Page {
						t.Errorf("%s: the create moved the mark", m.UUID)
					}
				}
			})
		}
	})

	t.Run("import", func(t *testing.T) {
		f := newFreezeFixture(t)
		tg := f.target
		marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
		kept := &domain.SourceAnchor{RunText: testutil.FreezeTarget, Occurrence: 1, CharStart: 0, CharEnd: 5}
		// The layer rides in the payload, as a band folder's does: import anchoring resolves a mark's chart
		// through the payload's layers.
		layer, ok := f.eng.Layer(f.song, "L1")
		if !ok {
			t.Fatal("fixture: no layer")
		}
		in := annotationsJSON{Layers: []layerJSON{layerToJSON(layer)}, Objects: []objectJSON{}}
		for _, shape := range testutil.FreezeShapes {
			for _, m := range marks[shape] {
				m.OwnerID = f.admin.ID
				in.Objects = append(in.Objects, objectToJSON(m))
			}
		}
		pre := domain.Object{UUID: "pre-linked", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page, Version: 1,
			OwnerID: f.admin.ID, Points: []domain.Point{{X: tg.X0, Y: tg.Y0}, {X: tg.X1, Y: tg.Y1}},
			Anchor: kept, PointsRenderHash: "old-render"}
		in.Objects = append(in.Objects, objectToJSON(pre))
		body, _ := json.Marshal(in)
		rec := httptest.NewRecorder()
		f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, f.path("/import"), bytes.NewReader(body)))
		if rec.Code != http.StatusOK {
			t.Fatalf("import: %d %s", rec.Code, rec.Body)
		}
		head, err := f.eng.Head(f.song)
		if err != nil {
			t.Fatal(err)
		}
		stored := map[string]domain.Object{}
		for _, o := range head.LiveObjects() {
			stored[o.UUID] = o
		}
		for _, shape := range testutil.FreezeShapes {
			shape := shape
			t.Run(shape, func(t *testing.T) {
				for _, m := range marks[shape] {
					st, ok := stored[m.UUID]
					if !ok {
						t.Fatalf("%s not imported", m.UUID)
					}
					if st.Anchor != nil || st.PointsRenderHash != "" {
						t.Errorf("%s: imported with link %+v / hash %q while frozen, want none", m.UUID, st.Anchor, st.PointsRenderHash)
					}
				}
			})
		}
		t.Run("pre_linked_kept", func(t *testing.T) {
			st := stored["pre-linked"]
			if !reflect.DeepEqual(st.Anchor, kept) || st.PointsRenderHash != "old-render" {
				t.Errorf("an imported link was not kept as is: %+v / %q", st.Anchor, st.PointsRenderHash)
			}
		})
	})
}

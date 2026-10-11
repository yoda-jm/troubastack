package httpapi

import (
	"bytes"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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

// T195 (anchoring stage 2) — the serve, create, import and move halves through the real annotation routes
// (GET /annotations, POST /annotations/import, mounted on a mux with the caller injected in place of the
// session check) and the real realtime anchorer (what sync/apply.go calls). It replaces T193's freeze
// tests (TestAnchorFreezeServe, TestAnchorFreezeCreateNoAnchor), which pinned "nothing is ever linked or
// projected": stage 2 links and projects new-style marks, and keeps only OLD-STYLE anchors frozen — which
// the serve test below still pins, shape by shape. It also replaces T145's anchorer tests, which pinned
// the old text+occurrence anchor.

type stage2Fixture struct {
	svc    *app.Service
	eng    *engine.Engine
	admin  app.User
	band   string
	song   string
	chart  app.SongFile
	target chartpdf.Anchor
	mux    *http.ServeMux
}

func newStage2Fixture(t *testing.T) *stage2Fixture {
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
	return &stage2Fixture{svc: svc, eng: eng, admin: admin, band: band.ID, song: song.ID, chart: chart, target: target, mux: mux}
}

func (f *stage2Fixture) path(suffix string) string {
	return "/api/bands/" + f.band + "/songs/" + f.song + "/annotations" + suffix
}

// served GETs the annotations exactly as the editor does, keyed by uuid, and returns the raw body too.
func (f *stage2Fixture) served(t *testing.T) (map[string]domain.Object, string) {
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
	return out, rec.Body.String()
}

func (f *stage2Fixture) create(t *testing.T, o domain.Object) {
	t.Helper()
	if _, err := f.eng.Apply(f.song, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: f.admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
}

func (f *stage2Fixture) render(t *testing.T) chartpdf.ChartRender {
	t.Helper()
	r, err := f.svc.ChartRenderForFile(f.chart.ID)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (f *stage2Fixture) save(t *testing.T, src string) app.SongFile {
	t.Helper()
	cur, _, err := f.svc.ChartSource(f.admin, f.band, f.song, f.chart.ID)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := f.svc.SaveChartSource(f.admin, f.band, f.song, f.chart.ID, cur.Revision, src)
	if err != nil {
		t.Fatal(err)
	}
	if saved.BlobHash == cur.BlobHash {
		t.Fatal("fixture: the edit did not change the render")
	}
	return saved
}

// TestAnchorStage2Serve/<edit>/<shape>: one mark of each population per shape — a stage-2 link made by
// the realtime anchorer, and an old-style anchor (the T145 data the freeze kept) — then a source edit that
// re-renders the chart. GET serves the linked mark where the projection puts it (it moved with its line,
// and its state rides in `link`), and the old-style mark exactly where it is stored, its anchor intact.
func TestAnchorStage2Serve(t *testing.T) {
	for _, edit := range []string{"insert_line_above", "size_change"} {
		edit := edit
		t.Run(edit, func(t *testing.T) {
			f := newStage2Fixture(t)
			tg := f.target
			anc := newChartAnchorer(f.svc, f.eng)
			stored := map[string]domain.Object{}
			for _, pop := range []string{"linked", "oldstyle"} {
				marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
				for _, shape := range testutil.FreezeShapes {
					for _, o := range marks[shape] {
						o.UUID, o.OwnerID = pop+"-"+o.UUID, f.admin.ID
						if o.JumpTo != "" {
							o.JumpTo = pop + "-" + o.JumpTo
						}
						if pop == "linked" {
							o = anc.AnchorMark(f.song, o)
							if o.Link == nil || o.Link.State != domain.LinkLinked {
								t.Fatalf("fixture: %s did not link (the cell would prove nothing): %+v", o.UUID, o.Link)
							}
						} else {
							o.Anchor = &domain.SourceAnchor{RunText: tg.Text, Occurrence: 1, CharStart: 0, CharEnd: 9}
							o.PointsRenderHash = f.chart.BlobHash
						}
						f.create(t, o)
						stored[o.UUID] = o
					}
				}
			}
			saved := f.save(t, testutil.FreezeEdits[edit])
			cur := f.render(t)
			got, body := f.served(t)
			if !strings.Contains(body, `"link":{"state":"linked"`) {
				t.Errorf("the served payload carries no link state: %.300s", body)
			}
			for uuid, st := range stored {
				uuid, st := uuid, st
				t.Run(uuid, func(t *testing.T) {
					g := got[uuid]
					if strings.HasPrefix(uuid, "oldstyle-") {
						// Its coordinates and anchor exactly as stored; its hash is the CURRENT render's (F1: the
						// stored coordinates are what is drawn on it, and a move echoes this hash back).
						if g.Page != st.Page || !reflect.DeepEqual(g.Points, st.Points) || !reflect.DeepEqual(g.Anchor, st.Anchor) ||
							g.Link != nil || g.PointsRenderHash != saved.BlobHash {
							t.Errorf("an old-style mark was not served as stored on the current render: %+v, stored %+v", g, st)
						}
						return
					}
					want := chartpdf.ProjectMark(st, cur)
					if g.Page != want.Page || !reflect.DeepEqual(g.Points, want.Points) {
						t.Errorf("served page %d points %v, want the projection's page %d points %v", g.Page, g.Points, want.Page, want.Points)
					}
					if g.Link == nil || g.Link.State != domain.LinkLinked || g.PointsRenderHash != saved.BlobHash {
						t.Errorf("served link %+v / hash %q, want linked on the current render %q", g.Link, g.PointsRenderHash, saved.BlobHash)
					}
					if math.Abs(g.Points[0].Y-st.Points[0].Y) < 1e-4 {
						t.Errorf("the linked mark did not move with its line (y %.4f, stored %.4f)", g.Points[0].Y, st.Points[0].Y)
					}
				})
			}
			// P33: the realtime join snapshot is served by the same projection.
			head, err := f.eng.Head(f.song)
			if err != nil {
				t.Fatal(err)
			}
			ws := anc.ServeSnapshot(f.song, head)
			for _, o := range ws.LiveObjects() {
				if g := got[o.UUID]; !reflect.DeepEqual(g.Points, o.Points) || g.Page != o.Page {
					t.Errorf("%s: the join snapshot serves %v, GET serves %v", o.UUID, o.Points, g.Points)
				}
			}
		})
	}
}

// TestAnchorStage2CreateAndImport/<path>/<shape>: a mark drawn on the target line of a generated chart is
// linked to THAT line by the server — through the realtime anchorer and through the HTTP import (P31). An
// imported mark that already carries a link, of either style, keeps it unchanged (P30).
func TestAnchorStage2CreateAndImport(t *testing.T) {
	t.Run("realtime", func(t *testing.T) {
		f := newStage2Fixture(t)
		tg := f.target
		r := f.render(t)
		want := r.Lines.IDs[tg.SrcLine]
		marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
		anc := newChartAnchorer(f.svc, f.eng)
		for _, shape := range testutil.FreezeShapes {
			shape := shape
			t.Run(shape, func(t *testing.T) {
				for _, m := range marks[shape] {
					out := anc.AnchorMark(f.song, m)
					if out.Link == nil || out.Link.State != domain.LinkLinked || out.Link.Line != want || out.Anchor != nil {
						t.Errorf("%s: created with link %+v anchor %+v, want linked to the target line %s", m.UUID, out.Link, out.Anchor, want)
					}
					if out.PointsRenderHash != f.chart.BlobHash {
						t.Errorf("%s: render hash %q, want the chart's %q", m.UUID, out.PointsRenderHash, f.chart.BlobHash)
					}
					if !reflect.DeepEqual(out.Points, m.Points) || out.Page != m.Page {
						t.Errorf("%s: linking moved the mark", m.UUID)
					}
				}
			})
		}
		// Best-effort degradation: an unknown layer, or no layer, leaves the mark unlinked.
		for _, o := range []domain.Object{{UUID: "x", LayerID: "nope", Points: marks["rect"][0].Points}, {UUID: "y", Points: marks["rect"][0].Points}} {
			if out := anc.AnchorMark(f.song, o); out.Link != nil {
				t.Errorf("%s: linked with no generated chart behind it: %+v", o.UUID, out.Link)
			}
		}
	})

	t.Run("import", func(t *testing.T) {
		f := newStage2Fixture(t)
		tg := f.target
		r := f.render(t)
		want := r.Lines.IDs[tg.SrcLine]
		marks := testutil.FreezeMarks("L1", tg.Page, tg.X0, tg.Y0, tg.X1, tg.Y1)
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
		box := []domain.Point{{X: tg.X0, Y: tg.Y0}, {X: tg.X1, Y: tg.Y1}}
		oldAnchor := &domain.SourceAnchor{RunText: testutil.FreezeTarget, Occurrence: 1, CharStart: 0, CharEnd: 5}
		preOld := domain.Object{UUID: "pre-old", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page, Version: 1,
			OwnerID: f.admin.ID, Points: box, Anchor: oldAnchor, PointsRenderHash: "old-render"}
		newLink := &domain.MarkLink{State: domain.LinkCheck, Line: "a-line-from-the-folder", XRef: "start", LH: 0.02, Render: "old-render"}
		preNew := domain.Object{UUID: "pre-new", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page, Version: 1,
			OwnerID: f.admin.ID, Points: box, Link: newLink, PointsRenderHash: "old-render"}
		in.Objects = append(in.Objects, objectToJSON(preOld), objectToJSON(preNew))
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
					if st.Link == nil || st.Link.State != domain.LinkLinked || st.Link.Line != want {
						t.Errorf("%s: imported with link %+v, want linked to the target line (P31)", m.UUID, st.Link)
					}
				}
			})
		}
		t.Run("pre_linked_kept", func(t *testing.T) {
			if st := stored["pre-old"]; !reflect.DeepEqual(st.Anchor, oldAnchor) || st.Link != nil || st.PointsRenderHash != "old-render" {
				t.Errorf("an imported old-style anchor was not kept as is: %+v / %+v / %q", st.Anchor, st.Link, st.PointsRenderHash)
			}
			if st := stored["pre-new"]; !reflect.DeepEqual(st.Link, newLink) || st.PointsRenderHash != "old-render" {
				t.Errorf("an imported stage-2 link was not kept as is: %+v / %q", st.Link, st.PointsRenderHash)
			}
		})
	})
}

// TestRelinkMark_StaleRenderKeepsTheLink is P27 at the seam that knows the real render hash: a mark
// linked on render R1; the chart is saved (R2). A move sent from a tab still showing R1 keeps the stored
// line (the server never re-links against a render the user did not see) and shifts its offset by the
// move, so the next serve puts the mark where it was dropped, relative to its line. The same move sent
// from a tab showing R2 re-links from the new position. Both go through the real chartAnchorer.
func TestRelinkMark_StaleRenderKeepsTheLink(t *testing.T) {
	f := newStage2Fixture(t)
	tg := f.target
	anc := newChartAnchorer(f.svc, f.eng)
	h := tg.Y1 - tg.Y0
	mark := anc.AnchorMark(f.song, domain.Object{UUID: "m", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page,
		OwnerID: f.admin.ID, Points: []domain.Point{{X: tg.X0, Y: tg.Y0 - 0.2*h}, {X: tg.X0 + 0.1, Y: tg.Y1 + 0.2*h}}})
	if mark.Link == nil || mark.Link.State != domain.LinkLinked {
		t.Fatalf("fixture: the mark did not link: %+v", mark.Link)
	}
	f.create(t, mark)
	r1 := mark.PointsRenderHash
	saved := f.save(t, testutil.FreezeEdits["insert_line_above"])

	// The stale tab drags the mark down by a quarter line, in R1's coordinates.
	stale := mark.Clone()
	for i := range stale.Points {
		stale.Points[i].Y += 0.25 * h
	}
	stale.PointsRenderHash = r1
	stale.Link = &domain.MarkLink{State: domain.LinkLinked, Line: "forged"}
	out := anc.RelinkMark(f.song, stale, mark)
	if out.Link == nil || out.Link.Line != mark.Link.Line || out.Link.Render != r1 {
		t.Fatalf("a stale-view move stored link %+v; want the stored line on its own render", out.Link)
	}
	if out.Link.State != domain.LinkLinked {
		t.Errorf("a stale-view move on the link's own render is a known shift, not a guess: state %s", out.Link.State)
	}
	served := chartpdf.ProjectMark(out, f.render(t))
	before := chartpdf.ProjectMark(mark, f.render(t))
	if d := served.Points[0].Y - before.Points[0].Y; math.Abs(d-0.25*h) > 0.02*h {
		t.Errorf("the stale-view move landed %.4f below the unmoved projection, want a quarter line (%.4f)", d, 0.25*h)
	}

	// The same drag from a tab showing the current render re-links from the new position.
	fresh := before.Clone()
	cur := f.render(t)
	var other chartpdf.Anchor
	for _, a := range cur.Anchors {
		if a.Text == "Oh the open road" {
			other = a
		}
	}
	dy := other.Y0 - before.Points[0].Y - 0.2*(other.Y1-other.Y0)
	for i := range fresh.Points {
		fresh.Points[i].Y += dy
	}
	fresh.PointsRenderHash = saved.BlobHash
	out2 := anc.RelinkMark(f.song, fresh, mark)
	if out2.Link == nil || out2.Link.Line != cur.Lines.IDs[other.SrcLine] || out2.Link.Render != saved.BlobHash {
		t.Errorf("a current-view move stored link %+v; want a new link to the line it was dropped on", out2.Link)
	}
}

// F2 (T195 review): an imported stage-2 link is client-authored. A forged one — a real line id but a
// line-height of 5e-324, so the scale k is +Inf — used to put Inf on the wire: JSON encoding failed and
// GET /annotations returned an empty body for the WHOLE song. Now the import drops every link the server
// could not have written and re-links the mark by the rule (P31); other forgeries are covered too.
func TestAnchorStage2_ForgedImportedLinkIsRelinked(t *testing.T) {
	f := newStage2Fixture(t)
	tg := f.target
	r := f.render(t)
	line := r.Lines.IDs[tg.SrcLine]
	layer, _ := f.eng.Layer(f.song, "L1")
	box := []domain.Point{{X: tg.X0, Y: tg.Y0}, {X: tg.X1, Y: tg.Y1}}
	forged := map[string]*domain.MarkLink{
		"tiny-lh":     {State: domain.LinkLinked, Line: line, XRef: "start", LH: 5e-324, Render: "some-other-render"},
		"huge-dy":     {State: domain.LinkLinked, Line: line, XRef: "start", LH: 0.02, DY: 1e300, Render: "some-other-render"},
		"bad-state":   {State: "pinned", Line: line, XRef: "start", LH: 0.02, Render: "some-other-render"},
		"no-line":     {State: domain.LinkLinked, XRef: "start", LH: 0.02, Render: "some-other-render"},
		"char-no-ptr": {State: domain.LinkLinked, Line: line, XRef: "char", LH: 0.02, Render: "some-other-render"},
	}
	in := annotationsJSON{Layers: []layerJSON{layerToJSON(layer)}}
	for id, l := range forged {
		in.Objects = append(in.Objects, objectToJSON(domain.Object{UUID: id, LayerID: "L1", Type: domain.TypeRect, Page: tg.Page,
			Version: 1, OwnerID: f.admin.ID, Points: box, Link: l, PointsRenderHash: "some-other-render"}))
	}
	in.Objects = append(in.Objects, objectToJSON(domain.Object{UUID: "good", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page,
		Version: 1, OwnerID: f.admin.ID, Points: box}))
	body, _ := json.Marshal(in)
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, f.path("/import"), bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("import: %d %s", rec.Code, rec.Body)
	}
	f.save(t, testutil.FreezeEdits["insert_line_above"]) // stale every link, so serve projects
	rec = httptest.NewRecorder()
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, f.path(""), nil))
	var doc annotationsJSON
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &doc) != nil || len(doc.Objects) != len(forged)+1 {
		t.Fatalf("a forged imported link broke GET /annotations: %d, %d bytes", rec.Code, rec.Body.Len())
	}
	for _, oj := range doc.Objects {
		o := objectFromJSON(oj)
		if o.Link == nil || o.Link.State != domain.LinkLinked || o.Link.Line != line || !chartpdf.ValidLink(o.Link) {
			t.Errorf("%s: served link %+v, want a valid server-made link to the target line", o.UUID, o.Link)
		}
	}
}

// F2's backstop: even a link that reaches projection without passing the import (a store written by an
// older build) cannot put a non-finite number on the wire — the mark is served where it is, as lost.
func TestProjectSnapshot_NonFiniteLinkIsServedLost(t *testing.T) {
	f := newStage2Fixture(t)
	tg := f.target
	r := f.render(t)
	o := domain.Object{UUID: "x", LayerID: "L1", Type: domain.TypeRect, Page: tg.Page, Version: 1, OwnerID: f.admin.ID,
		Points:           []domain.Point{{X: tg.X0, Y: tg.Y0}, {X: tg.X1, Y: tg.Y1}},
		Link:             &domain.MarkLink{State: domain.LinkLinked, Line: r.Lines.IDs[tg.SrcLine], XRef: "start", LH: 5e-324, Render: "older"},
		PointsRenderHash: "older"}
	f.create(t, o)
	got, body := f.served(t)
	if len(body) == 0 || got["x"].Link == nil || got["x"].Link.State != domain.LinkLost || !reflect.DeepEqual(got["x"].Points, o.Points) {
		t.Fatalf("a non-finite projection was served as %+v (body %d bytes); want lost, at its stored points", got["x"], len(body))
	}
}

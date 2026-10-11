package app_test

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
)

// T195 review F3: SaveChartSource re-places the marks linked to the chart it saves (eager), so a link is
// never more than one save behind and never outlives LineIdentity's bounded history.

const eagerChart = "# A Long Test Chart\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\n\n## Notes\nnote 0\n"

type eagerWorld struct {
	st    stack
	admin app.User
	band  string
	song  string
	file  app.SongFile
}

func newEager(t *testing.T, ms app.MarkStore) *eagerWorld {
	t.Helper()
	st := newStack()
	if ms == nil {
		ms = st.eng
	}
	st.svc.WithAnnotationEngine(ms)
	admin, err := st.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, _ := st.svc.CreateBand(admin, "Demo Band")
	song, _ := st.svc.CreateSong(admin, band.ID, "A Test Song", "")
	f, err := st.svc.CreateTextChart(admin, band.ID, song.ID, eagerChart)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: f.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	return &eagerWorld{st: st, admin: admin, band: band.ID, song: song.ID, file: f}
}

func (w *eagerWorld) render(t *testing.T) chartpdf.ChartRender {
	t.Helper()
	r, err := w.st.svc.ChartRenderForFile(w.file.ID)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (w *eagerWorld) run(t *testing.T, text string) chartpdf.Anchor {
	t.Helper()
	for _, a := range w.render(t).Anchors {
		if a.Text == text {
			return a
		}
	}
	t.Fatalf("fixture: %q not drawn", text)
	return chartpdf.Anchor{}
}

func (w *eagerWorld) save(t *testing.T, edit func(string) string) {
	t.Helper()
	cur, src, err := w.st.svc.ChartSource(w.admin, w.band, w.song, w.file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.svc.SaveChartSource(w.admin, w.band, w.song, w.file.ID, cur.Revision, edit(src)); err != nil {
		t.Fatal(err)
	}
}

func (w *eagerWorld) stored(t *testing.T, uuid string) domain.Object {
	t.Helper()
	snap, _ := w.st.eng.Head(w.song)
	for _, o := range snap.LiveObjects() {
		if o.UUID == uuid {
			return o
		}
	}
	t.Fatalf("%s not stored", uuid)
	return domain.Object{}
}

func cwOf(a chartpdf.Anchor) float64 { return (a.X1 - a.X0) / float64(len([]rune(a.Text))) }

// The reviewer's 40-save vector, through the real save: an icon on "moon", untouched while the chart is
// saved 40 times (more than the 32 the history keeps), then a word inserted before "moon". It is STORED on
// the current render after every save, and lands on "moon", linked.
func TestSaveChartSource_RebasesAnUntouchedMarkThroughManySaves(t *testing.T) {
	w := newEager(t, nil)
	line := w.run(t, "beneath a paper moon")
	c := cwOf(line)
	o := chartpdf.LinkMark(domain.Object{UUID: "m", LayerID: "L1", Type: domain.TypeIcon, Page: line.Page, Version: 1, OwnerID: w.admin.ID,
		Points: []domain.Point{{X: line.X0 + 16*c, Y: line.Y0}, {X: line.X0 + 18*c, Y: line.Y1}}}, w.render(t)) // on "moon"
	if _, err := w.st.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: "m", AuthorID: w.admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 40; i++ {
		w.save(t, func(s string) string {
			return strings.Replace(s, fmt.Sprintf("note %d\n", i-1), fmt.Sprintf("note %d\n", i), 1)
		})
		if st := w.stored(t, "m"); st.Link.Render != w.render(t).Hash {
			t.Fatalf("save %d: the mark was not re-placed on the new render (link render %s)", i, st.Link.Render)
		}
	}
	w.save(t, func(s string) string {
		return strings.Replace(s, "beneath a paper moon", "beneath a big paper moon", 1)
	})
	nl := w.run(t, "beneath a big paper moon")
	st := w.stored(t, "m")
	served := chartpdf.ProjectMark(st, w.render(t))
	want := nl.X0 + 20*cwOf(nl)
	if st.Link.State != domain.LinkLinked || math.Abs(served.Points[0].X-want) > 0.25*cwOf(nl) {
		t.Errorf("after 41 saves: state %s, x %.4f, want linked at %.4f (on \"moon\")", st.Link.State, served.Points[0].X, want)
	}
	if st.Version < 42 {
		t.Errorf("version %d: every re-placing is a normal mutation (want ≥ 42)", st.Version)
	}
}

// A lost mark's state is persisted by the save (stage 3 reads it from the store), at its coordinates.
func TestSaveChartSource_PersistsLost(t *testing.T) {
	w := newEager(t, nil)
	line := w.run(t, "beneath a paper moon")
	o := chartpdf.LinkMark(domain.Object{UUID: "m", LayerID: "L1", Type: domain.TypeRect, Page: line.Page, Version: 1, OwnerID: w.admin.ID,
		Points: []domain.Point{{X: line.X0, Y: line.Y0}, {X: line.X1, Y: line.Y1}}}, w.render(t))
	if _, err := w.st.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: "m", AuthorID: w.admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
	w.save(t, func(s string) string { return strings.Replace(s, "beneath a paper moon\n", "", 1) })
	st := w.stored(t, "m")
	if st.Link.State != domain.LinkLost || st.Points[0] != o.Points[0] {
		t.Errorf("stored state %s points %v, want lost at %v", st.Link.State, st.Points, o.Points)
	}
}

// racingStore lets a "user" move the mark just before the save's first re-placing write lands, so that
// write loses the engine's LWW check. The re-placing must re-read and re-base the user's move, never
// overwrite it.
type racingStore struct {
	app.MarkStore
	once func()
}

func (r *racingStore) Apply(songID string, m domain.Mutation) (domain.Mutation, error) {
	if r.once != nil && m.Kind == domain.KindMove && strings.Contains(m.Summary, "re-place") {
		f := r.once
		r.once = nil
		f()
	}
	return r.MarkStore.Apply(songID, m)
}

func TestSaveChartSource_RebaseNeverOverwritesAConcurrentEdit(t *testing.T) {
	rs := &racingStore{}
	w := newEager(t, rs)
	rs.MarkStore = w.st.eng
	line := w.run(t, "beneath a paper moon")
	o := chartpdf.LinkMark(domain.Object{UUID: "m", LayerID: "L1", Type: domain.TypeRect, Page: line.Page, Version: 1, OwnerID: w.admin.ID,
		Points: []domain.Point{{X: line.X0, Y: line.Y0}, {X: line.X1, Y: line.Y1}}}, w.render(t))
	if _, err := w.st.eng.Apply(w.song, domain.Mutation{Kind: domain.KindCreate, UUID: "m", AuthorID: w.admin.ID, Object: &o}); err != nil {
		t.Fatal(err)
	}
	userColour := "#16a34a"
	rs.once = func() { // the concurrent edit: a restyle by another member, on the old version
		cur := w.stored(t, "m")
		edited := cur.Clone()
		edited.Style.Color = userColour
		edited.Version = cur.Version + 1
		if _, err := w.st.eng.Apply(w.song, domain.Mutation{Kind: domain.KindSetStyle, UUID: "m", AuthorID: w.admin.ID,
			Object: &edited, BaseVersion: cur.Version}); err != nil {
			t.Fatal(err)
		}
	}
	w.save(t, func(s string) string { return strings.Replace(s, "## Verse\n", "## Verse\nan added line\n", 1) })
	st := w.stored(t, "m")
	if st.Style.Color != userColour {
		t.Errorf("the concurrent restyle was overwritten by the save's re-placing (colour %s)", st.Style.Color)
	}
	if st.Link.Render != w.render(t).Hash {
		t.Errorf("the re-placing gave up after losing the race: link render %s", st.Link.Render)
	}
}

// The render cache's cost model (the reviewer's measurement, kept as a benchmark):
//
//	go test ./internal/app/ -run XXX -bench ChartRender
func BenchmarkChartRenderForFile(b *testing.B) {
	st := newStack()
	admin, _ := st.svc.Register("owner", "Owner", "password123", "")
	band, _ := st.svc.CreateBand(admin, "Demo Band")
	song, _ := st.svc.CreateSong(admin, band.ID, "A Test Song", "")
	var sb strings.Builder
	sb.WriteString("# Bench Chart\n\n")
	for s := 0; s < 6; s++ {
		fmt.Fprintf(&sb, "## Section %d\n", s)
		for i := 0; i < 8; i++ {
			fmt.Fprintf(&sb, "G       D        Em      C\nsome lyric words on line %d of section %d here\n", i, s)
		}
		sb.WriteString("\n")
	}
	f, err := st.svc.CreateTextChart(admin, band.ID, song.ID, sb.String())
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := st.svc.ChartRenderForFile(f.ID); err != nil {
			b.Fatal(err)
		}
	}
}

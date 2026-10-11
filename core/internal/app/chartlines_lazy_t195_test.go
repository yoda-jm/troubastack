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

// T195 re-review: a mark nobody touches while its chart is saved many times is projected LAZILY from the
// render it was linked on; the line-id history keeps every step such a link needs (LineIdentity.Compact at
// serve, a 512-step cap at save), so it still follows its character. No save writes a mark.

const lazyChart = "# A Long Test Chart\n\n## Verse\nthe kettle hums a quiet tune\nbeneath a paper moon\n\n## Notes\nnote 0\n"

type lazyWorld struct {
	st    stack
	admin app.User
	band  string
	song  string
	file  app.SongFile
}

func newLazyWorld(t *testing.T) *lazyWorld {
	t.Helper()
	st := newStack()
	admin, err := st.svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, _ := st.svc.CreateBand(admin, "Demo Band")
	song, _ := st.svc.CreateSong(admin, band.ID, "A Test Song", "")
	f, err := st.svc.CreateTextChart(admin, band.ID, song.ID, lazyChart)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: f.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	return &lazyWorld{st: st, admin: admin, band: band.ID, song: song.ID, file: f}
}

func (w *lazyWorld) render(t *testing.T) chartpdf.ChartRender {
	t.Helper()
	r, err := w.st.svc.ChartRenderForFile(w.file.ID)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func (w *lazyWorld) run(t *testing.T, text string) chartpdf.Anchor {
	t.Helper()
	for _, a := range w.render(t).Anchors {
		if a.Text == text {
			return a
		}
	}
	t.Fatalf("fixture: %q not drawn", text)
	return chartpdf.Anchor{}
}

func (w *lazyWorld) save(t *testing.T, edit func(string) string) {
	t.Helper()
	cur, src, err := w.st.svc.ChartSource(w.admin, w.band, w.song, w.file.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.svc.SaveChartSource(w.admin, w.band, w.song, w.file.ID, cur.Revision, edit(src)); err != nil {
		t.Fatal(err)
	}
}

func (w *lazyWorld) stored(t *testing.T, uuid string) domain.Object {
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

// The reviewer's 40-save vector, through the real save, LAZILY: an icon on "moon", untouched while the
// chart is saved 40 times (more than the 32 steps kept as recorded), then a word inserted before "moon".
// The save writes nothing to the mark; the serve-time compaction (with the mark's render kept) runs half
// way; the projection still lands on "moon", linked.
func TestSaveChartSource_UntouchedMarkFollowsLazilyThroughManySaves(t *testing.T) {
	w := newLazyWorld(t)
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
		if i == 36 { // what a GET does: compact the history down to what the links reference
			if err := w.st.svc.CompactChartLines(w.file.ID, map[string]bool{o.Link.Render: true}); err != nil {
				t.Fatal(err)
			}
			if n := len(w.render(t).Lines.History); n > 33 {
				t.Fatalf("after compaction the history holds %d steps, want ≤ 32 recent + 1 for the one referenced render", n)
			}
		}
	}
	w.save(t, func(s string) string {
		return strings.Replace(s, "beneath a paper moon", "beneath a big paper moon", 1)
	})
	nl := w.run(t, "beneath a big paper moon")
	st := w.stored(t, "m")
	if st.Version != 1 || st.Link.Render != o.Link.Render {
		t.Errorf("a save wrote the mark (version %d, link render changed: %v) — projection is lazy", st.Version, st.Link.Render != o.Link.Render)
	}
	served := chartpdf.ProjectMark(st, w.render(t))
	want := nl.X0 + 20*cwOf(nl)
	if served.Link.State != domain.LinkLinked || math.Abs(served.Points[0].X-want) > 0.25*cwOf(nl) {
		t.Errorf("after 41 saves: state %s, x %.4f, want linked at %.4f (on \"moon\")", served.Link.State, served.Points[0].X, want)
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

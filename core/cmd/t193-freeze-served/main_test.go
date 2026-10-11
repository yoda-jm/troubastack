package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/filerepo"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/filestore"
	"troubastack/core/internal/testutil"
)

// edited inserts a line above the target AND rewrites the first lyric line, so a mark linked to that line
// is lost (its run is gone: served at its stored coordinates, never listed).
var edited = strings.Replace(testutil.FreezeEdits["insert_line_above"],
	"Pack a little light for the road ahead", "Pack a little lamp for the long way home", 1)

type fixture struct {
	dir      string
	song     string
	newHash  string
	anchors  []chartpdf.Anchor // the edited render
	expected domain.Object     // the stale mark as served before the freeze
}

func openEngine(t *testing.T, dir string) *engine.Engine {
	t.Helper()
	return engine.New(filestore.New(dir).(store.HistoryAware))
}

// newDataDir builds a real file-backed data dir: a generated chart with four marks, then a source edit.
func newDataDir(t *testing.T) fixture {
	t.Helper()
	dir := t.TempDir()
	repo, err := filerepo.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	blobs, err := blob.NewFile(filepath.Join(dir, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	svc := app.NewService(repo).WithBlobStore(blobs)
	eng := openEngine(t, dir)
	u, err := svc.Register("owner", "Owner", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Demo Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "The Open Road", "Demo")
	if err != nil {
		t.Fatal(err)
	}
	chart, err := svc.CreateTextChart(u, band.ID, song.ID, testutil.FreezeChart)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(m domain.Mutation) {
		t.Helper()
		if _, err := eng.Apply(song.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	apply(domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: u.ID, Layer: &domain.Layer{
		ID: "L1", FileID: chart.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}})
	_, oldAnchors, err := chartpdf.RenderWithAnchors(testutil.FreezeChart)
	if err != nil {
		t.Fatal(err)
	}
	line := func(anchors []chartpdf.Anchor, prefix string) chartpdf.Anchor {
		for _, a := range anchors {
			if strings.HasPrefix(a.Text, prefix) {
				return a
			}
		}
		t.Fatalf("fixture: no line %q", prefix)
		return chartpdf.Anchor{}
	}
	box := func(uuid string, a chartpdf.Anchor) domain.Object {
		return domain.Object{UUID: uuid, LayerID: "L1", Type: domain.TypeRect, Page: a.Page, Version: 1, OwnerID: u.ID,
			Points: []domain.Point{{X: a.X0 + 0.01, Y: a.Y0}, {X: a.X0 + 0.1, Y: a.Y1}}, Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.003}}
	}
	create := func(o domain.Object) {
		apply(domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: u.ID, Object: &o})
	}

	stale := chartpdf.AnchorObject(box("m-stale", line(oldAnchors, "Leave the porch")), oldAnchors, chart.BlobHash)
	lost := chartpdf.AnchorObject(box("m-lost", line(oldAnchors, "Pack a little light")), oldAnchors, chart.BlobHash)
	if stale.Anchor == nil || lost.Anchor == nil {
		t.Fatal("fixture: marks did not link")
	}
	create(stale)
	create(lost)
	create(box("m-unlinked", line(oldAnchors, "Leave the porch")))

	saved, err := svc.SaveChartSource(u, band.ID, song.ID, chart.ID, chart.Revision, edited)
	if err != nil {
		t.Fatal(err)
	}
	_, newAnchors, err := chartpdf.RenderWithAnchors(edited)
	if err != nil {
		t.Fatal(err)
	}
	current := chartpdf.AnchorObject(box("m-current", line(newAnchors, "Leave the porch")), newAnchors, saved.BlobHash)
	if current.Anchor == nil {
		t.Fatal("fixture: current mark did not link")
	}
	create(current)

	want, _ := chartpdf.Reproject([]domain.Object{stale}, newAnchors, saved.BlobHash)
	return fixture{dir: dir, song: song.ID, newHash: saved.BlobHash, anchors: newAnchors, expected: want[0]}
}

func head(t *testing.T, dir, song string) domain.Snapshot {
	t.Helper()
	snap, err := openEngine(t, dir).Head(song)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func byID(snap domain.Snapshot) map[string]domain.Object {
	m := map[string]domain.Object{}
	for _, o := range snap.LiveObjects() {
		m[o.UUID] = o
	}
	return m
}

func TestFreezeServed(t *testing.T) {
	f := newDataDir(t)
	before := head(t, f.dir, f.song)

	// 1. Dry run (the default): exactly the stale mark is listed, nothing is written.
	var out bytes.Buffer
	if err := run([]string{"-data", f.dir}, &out); err != nil {
		t.Fatal(err)
	}
	t.Logf("dry run:\n%s", out.String())
	if !strings.Contains(out.String(), "DRY RUN") {
		t.Error("the default is not a dry run")
	}
	for _, id := range []string{"m-lost", "m-unlinked", "m-current"} {
		if strings.Contains(out.String(), id) {
			t.Errorf("%s is listed, but it is served at its stored position", id)
		}
	}
	if !strings.Contains(out.String(), "m-stale") || !strings.Contains(out.String(), "4 mark(s) on generated charts scanned, 1 served away") {
		t.Errorf("want m-stale listed alone (4 scanned, 1 differing), got:\n%s", out.String())
	}
	for _, banned := range []string{"Open Road", "porch", "Demo"} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("output leaks title/lyric text %q", banned)
		}
	}
	if after := head(t, f.dir, f.song); after.Revision != before.Revision || !reflect.DeepEqual(after.Objects, before.Objects) {
		t.Fatal("the dry run wrote")
	}

	// 2. A server holds the dir: refused, nothing written.
	lock, err := filerepo.LockDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply"}, &out); !errors.Is(err, errLive) {
		t.Fatalf("with the server's lock held: err = %v, want errLive", err)
	}
	lock.Close()
	if after := head(t, f.dir, f.song); after.Revision != before.Revision {
		t.Fatal("wrote behind a live server")
	}

	// 3. Apply: the stale mark now STORES what was served, through a new revision; the rest is untouched.
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply"}, &out); err != nil {
		t.Fatal(err)
	}
	t.Logf("apply:\n%s", out.String())
	after := head(t, f.dir, f.song)
	if after.Revision != before.Revision+1 {
		t.Errorf("revision %d -> %d, want exactly one new revision", before.Revision, after.Revision)
	}
	got, was := byID(after), byID(before)
	st := got["m-stale"]
	if st.Page != f.expected.Page || !reflect.DeepEqual(st.Points, f.expected.Points) {
		t.Errorf("m-stale stored %d %v, want the served %d %v", st.Page, st.Points, f.expected.Page, f.expected.Points)
	}
	if st.PointsRenderHash != f.newHash {
		t.Errorf("m-stale hash %q, want the current blob hash %q", st.PointsRenderHash, f.newHash)
	}
	if !reflect.DeepEqual(st.Anchor, was["m-stale"].Anchor) {
		t.Errorf("m-stale anchor changed: %+v -> %+v (the link must be kept for stage 4)", was["m-stale"].Anchor, st.Anchor)
	}
	for _, id := range []string{"m-lost", "m-unlinked", "m-current"} {
		if !reflect.DeepEqual(got[id], was[id]) {
			t.Errorf("%s changed", id)
		}
	}

	// 4. Idempotent: a second pass finds nothing.
	out.Reset()
	if err := run([]string{"-data", f.dir}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "0 served away") {
		t.Errorf("second pass still finds marks:\n%s", out.String())
	}
}

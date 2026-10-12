package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
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

// The fixture: a synthetic generated chart (testutil.FreezeChart) with
//   - m-old:    an OLD-STYLE anchored rect on the target lyric, stored half a line below its old projection;
//   - m-icon:   an unlinked icon on the first lyric;
//   - m-margin: an unlinked rect in the right margin beside the target lyric;
//   - m-far:    an unlinked icon far below every line (the proposal is fixed);
//   - m-new:    a mark already linked by stage 2 (never a candidate);
//
// and an UPLOADED PDF in the same song with a mark (never a candidate).
type fixture struct {
	dir, song, chart, hash string
	targetLine, firstLine  string // the line ids the proposals must name
}

func openEngine(t *testing.T, dir string) *engine.Engine {
	t.Helper()
	return engine.New(filestore.New(dir).(store.HistoryAware))
}

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
	pdf, err := chartpdf.Render("# Uploaded sheet\n\nC G\nSome uploaded words\n")
	if err != nil {
		t.Fatal(err)
	}
	upload, err := svc.UploadSongFile(u, band.ID, song.ID, "sheet.pdf", "application/pdf", pdf)
	if err != nil {
		t.Fatal(err)
	}
	apply := func(m domain.Mutation) {
		t.Helper()
		if _, err := eng.Apply(song.ID, m); err != nil {
			t.Fatal(err)
		}
	}
	for _, l := range []domain.Layer{{ID: "L1", FileID: chart.ID}, {ID: "L2", FileID: upload.ID}} {
		l := l
		l.Name, l.OwnerID, l.Zone, l.Access = "Marks", domain.SharedOwner, domain.ZoneShared, domain.AccessRW
		apply(domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: u.ID, Layer: &l})
	}
	r, err := svc.ChartRenderForFile(chart.ID)
	if err != nil {
		t.Fatal(err)
	}
	line := func(prefix string) chartpdf.Anchor {
		for _, a := range r.Anchors {
			if strings.HasPrefix(a.Text, prefix) {
				return a
			}
		}
		t.Fatalf("fixture: no line %q", prefix)
		return chartpdf.Anchor{}
	}
	target, first := line("Leave the porch"), line("Pack a little light")
	style := domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.003}
	mk := func(uuid, layer string, typ domain.ObjectType, text string, pts ...domain.Point) domain.Object {
		return domain.Object{UUID: uuid, LayerID: layer, Type: typ, Page: 0, Version: 1, OwnerID: u.ID, Text: text, Points: pts, Style: style}
	}
	h := target.Y1 - target.Y0
	old := mk("m-old", "L1", domain.TypeRect, "",
		domain.Point{X: target.X0 + 0.01, Y: target.Y0 + 0.5*h}, domain.Point{X: target.X0 + 0.12, Y: target.Y1 + 0.5*h})
	old.Anchor = &domain.SourceAnchor{RunText: target.Text, Occurrence: 1, CharStart: 0, CharEnd: 10}
	old.PointsRenderHash = "an-older-render"
	fh := first.Y1 - first.Y0
	objs := []domain.Object{
		old,
		mk("m-icon", "L1", domain.TypeIcon, "star", domain.Point{X: first.X0 + 0.05, Y: first.Y0}, domain.Point{X: first.X0 + 0.07, Y: first.Y1}),
		mk("m-margin", "L1", domain.TypeRect, "", domain.Point{X: 0.90, Y: target.Y0}, domain.Point{X: 0.95, Y: target.Y1}),
		mk("m-far", "L1", domain.TypeIcon, "star", domain.Point{X: 0.5, Y: 0.90}, domain.Point{X: 0.52, Y: 0.92}),
		chartpdf.LinkMark(mk("m-new", "L1", domain.TypeRect, "", domain.Point{X: first.X0, Y: first.Y0 - 0.1*fh}, domain.Point{X: first.X0 + 0.1, Y: first.Y1}), r),
		mk("m-upload", "L2", domain.TypeRect, "", domain.Point{X: 0.1, Y: 0.1}, domain.Point{X: 0.3, Y: 0.15}),
	}
	for _, o := range objs {
		o := o
		apply(domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: u.ID, Object: &o})
	}
	return fixture{dir: dir, song: song.ID, chart: chart.ID, hash: chart.BlobHash,
		targetLine: r.Lines.IDs[target.SrcLine], firstLine: r.Lines.IDs[first.SrcLine]}
}

func byID(t *testing.T, dir, song string) (map[string]domain.Object, uint64) {
	t.Helper()
	snap, err := openEngine(t, dir).Head(song)
	if err != nil {
		t.Fatal(err)
	}
	m := map[string]domain.Object{}
	for _, o := range snap.LiveObjects() {
		m[o.UUID] = o
	}
	return m, snap.Revision
}

// dataDigest hashes every file the tool could write in the data dir (app.json and the song logs).
func dataDigest(t *testing.T, dir string) string {
	t.Helper()
	files := []string{filepath.Join(dir, "app.json")}
	songs, _ := filepath.Glob(filepath.Join(dir, "songs", "*"))
	files = append(files, songs...)
	sort.Strings(files)
	h := sha256.New()
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		h.Write([]byte(f))
		h.Write(b)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func readEntries(t *testing.T, outDir string) map[string]entry {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(outDir, "marks.json"))
	if err != nil {
		t.Fatal(err)
	}
	var es []entry
	if err := json.Unmarshal(b, &es); err != nil {
		t.Fatal(err)
	}
	m := map[string]entry{}
	for _, e := range es {
		m[e.ObjectID] = e
	}
	return m
}

func ids(m map[string]entry) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// hasColour reports whether the crop holds a pixel of exactly c (the overlay colours do not occur in a
// black-on-white chart).
func hasColour(img image.Image, c color.RGBA) bool {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if uint8(r>>8) == c.R && uint8(g>>8) == c.G && uint8(bb>>8) == c.B {
				return true
			}
		}
	}
	return false
}

func TestRelink(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm (poppler) not installed")
	}
	f := newDataDir(t)
	before, rev0 := byID(t, f.dir, f.song)
	digest0 := dataDigest(t, f.dir)

	// 1. The dry run (the default): JSON + one crop per candidate, and nothing written to the data dir.
	review := filepath.Join(t.TempDir(), "review")
	var out bytes.Buffer
	if err := run([]string{"-data", f.dir, "-out", review}, &out); err != nil {
		t.Fatalf("dry run: %v\n%s", err, out.String())
	}
	t.Logf("dry run:\n%s", out.String())
	if d := dataDigest(t, f.dir); d != digest0 {
		t.Fatal("the dry run changed the data dir")
	}
	es := readEntries(t, review)
	if got, want := ids(es), []string{"m-far", "m-icon", "m-margin", "m-old"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("candidates %v, want %v (never the linked mark or the uploaded PDF's)", got, want)
	}
	expect := map[string]struct {
		old, prop, line string
		margin          bool
	}{
		"m-old":    {oldStyle, "linked", f.targetLine, false},
		"m-icon":   {oldNone, "linked", f.firstLine, false},
		"m-margin": {oldNone, "linked", f.targetLine, true},
		"m-far":    {oldNone, "fixed", "", false},
	}
	for id, x := range expect {
		e := es[id]
		if e.OldState != x.old || e.Proposed != x.prop || e.LineID != x.line || e.Margin != x.margin {
			t.Errorf("%s: old %s proposed %s line %q margin %v; want %s %s %q %v", id, e.OldState, e.Proposed, e.LineID, e.Margin, x.old, x.prop, x.line, x.margin)
		}
		if e.SongID != f.song || e.FileID != f.chart || e.Reason == "" {
			t.Errorf("%s: song %q file %q reason %q", id, e.SongID, e.FileID, e.Reason)
		}
		if x.prop == "linked" && (e.LineText == "" || e.DX == nil || e.DY == nil) {
			t.Errorf("%s: a linked proposal needs the line text and dx/dy: %+v", id, e)
		}
		fp, err := os.Open(filepath.Join(review, id+".png"))
		if err != nil {
			t.Fatalf("%s: no crop: %v", id, err)
		}
		img, err := png.Decode(fp)
		fp.Close()
		if err != nil {
			t.Fatalf("%s: crop: %v", id, err)
		}
		if img.Bounds().Dx() < 600 {
			t.Errorf("%s: crop %d px wide, want ≥ 600", id, img.Bounds().Dx())
		}
		if !hasColour(img, colRed) {
			t.Errorf("%s: the crop does not outline the mark in red", id)
		}
		if hasColour(img, colBlue) != (x.prop == "linked") {
			t.Errorf("%s: blue run box drawn = %v, want %v", id, !(x.prop == "linked"), x.prop == "linked")
		}
		if hasColour(img, colOrange) != (x.old == oldStyle) {
			t.Errorf("%s: dashed orange old-projection drawn = %v, want %v", id, !(x.old == oldStyle), x.old == oldStyle)
		}
	}
	if pi := es["m-far"].PageImage; pi == "" {
		t.Error("m-far (fixed): no whole-page image for context")
	} else if _, err := os.Stat(filepath.Join(review, pi)); err != nil {
		t.Errorf("m-far: %v", err)
	}
	if o := es["m-old"].Old; o == nil || o.Projected == nil {
		t.Error("m-old: marks.json does not carry the old anchor's projection")
	}
	if !strings.Contains(out.String(), "old-style  -> linked   1") || !strings.Contains(out.String(), "unlinked   -> fixed    1") ||
		!strings.Contains(out.String(), "unlinked   -> linked   2") {
		t.Errorf("summary counts wrong:\n%s", out.String())
	}
	for _, banned := range []string{"Open Road", "porch", "Demo"} {
		if strings.Contains(out.String(), banned) {
			t.Errorf("stdout leaks title/lyric text %q", banned)
		}
	}

	// 2. A server holds the dir: both modes refuse, nothing written.
	approve := filepath.Join(t.TempDir(), "approved.txt")
	if err := os.WriteFile(approve, []byte("# VLL, one by one\nm-old\n\nm-far\nm-margin:fixed\nm-upload\nm-new\nm-bogus\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	lock, err := filerepo.LockDir(f.dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-data", f.dir, "-apply", "-approve", approve, "-review", filepath.Join(review, "marks.json")}, {"-data", f.dir, "-out", review}} {
		out.Reset()
		if err := run(args, &out); !errors.Is(err, errLive) {
			t.Fatalf("%v with the server's lock held: err = %v, want errLive", args, err)
		}
	}
	lock.Close()
	if d := dataDigest(t, f.dir); d != digest0 {
		t.Fatal("wrote behind a live server")
	}

	// 3. Apply: only the approved candidates are written, each in its own revision.
	out.Reset()
	if err := run([]string{"-data", f.dir, "-apply", "-approve", approve, "-review", filepath.Join(review, "marks.json")}, &out); err != nil {
		t.Fatalf("apply: %v\n%s", err, out.String())
	}
	t.Logf("apply:\n%s", out.String())
	after, rev1 := byID(t, f.dir, f.song)
	if rev1 != rev0+3 {
		t.Errorf("revision %d -> %d, want exactly 3 new revisions (m-old, m-far, m-margin)", rev0, rev1)
	}
	for _, id := range []string{"m-icon", "m-new", "m-upload"} {
		if !reflect.DeepEqual(after[id], before[id]) {
			t.Errorf("%s was not approved as a candidate, yet it changed:\n  before %+v\n  after  %+v", id, before[id], after[id])
		}
	}
	want := map[string]struct {
		state domain.LinkState
		line  string
	}{
		"m-old":    {domain.LinkLinked, f.targetLine},
		"m-far":    {domain.LinkFixed, ""},
		"m-margin": {domain.LinkFixed, ""}, // forced by ":fixed", although the rule would link it
	}
	for id, x := range want {
		o, was := after[id], before[id]
		if o.Link == nil || o.Link.State != x.state || o.Link.Line != x.line {
			t.Errorf("%s: link %+v, want %s on %q", id, o.Link, x.state, x.line)
		}
		if o.Anchor != nil || o.PointsRenderHash != f.hash || o.Version != was.Version+1 {
			t.Errorf("%s: anchor %+v hash %q version %d; want no anchor, hash %q, version %d", id, o.Anchor, o.PointsRenderHash, o.Version, f.hash, was.Version+1)
		}
		if o.Page != was.Page || !reflect.DeepEqual(o.Points, was.Points) || !reflect.DeepEqual(o.Style, was.Style) || o.Type != was.Type {
			t.Errorf("%s: the apply moved or restyled the mark", id)
		}
	}
	for _, id := range []string{"m-upload", "m-new", "m-bogus"} {
		if !strings.Contains(out.String(), "skip  "+id+": not a candidate") {
			t.Errorf("approved non-candidate %s is not listed as skipped", id)
		}
	}

	// 4. A second dry run: the approved marks are no longer candidates.
	review2 := filepath.Join(t.TempDir(), "review2")
	out.Reset()
	if err := run([]string{"-data", f.dir, "-out", review2}, &out); err != nil {
		t.Fatal(err)
	}
	if got := ids(readEntries(t, review2)); !reflect.DeepEqual(got, []string{"m-icon"}) {
		t.Errorf("second dry run candidates %v, want [m-icon]", got)
	}
}

func TestReadApprovals(t *testing.T) {
	p := filepath.Join(t.TempDir(), "a.txt")
	_ = os.WriteFile(p, []byte(" a \n#x\nb:fixed\n\n"), 0o600)
	m, err := readApprovals(p)
	if err != nil || !reflect.DeepEqual(m, map[string]bool{"a": false, "b": true}) {
		t.Fatalf("got %v %v", m, err)
	}
	for _, bad := range []string{"a:linked\n", "a b\n", ":fixed\n"} {
		_ = os.WriteFile(p, []byte(bad), 0o600)
		if _, err := readApprovals(p); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
	if err := run([]string{"-data", t.TempDir(), "-apply"}, &bytes.Buffer{}); err == nil {
		t.Error("-apply without -approve accepted")
	}
	_ = os.WriteFile(p, []byte("a\n"), 0o600)
	if err := run([]string{"-data", t.TempDir(), "-apply", "-approve", p}, &bytes.Buffer{}); err == nil {
		t.Error("-apply without -review (the reviewed marks.json) accepted")
	}
	if err := run([]string{"-data", t.TempDir()}, &bytes.Buffer{}); err == nil {
		t.Error("a dry run without -out accepted")
	}
}

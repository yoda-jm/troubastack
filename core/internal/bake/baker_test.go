package bake

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// tinyPNG returns a valid w×h PNG (fake rasterizer/overlay output; the Baker
// decodes its dimensions via image.DecodeConfig, so it must be real PNG bytes).
func tinyPNG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

type fakeRaster struct {
	pages int
	png   []byte
}

func (f fakeRaster) Rasterize(context.Context, []byte) ([][]byte, error) {
	out := make([][]byte, f.pages)
	for i := range out {
		out[i] = f.png
	}
	return out, nil
}

// fakeOverlays returns one overlay per doc layer on page 0 (mimics web/bake).
type fakeOverlays struct{ png []byte }

func (f fakeOverlays) Render(_ context.Context, req cliRequest) ([]renderedOverlay, error) {
	var out []renderedOverlay
	for _, l := range req.Doc.Layers {
		out = append(out, renderedOverlay{
			Page: 0, LayerID: l.ID, Order: int32(l.Order), Mandatory: l.Mandatory,
			RoleTag: l.RoleTag, ContentHash: Sha256Hex(f.png), PNG: f.png,
		})
	}
	return out, nil
}

// seed builds a service + engine with one band/song/PDF/annotation-layer/setlist.
func seed(t *testing.T) (*app.Service, *engine.Engine, app.User, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatalf("create band: %v", err)
	}
	song, err := svc.CreateSong(u, band.ID, "Song", "")
	if err != nil {
		t.Fatalf("create song: %v", err)
	}
	if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload file: %v", err)
	}
	// One annotation layer + a rect object on it (the renderer doc source).
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", Name: "Marks", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
		AuthorID: u.ID,
	}); err != nil {
		t.Fatalf("layer create: %v", err)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind: domain.KindCreate, UUID: "o1", AuthorID: u.ID,
		Object: &domain.Object{
			UUID: "o1", LayerID: "L1", Type: domain.TypeRect, Page: 0, Version: 1,
			Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
			Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
		},
	}); err != nil {
		t.Fatalf("object create: %v", err)
	}
	sl, err := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("create setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	return svc, eng, u, band.ID, sl.ID
}

func TestBake_ProducesValidBundle_andBumpsRev(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	pngBytes := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: pngBytes},
		overlays: fakeOverlays{png: pngBytes},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	cb, err := b.Bake(context.Background(), bandID, setlistID, u, false)
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if cb.ConcertRev != 1 {
		t.Fatalf("first bake rev = %d, want 1", cb.ConcertRev)
	}
	if len(cb.Songs) != 1 || len(cb.Songs[0].Pages) != 1 {
		t.Fatalf("want 1 song × 1 page, got %d songs", len(cb.Songs))
	}
	if got := len(cb.Songs[0].Pages[0].Overlays); got != 1 {
		t.Fatalf("want 1 overlay (one layer), got %d", got)
	}
	if cb.Songs[0].SourceRevision == 0 {
		t.Fatalf("source_revision should be the song's head revision, got 0")
	}
	// T26: the bundle carries the song's real Title (kills the "Song N" fallback).
	if cb.Songs[0].Title != "Song" {
		t.Fatalf("baked song title = %q, want the song's title (T26)", cb.Songs[0].Title)
	}

	// bundle.json parses and every blob ref resolves on disk (the container contract).
	revDir := filepath.Join(b.bakesDir, setlistID, "1")
	data, err := os.ReadFile(filepath.Join(revDir, "bundle.json"))
	if err != nil {
		t.Fatalf("read bundle.json: %v", err)
	}
	var parsed ConcertBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("bundle.json does not parse: %v", err)
	}
	for _, song := range parsed.Songs {
		for _, page := range song.Pages {
			for _, ref := range append([]string{page.PageRasterRef}, refsOf(page.Overlays)...) {
				if _, err := os.Stat(filepath.Join(revDir, filepath.FromSlash(ref))); err != nil {
					t.Fatalf("blob ref %q missing: %v", ref, err)
				}
			}
		}
	}
	// The .tstage was written alongside the rev dir.
	if _, err := os.Stat(filepath.Join(b.bakesDir, setlistID, "1.tstage")); err != nil {
		t.Fatalf(".tstage missing: %v", err)
	}

	// Re-bake bumps concert_rev monotonically.
	cb2, err := b.Bake(context.Background(), bandID, setlistID, u, false)
	if err != nil {
		t.Fatalf("re-bake: %v", err)
	}
	if cb2.ConcertRev != 2 {
		t.Fatalf("re-bake rev = %d, want 2", cb2.ConcertRev)
	}
	if got := b.BundlePath(setlistID); got != filepath.Join(b.bakesDir, setlistID, "2.tstage") {
		t.Fatalf("BundlePath = %q, want the rev-2 tstage", got)
	}
	if concerts := b.ListConcerts(); len(concerts) != 1 || concerts[0].ConcertRev != 2 {
		t.Fatalf("ListConcerts should show the latest rev (2), got %+v", concerts)
	}
}

// TestBake_BenchSortsAfterMain_flagsOnCall (T23): a bench (on-call) item is baked
// AFTER the whole main order regardless of its position, and carries on_call in the
// bundle; main entries stay on_call=false.
func TestBake_BenchSortsAfterMain_flagsOnCall(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t) // seed adds one main song ("Song", pos 0)

	mkSong := func(title string) string {
		s, err := svc.CreateSong(u, bandID, title, "")
		if err != nil {
			t.Fatalf("create song %q: %v", title, err)
		}
		if _, err := svc.UploadSongFile(u, bandID, s.ID, "s.pdf", "application/pdf", []byte("%PDF-1.4 x")); err != nil {
			t.Fatalf("upload %q: %v", title, err)
		}
		return s.ID
	}

	// Add the bench song EARLY (position 1) so position order alone would put it
	// second — the on_call sort must still push it last.
	benchSong := mkSong("Bench")
	benchItem, err := svc.AddSetlistItem(u, bandID, setlistID, benchSong)
	if err != nil {
		t.Fatalf("add bench item: %v", err)
	}
	// Two more main songs after it (positions 2, 3).
	mainC := mkSong("Cmain")
	mainD := mkSong("Dmain")
	for _, sid := range []string{mainC, mainD} {
		if _, err := svc.AddSetlistItem(u, bandID, setlistID, sid); err != nil {
			t.Fatalf("add main item: %v", err)
		}
	}
	onCall := true
	if _, err := svc.UpdateSetlistItem(u, bandID, setlistID, benchItem.ID, app.SetlistItemPatch{OnCall: &onCall}); err != nil {
		t.Fatalf("set on-call: %v", err)
	}

	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	cb, err := b.Bake(context.Background(), bandID, setlistID, u, false)
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if len(cb.Songs) != 4 {
		t.Fatalf("want 4 baked songs, got %d", len(cb.Songs))
	}
	// The three main songs come first, all on_call=false; the bench song is LAST.
	for i, s := range cb.Songs[:3] {
		if s.OnCall {
			t.Errorf("song %d (%s) should be main (on_call=false)", i, s.SongID)
		}
	}
	last := cb.Songs[3]
	if !last.OnCall {
		t.Errorf("last baked song should be the bench item (on_call=true)")
	}
	if last.SongID != benchSong {
		t.Errorf("bench song should sort last despite position 1; got last=%s want=%s", last.SongID, benchSong)
	}

	// B07: the PERSONAL variant bake also includes the bench, flagged and last —
	// the bench is a setlist-structure property, independent of which file resolves.
	pcb, err := b.Bake(context.Background(), bandID, setlistID, u, true)
	if err != nil {
		t.Fatalf("personal bake: %v", err)
	}
	if len(pcb.Songs) != 4 || !pcb.Songs[3].OnCall || pcb.Songs[3].SongID != benchSong {
		t.Fatalf("variant bake should include the bench last+flagged, got %+v", pcb.Songs)
	}
}

// TestBake_ConcurrentSameSetlist_distinctRevs is the B04 guard: two bakes of the
// same setlist running at once must mint DISTINCT revs (atomic rev claim), both fully
// published (atomic rename), with no staging dir left visible.
func TestBake_ConcurrentSameSetlist_distinctRevs(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	// n=4 (not 2): with >2 racers the B08 re-claim path fires often, which is exactly
	// where the B09 two-phase .tstage matters — a loser must never clobber the winner's
	// published <rev>.tstage. The per-rev checks below (every rev has its .tstage) catch
	// a clobbered/removed one under -race.
	const n = 4
	var wg sync.WaitGroup
	res := make([]ConcertBundle, n)
	errs := make([]error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			res[i], errs[i] = b.Bake(context.Background(), bandID, setlistID, u, false)
		}(i)
	}
	wg.Wait()

	for i := 0; i < n; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent bake %d failed: %v", i, errs[i])
		}
	}
	seen := map[uint64]bool{}
	for i := 0; i < n; i++ {
		if seen[res[i].ConcertRev] {
			t.Fatalf("rev %d minted twice — not distinct (rev race)", res[i].ConcertRev)
		}
		seen[res[i].ConcertRev] = true
	}

	// Both revs are fully published: bundle.json parses, every blob ref resolves, .tstage exists.
	for _, cb := range res {
		revName := strconv.FormatUint(cb.ConcertRev, 10)
		revDir := filepath.Join(b.bakesDir, setlistID, revName)
		data, err := os.ReadFile(filepath.Join(revDir, "bundle.json"))
		if err != nil {
			t.Fatalf("rev %s bundle.json: %v", revName, err)
		}
		var parsed ConcertBundle
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("rev %s bundle.json parse: %v", revName, err)
		}
		for _, song := range parsed.Songs {
			for _, page := range song.Pages {
				for _, ref := range append([]string{page.PageRasterRef}, refsOf(page.Overlays)...) {
					if _, err := os.Stat(filepath.Join(revDir, filepath.FromSlash(ref))); err != nil {
						t.Fatalf("rev %s blob %q missing: %v", revName, ref, err)
					}
				}
			}
		}
		if _, err := os.Stat(filepath.Join(b.bakesDir, setlistID, revName+".tstage")); err != nil {
			t.Fatalf("rev %s .tstage missing: %v", revName, err)
		}
	}

	// No staging dir is left behind (atomic publish cleaned/renamed it).
	entries, err := os.ReadDir(filepath.Join(b.bakesDir, setlistID))
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("staging dir left visible after publish: %s", e.Name())
		}
	}

	// ListConcerts / BundlePath expose the highest rev cleanly (latest-per-concert).
	var hi uint64
	for i := 0; i < n; i++ {
		if res[i].ConcertRev > hi {
			hi = res[i].ConcertRev
		}
	}
	concerts := b.ListConcerts()
	if len(concerts) != 1 || concerts[0].ConcertRev != hi {
		t.Fatalf("ListConcerts = %+v, want one concert at rev %d", concerts, hi)
	}
	if got, want := b.BundlePath(setlistID), filepath.Join(b.bakesDir, setlistID, strconv.FormatUint(hi, 10)+".tstage"); got != want {
		t.Fatalf("BundlePath = %q, want %q", got, want)
	}
}

// TestBake_PublishReclaimsOnConcurrentPublish drives the EXACT B08 window
// deterministically: bake B runs nextRev (claims rev 1), then — via the afterNextRev
// test seam, before B's own claim/publish — a full bake A publishes rev 1. B must NOT
// fail its publish; it re-claims a higher rev and lands a distinct, fully-published
// one. This fails on the pre-B08 code (B's rename 1.tmp→1 errors "file exists").
func TestBake_PublishReclaimsOnConcurrentPublish(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	var once sync.Once
	var aRev uint64
	b.afterNextRev = func() {
		once.Do(func() {
			// A: a full bake to completion (publishes rev 1) inside B's window, using a
			// hookless copy so it doesn't recurse. Same goroutine → t.Errorf is safe.
			a := *b
			a.afterNextRev = nil
			ab, err := a.Bake(context.Background(), bandID, setlistID, u, false)
			if err != nil {
				t.Errorf("inner bake A failed: %v", err)
				return
			}
			aRev = ab.ConcertRev
		})
	}

	bb, err := b.Bake(context.Background(), bandID, setlistID, u, false)
	if err != nil {
		t.Fatalf("B must re-claim, not fail, when its rev was published concurrently: %v", err)
	}
	if aRev == 0 {
		t.Fatal("inner bake A did not run")
	}
	if bb.ConcertRev == aRev {
		t.Fatalf("B minted the same rev as A (%d) — not distinct", bb.ConcertRev)
	}

	// Both A and B are fully published: distinct <rev> dirs + .tstage, no leftover .tmp.
	for _, rev := range []uint64{aRev, bb.ConcertRev} {
		revName := strconv.FormatUint(rev, 10)
		if _, err := os.Stat(filepath.Join(b.bakesDir, setlistID, revName, "bundle.json")); err != nil {
			t.Fatalf("rev %s bundle.json missing: %v", revName, err)
		}
		if _, err := os.Stat(filepath.Join(b.bakesDir, setlistID, revName+".tstage")); err != nil {
			t.Fatalf("rev %s .tstage missing: %v", revName, err)
		}
	}
	entries, _ := os.ReadDir(filepath.Join(b.bakesDir, setlistID))
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Fatalf("staging dir left behind: %s", e.Name())
		}
	}
}

func refsOf(overlays []LayerImage) []string {
	out := make([]string, len(overlays))
	for i, o := range overlays {
		out[i] = o.ImageRef
	}
	return out
}

// TestPopplerRasterizer exercises the REAL pdftoppm on a minimal one-page PDF.
// Skips when the binary is absent (local dev without poppler); CI installs
// poppler-utils so it runs there.
func TestPopplerRasterizer(t *testing.T) {
	if _, err := exec.LookPath("pdftoppm"); err != nil {
		t.Skip("pdftoppm not installed")
	}
	r := popplerRasterizer{bin: "pdftoppm", dpi: 72}
	pages, err := r.Rasterize(context.Background(), []byte(minimalPDF))
	if err != nil {
		t.Fatalf("rasterize: %v", err)
	}
	if len(pages) != 1 {
		t.Fatalf("want 1 page, got %d", len(pages))
	}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(pages[0])); err != nil || cfg.Width == 0 {
		t.Fatalf("page 0 is not a valid PNG: %v (w=%d)", err, cfg.Width)
	}
}

// A minimal valid single-page PDF (200×280pt, blank). poppler repairs the xref if
// needed, so this rasterizes to one blank page.
const minimalPDF = "%PDF-1.4\n" +
	"1 0 obj<</Type/Catalog/Pages 2 0 R>>endobj\n" +
	"2 0 obj<</Type/Pages/Kids[3 0 R]/Count 1>>endobj\n" +
	"3 0 obj<</Type/Page/Parent 2 0 R/MediaBox[0 0 200 280]>>endobj\n" +
	"trailer<</Root 1 0 R>>\n%%EOF\n"

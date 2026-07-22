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
	"time"

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

	cb, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
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
	// T53: the baked overlay carries the source Layer's Name (looked up from the snapshot),
	// so the viewer can label layers instead of showing the raw layer id.
	if got := cb.Songs[0].Pages[0].Overlays[0].Name; got != "Marks" {
		t.Fatalf("baked overlay name = %q, want the source layer's name %q (T53)", got, "Marks")
	}
	// P205: the overlay carries its owner (the seed layer L1 is owned by u, not shared),
	// so a band-wide bundle can be filtered to the viewer's identity at view time.
	if got := cb.Songs[0].Pages[0].Overlays[0].Owner; got != u.ID {
		t.Fatalf("baked overlay owner = %q, want the layer owner %q (P205)", got, u.ID)
	}
	// P205: the bundle carries the band roster for view-time identity resolution.
	if len(cb.Roster) != 1 || cb.Roster[0].MemberID != u.ID || cb.Roster[0].Role != "admin" {
		t.Fatalf("bundle roster = %+v, want one admin member %q (P205)", cb.Roster, u.ID)
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
	cb2, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
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
// TestBake_DefaultOnCapture is the P205 bake-dialog guard: when the dialog runs
// (layerDefaults non-nil), every overlay carries an explicit default_on keyed by
// layer name — toggling "Marks" off must land default_on=false in the bundle; with
// nil the field stays absent (legacy compute).
func TestBake_DefaultOnCapture(t *testing.T) {
	png := tinyPNG(t, 40, 56)
	mk := func() (*Baker, app.User, string, string) {
		svc, eng, u, bandID, setlistID := seed(t)
		return &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 1, png: png}, overlays: fakeOverlays{png: png}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}, u, bandID, setlistID
	}
	// nil → absent (legacy).
	b, u, bandID, setlistID := mk()
	cb, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
	if err != nil {
		t.Fatalf("bake(nil): %v", err)
	}
	if ov := cb.Songs[0].Pages[0].Overlays[0]; ov.DefaultOn != nil {
		t.Fatalf("no dialog → DefaultOn must be nil (legacy), got %v", *ov.DefaultOn)
	}
	// Dialog turned "Marks" OFF → explicit default_on=false on that overlay.
	b2, u2, bandID2, setlistID2 := mk()
	cb2, err := b2.Bake(context.Background(), bandID2, setlistID2, u2, map[string]bool{"Marks": false})
	if err != nil {
		t.Fatalf("bake(off): %v", err)
	}
	ov := cb2.Songs[0].Pages[0].Overlays[0]
	if ov.DefaultOn == nil || *ov.DefaultOn {
		t.Fatalf("Marks toggled off → DefaultOn should be non-nil false, got %v", ov.DefaultOn)
	}
	// Dialog turned "Marks" ON → default_on=true.
	b3, u3, bandID3, setlistID3 := mk()
	cb3, err := b3.Bake(context.Background(), bandID3, setlistID3, u3, map[string]bool{"Marks": true})
	if err != nil {
		t.Fatalf("bake(on): %v", err)
	}
	if ov := cb3.Songs[0].Pages[0].Overlays[0]; ov.DefaultOn == nil || !*ov.DefaultOn {
		t.Fatalf("Marks on → DefaultOn should be non-nil true, got %v", ov.DefaultOn)
	}
}

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
	cb, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
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
}

// TestBake_MemberCuesInjected is the T50/P205 guard: the band-wide bake carries a
// member's song cues as member_cues (field 11, keyed by id), and leaves field 10
// (`cues`) empty so an old app degrades to no-cues. (The personal-variant field-10
// path was retired with the ?scope=mine bake.)
func TestBake_MemberCuesInjected(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)

	// The seeded setlist has one song; grab its id and set two personal cues on it.
	songs, err := svc.Songs(u, bandID)
	if err != nil || len(songs) != 1 {
		t.Fatalf("songs: %v (len %d)", err, len(songs))
	}
	songID := songs[0].ID
	want := []app.SongCue{{Icon: "mic"}, {Icon: "guitar-electric", Color: "#e11d48"}}
	if _, err := svc.SetMyCues(u, bandID, songID, want); err != nil {
		t.Fatalf("set my cues: %v", err)
	}

	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	// Shared band bake: cues are personal → none ride.
	shared, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
	if err != nil {
		t.Fatalf("shared bake: %v", err)
	}
	if len(shared.Songs) != 1 {
		t.Fatalf("want 1 baked song, got %d", len(shared.Songs))
	}
	if len(shared.Songs[0].Cues) != 0 {
		t.Fatalf("shared bake cues = %+v, want none in field 10 (cues are personal)", shared.Songs[0].Cues)
	}
	// P205: the band-wide (admin) bake carries every member's cues as member_cues,
	// keyed by member id — the viewer filters to its own identity (Stage 3). Field 10
	// stays empty so an old app degrades to no-cues, never wrong-cues.
	mc := shared.Songs[0].MemberCues
	if len(mc) != 1 || mc[0].MemberID != u.ID || len(mc[0].Cues) != 2 ||
		mc[0].Cues[0].Icon != "mic" || mc[0].Cues[1].Color != "#e11d48" {
		t.Fatalf("shared bake member_cues = %+v, want u's cues keyed by member id (P205)", mc)
	}
}

// TestBake_BandWide_CarriesEveryMember is the P205 Stage-2 contract guard: ONE
// band-wide bake (personal=false, baked by the admin) is THE bake for the whole
// band — it must carry EVERY member's cues (member_cues) AND every member's
// personal layer (owner-tagged), not just the actor's. This is the invariant that
// lets scope=mine retire (Stage 2 step 4): if the band-wide bundle didn't already
// carry a non-actor member's content, retiring the per-member bake would silently
// drop it. The single-member tests above can't distinguish "the actor's" from
// "everyone's" — this one has a second member whose content the admin never owns.
func TestBake_BandWide_CarriesEveryMember(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo)
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatalf("register admin: %v", err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatalf("band: %v", err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatalf("song: %v", err)
	}
	if _, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	// Leo is a second member (added directly — no invite flow needed here).
	leo, err := svc.Register("leo", "Leo", "password123", "")
	if err != nil {
		t.Fatalf("register leo: %v", err)
	}
	if err := repo.AddMembership(app.Membership{BandID: band.ID, UserID: leo.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
		t.Fatalf("add leo: %v", err)
	}

	// Each member has a PERSONAL annotation layer (owner-tagged) + a rect on it, and
	// their own song cues. The admin owns neither Leo's layer nor Leo's cues.
	mkLayer := func(layerID, owner string, order int) {
		t.Helper()
		if _, err := eng.Apply(song.ID, domain.Mutation{
			Kind:     domain.KindLayerCreate,
			Layer:    &domain.Layer{ID: layerID, Name: layerID, OwnerID: owner, Zone: domain.ZonePersonal, Order: order, Access: domain.AccessRW},
			AuthorID: owner,
		}); err != nil {
			t.Fatalf("layer %s: %v", layerID, err)
		}
		if _, err := eng.Apply(song.ID, domain.Mutation{
			Kind: domain.KindCreate, UUID: "o-" + layerID, AuthorID: owner,
			Object: &domain.Object{
				UUID: "o-" + layerID, LayerID: layerID, Type: domain.TypeRect, Page: 0, Version: 1,
				Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
				Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
			},
		}); err != nil {
			t.Fatalf("object %s: %v", layerID, err)
		}
	}
	mkLayer("LA", admin.ID, 0)
	mkLayer("LL", leo.ID, 1)

	if _, err := svc.SetMyCues(admin, band.ID, song.ID, []app.SongCue{{Icon: "mic"}}); err != nil {
		t.Fatalf("admin cues: %v", err)
	}
	if _, err := svc.SetMyCues(leo, band.ID, song.ID, []app.SongCue{{Icon: "guitar-acoustic"}, {Icon: "tambourine"}}); err != nil {
		t.Fatalf("leo cues: %v", err)
	}

	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}

	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	cb, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil)
	if err != nil {
		t.Fatalf("band-wide bake: %v", err)
	}

	// Roster carries both members.
	if len(cb.Roster) != 2 {
		t.Fatalf("roster = %+v, want both members", cb.Roster)
	}

	// member_cues carries BOTH members' cues, keyed by member id — including Leo's,
	// which the admin does not own.
	byMember := map[string][]SongCue{}
	for _, mc := range cb.Songs[0].MemberCues {
		byMember[mc.MemberID] = mc.Cues
	}
	if got := byMember[admin.ID]; len(got) != 1 || got[0].Icon != "mic" {
		t.Fatalf("admin member_cues = %+v, want [mic]", got)
	}
	if got := byMember[leo.ID]; len(got) != 2 || got[0].Icon != "guitar-acoustic" || got[1].Icon != "tambourine" {
		t.Fatalf("leo member_cues = %+v, want [guitar-acoustic, tambourine] (a NON-actor member's cues must ride the band-wide bake)", got)
	}

	// Both members' personal layers ride the band-wide bake, owner-tagged — so the
	// viewer can filter to its own identity at view time (Stage 3).
	owners := map[string]bool{}
	for _, pg := range cb.Songs[0].Pages {
		for _, ov := range pg.Overlays {
			owners[ov.Owner] = true
		}
	}
	if !owners[admin.ID] || !owners[leo.ID] {
		t.Fatalf("overlay owners = %v, want both admin %q and the NON-actor leo %q owner-tagged", owners, admin.ID, leo.ID)
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
			res[i], errs[i] = b.Bake(context.Background(), bandID, setlistID, u, nil)
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
			ab, err := a.Bake(context.Background(), bandID, setlistID, u, nil)
			if err != nil {
				t.Errorf("inner bake A failed: %v", err)
				return
			}
			aRev = ab.ConcertRev
		})
	}

	bb, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
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

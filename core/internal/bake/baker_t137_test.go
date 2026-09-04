package bake

import (
	"bytes"
	"context"
	"crypto/sha256"
	"image"
	"image/color"
	"image/png"
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

// contentRaster returns ONE valid PNG per file, deterministic in the file's bytes: distinct files →
// distinct rasters; byte-identical files → identical rasters (the ⟨D2⟩ dedup case). One page per file
// keeps the pool arithmetic obvious.
type contentRaster struct{}

func (contentRaster) Rasterize(_ context.Context, pdf []byte) ([][]byte, error) {
	h := sha256.Sum256(pdf)
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: h[0], G: h[1], B: h[2], A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return [][]byte{buf.Bytes()}, nil
}

// t137Env is a band with an admin (the default reader) + two members, one song, and the machinery to bake.
type t137Env struct {
	svc                *app.Service
	eng                *engine.Engine
	admin, marie, leo  app.User
	bandID, songID, sl string
}

func newT137Env(t *testing.T) t137Env {
	t.Helper()
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
	mk := func(username string) app.User {
		u, err := svc.Register(username, username, "password123", "")
		if err != nil {
			t.Fatalf("register %s: %v", username, err)
		}
		if err := repo.AddMembership(app.Membership{BandID: band.ID, UserID: u.ID, Role: app.RoleMember, CreatedAt: time.Now().UTC()}); err != nil {
			t.Fatalf("add %s: %v", username, err)
		}
		return u
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("item: %v", err)
	}
	return t137Env{svc: svc, eng: eng, admin: admin, marie: mk("marie"), leo: mk("leo"), bandID: band.ID, songID: song.ID, sl: sl.ID}
}

func (e t137Env) upload(t *testing.T, name string, content []byte) app.SongFile {
	t.Helper()
	f, err := e.svc.UploadSongFile(e.admin, e.bandID, e.songID, name, "application/pdf", content)
	if err != nil {
		t.Fatalf("upload %s: %v", name, err)
	}
	return f
}

func (e t137Env) selects(t *testing.T, who app.User, fileIDs ...string) {
	t.Helper()
	if _, err := e.svc.SetMyFileSelection(who, e.bandID, e.songID, fileIDs); err != nil {
		t.Fatalf("select: %v", err)
	}
}

func (e t137Env) bake(t *testing.T) ConcertBundle {
	t.Helper()
	b := &Baker{svc: e.svc, eng: e.eng, raster: contentRaster{}, overlays: fakeOverlays{png: []byte("ov")}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
	cb, _, err := b.Bake(context.Background(), e.bandID, e.sl, e.admin, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	return cb
}

func seqFor(mps []MemberPages, id string) []int32 {
	for _, mp := range mps {
		if mp.MemberID == id {
			return mp.Page
		}
	}
	return nil
}

// Two members with different selections get different page sequences from ONE bundle (acceptance §1).
func TestBakeT137_TwoMembersDifferentSequences(t *testing.T) {
	e := newT137Env(t)
	a := e.upload(t, "a.pdf", []byte("%PDF-1.4 AAA")) // default (lowest DisplayOrder)
	b := e.upload(t, "b.pdf", []byte("%PDF-1.4 BBB"))
	e.selects(t, e.marie, a.ID) // Marie reads the default file
	e.selects(t, e.leo, b.ID)   // Leo reads the other

	song := e.bake(t).Songs[0]
	if len(song.Pages) != 2 {
		t.Fatalf("pool = %d pages, want 2 (a + b)", len(song.Pages))
	}
	if song.Pages[0].PageRasterRef == song.Pages[1].PageRasterRef {
		t.Fatalf("distinct files should have distinct rasters")
	}
	// "" default and Marie both read file a (pool 0); Leo reads file b (pool 1).
	assertSeq(t, seqFor(song.MemberPages, ""), 0)
	assertSeq(t, seqFor(song.MemberPages, e.marie.ID), 0)
	assertSeq(t, seqFor(song.MemberPages, e.leo.ID), 1)
}

// No divergence ⇒ no member_pages ⇒ a bundle shaped exactly like today (acceptance §2/§3, ⟨D1⟩).
func TestBakeT137_UndivergentEmitsNoMemberPages(t *testing.T) {
	e := newT137Env(t)
	e.upload(t, "a.pdf", []byte("%PDF-1.4 AAA"))
	// nobody selects anything
	song := e.bake(t).Songs[0]
	if len(song.MemberPages) != 0 {
		t.Fatalf("member_pages = %d, want 0 (undivergent)", len(song.MemberPages))
	}
	if len(song.Pages) != 1 {
		t.Fatalf("pool = %d, want 1", len(song.Pages))
	}
}

// The same file selected by two members is stored once (union dedups by file id), still undivergent.
func TestBakeT137_SameFileTwoMembersDeduped(t *testing.T) {
	e := newT137Env(t)
	a := e.upload(t, "a.pdf", []byte("%PDF-1.4 AAA"))
	e.selects(t, e.marie, a.ID)
	e.selects(t, e.leo, a.ID)
	song := e.bake(t).Songs[0]
	if len(song.Pages) != 1 {
		t.Fatalf("pool = %d, want 1 (same file once)", len(song.Pages))
	}
	if len(song.MemberPages) != 0 {
		t.Fatalf("member_pages = %d, want 0 (everyone on one file)", len(song.MemberPages))
	}
}

// ⟨D2⟩ — two DISTINCT files with a byte-identical page but DIFFERENT per-file overlays: one stored raster
// blob (shared ref), two distinct entries with different overlay sets. Never merge the "pour flûte" marks.
func TestBakeT137_IdenticalRasterKeepsDistinctOverlays(t *testing.T) {
	e := newT137Env(t)
	a := e.upload(t, "a.pdf", []byte("%PDF-1.4 SAME")) // identical content ⇒ identical raster
	a2 := e.upload(t, "a2.pdf", []byte("%PDF-1.4 SAME"))
	// A per-file layer + object on each, scoped by FileID, so the two files draw DIFFERENT overlays.
	mkLayer := func(layerID, fileID string) {
		if _, err := e.eng.Apply(e.songID, domain.Mutation{
			Kind:     domain.KindLayerCreate,
			Layer:    &domain.Layer{ID: layerID, Name: layerID, OwnerID: e.admin.ID, FileID: fileID, Zone: domain.ZoneShared, Order: 0, Access: domain.AccessRW},
			AuthorID: e.admin.ID,
		}); err != nil {
			t.Fatalf("layer %s: %v", layerID, err)
		}
		if _, err := e.eng.Apply(e.songID, domain.Mutation{
			Kind: domain.KindCreate, UUID: "o-" + layerID, AuthorID: e.admin.ID,
			Object: &domain.Object{
				UUID: "o-" + layerID, LayerID: layerID, Type: domain.TypeRect, Page: 0, Version: 1,
				Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
				Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
			},
		}); err != nil {
			t.Fatalf("object %s: %v", layerID, err)
		}
	}
	mkLayer("LA", a.ID)
	mkLayer("LB", a2.ID)
	e.selects(t, e.marie, a.ID)
	e.selects(t, e.leo, a2.ID)

	song := e.bake(t).Songs[0]
	if len(song.Pages) != 2 {
		t.Fatalf("pool = %d entries, want 2 (one per file, entries NOT merged)", len(song.Pages))
	}
	if song.Pages[0].PageRasterRef != song.Pages[1].PageRasterRef {
		t.Fatalf("byte-identical rasters must share ONE stored image; got %q vs %q", song.Pages[0].PageRasterRef, song.Pages[1].PageRasterRef)
	}
	if song.Pages[0].RasterHash != song.Pages[1].RasterHash {
		t.Fatalf("identical rasters must hash equal")
	}
	l0, l1 := overlayLayerIDs(song.Pages[0]), overlayLayerIDs(song.Pages[1])
	if len(l0) != 1 || len(l1) != 1 || l0[0] == l1[0] {
		t.Fatalf("entries must keep DISTINCT overlays, got %v vs %v", l0, l1)
	}
}

func assertSeq(t *testing.T, got []int32, want ...int32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("sequence %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("sequence %v, want %v", got, want)
		}
	}
}

func overlayLayerIDs(p PageImages) []string {
	out := make([]string, 0, len(p.Overlays))
	for _, o := range p.Overlays {
		out = append(out, o.LayerID)
	}
	return out
}

package bake

import (
	"context"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// TestBakeCropChangesRasterHashAndRecordsSidecar is the end-to-end stage-2 pin (Fable): cropping a file
// through the real Bake changes that page's raster hash — so a rehearsal note drawn on the uncropped page
// reads pageChanged (the hashes no longer match) — AND the crop is recorded in the page-sources sidecar
// (never the bundle). The pre-crop background stays served by hash across revs (T186).
func TestBakeCropChangesRasterHashAndRecordsSidecar(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo)
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatal(err)
	}
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte(contentPDF))
	if err != nil {
		t.Fatal(err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatal(err)
	}

	// ONE baker / bakes dir: successive bakes mint successive revs of the same concert, so PageRasterByHash
	// can still find the pre-crop raster after the crop.
	b := &Baker{
		svc: svc, eng: eng,
		raster:   popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 150},
		overlays: fakeOverlays{png: tinyPNG(t, 10, 14)},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	// Rev 1: whole page.
	cb1, _, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil, "")
	if err != nil {
		t.Fatalf("bake 1: %v", err)
	}
	h1 := cb1.Songs[0].Pages[0].RasterHash
	sm1, ok := b.readSourceMap(cb1.ConcertID, cb1.ConcertRev)
	if !ok || sm1.Songs[song.ID][0].Crop != nil {
		t.Fatalf("uncropped bake should record no crop in the sidecar, got %+v", sm1.Songs[song.ID][0].Crop)
	}

	// Set a crop on the file directly (the save path is stage 3; here we exercise the bake side).
	f, err := repo.GetSongFile(file.ID)
	if err != nil {
		t.Fatal(err)
	}
	f.Crop = &app.Crop{LockAR: true, Pages: map[string]app.CropRect{"0": {X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Source: "manual"}}}
	if err := repo.UpdateSongFile(f); err != nil {
		t.Fatal(err)
	}

	// Rev 2: cropped.
	cb2, _, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil, "")
	if err != nil {
		t.Fatalf("bake 2: %v", err)
	}
	h2 := cb2.Songs[0].Pages[0].RasterHash
	if h1 == h2 {
		t.Fatal("a crop must change the page's raster hash — otherwise a note on that page never reads pageChanged")
	}
	// The crop is recorded in the sidecar.
	ps := sm2Page0(t, b, cb2)
	if ps.Crop == nil || ps.Crop.W != 0.5 || ps.Crop.X != 0.25 {
		t.Fatalf("crop not recorded in page-sources: %+v", ps.Crop)
	}
	// T186: the pre-crop background (h1) is STILL served, by hash across revs.
	if _, ok := b.PageRasterByHash(cb2.ConcertID, cb2.ConcertRev, song.ID, h1); !ok {
		t.Error("the pre-crop background (h1) is no longer served after the crop — T186 broken")
	}
}

// TestBakeCropThenResetIsByteIdentical pins P1 (non-destructive) through the REAL save path: a crop set via
// the service changes the page's raster hash, and a Reset (an empty crop via the SAME service call) bakes a
// page byte-identical to the original uncropped page — the PDF was never touched. This is the full
// user-facing cycle (SetFileCrop, not a hand-set repo field), so it also proves the service persists a crop
// the baker then honours.
func TestBakeCropThenResetIsByteIdentical(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo)
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatal(err)
	}
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte(contentPDF))
	if err != nil {
		t.Fatal(err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatal(err)
	}

	b := &Baker{
		svc: svc, eng: eng,
		raster:   popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 150},
		overlays: fakeOverlays{png: tinyPNG(t, 10, 14)},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	bakeHash := func() string {
		cb, _, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil, "")
		if err != nil {
			t.Fatalf("bake: %v", err)
		}
		return cb.Songs[0].Pages[0].RasterHash
	}

	h0 := bakeHash() // uncropped

	if _, err := svc.SetFileCrop(admin, band.ID, song.ID, file.ID, app.Crop{Pages: map[string]app.CropRect{
		"0": {X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Source: "manual"},
	}}); err != nil {
		t.Fatalf("set crop: %v", err)
	}
	hCrop := bakeHash() // cropped
	if hCrop == h0 {
		t.Fatal("a crop set through the service must change the page's raster hash")
	}

	if _, err := svc.SetFileCrop(admin, band.ID, song.ID, file.ID, app.Crop{}); err != nil {
		t.Fatalf("reset crop: %v", err)
	}
	hReset := bakeHash() // reset → whole page again
	if hReset != h0 {
		t.Fatalf("a Reset must bake byte-identically to the uncropped page (P1): h0=%s reset=%s", h0, hReset)
	}
}

// bakeLookupShim is the test twin of httpapi.bakePageLookup: it gives app.Service a BakeLookup over a real
// Baker using only the Baker's exported methods, so a bake-package test can exercise pageChanged end-to-end
// without importing httpapi (which would be a cycle).
type bakeLookupShim struct{ b *Baker }

func (l bakeLookupShim) PageRasterHash(concertID, songID string, pageInSong int) (string, bool, bool) {
	for _, cb := range l.b.ListConcerts() {
		if cb.ConcertID != concertID {
			continue
		}
		for _, s := range cb.Songs {
			if s.SongID != songID {
				continue
			}
			if pageInSong < 0 || pageInSong >= len(s.Pages) {
				return "", false, true
			}
			return s.Pages[pageInSong].RasterHash, true, true
		}
		return "", false, true
	}
	return "", false, false
}

func (l bakeLookupShim) ResolvePageSource(concertID string, rev uint64, songID, hash string) (string, int, bool) {
	return l.b.ResolvePageSource(concertID, rev, songID, hash)
}

func (l bakeLookupShim) PageRasterByHash(concertID string, rev uint64, songID, hash string) ([]byte, bool) {
	return l.b.PageRasterByHash(concertID, rev, songID, hash)
}

// TestBakeCropMakesNotePageChanged is Fable's stage-3 end-to-end assertion: a REAL rehearsal note on a baked
// page reads pageChanged==true once the page is re-cropped THROUGH THE SERVICE, and the note's own captured
// background is still served (the T186 viewer keeps working). This is the honest existing behaviour — a
// re-crop re-rasters the page, the note's pin no longer matches — pinned so a future change can't silently
// start re-placing notes.
func TestBakeCropMakesNotePageChanged(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo)
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))

	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(admin, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(admin, band.ID, "Song", "")
	if err != nil {
		t.Fatal(err)
	}
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte(contentPDF))
	if err != nil {
		t.Fatal(err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatal(err)
	}

	b := &Baker{
		svc: svc, eng: eng,
		raster:   popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 150},
		overlays: fakeOverlays{png: tinyPNG(t, 10, 14)},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	svc.WithBakeLookup(bakeLookupShim{b: b})

	// Bake uncropped, then pin a real note to page 0's captured raster.
	cb1, _, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil, "")
	if err != nil {
		t.Fatalf("bake 1: %v", err)
	}
	h0 := cb1.Songs[0].Pages[0].RasterHash
	if _, err := svc.PutRehearsalNote(admin, band.ID, song.ID, 0, app.RehearsalNoteMeta{
		RasterHash: h0, ConcertID: cb1.ConcertID, ConcertRev: cb1.ConcertRev,
	}, tinyPNG(t, 8, 8), false); err != nil {
		t.Fatalf("put note: %v", err)
	}

	pageChanged := func() *bool {
		views, err := svc.ListRehearsalNotes(admin, band.ID, song.ID)
		if err != nil {
			t.Fatalf("list notes: %v", err)
		}
		if len(views) != 1 {
			t.Fatalf("want 1 note, got %d", len(views))
		}
		return views[0].PageChanged
	}

	// Before any crop the note is on the current page.
	if pc := pageChanged(); pc == nil || *pc {
		t.Fatalf("a fresh note must read pageChanged=false, got %v", pc)
	}

	// Re-crop through the service, then re-bake: the page re-rasters.
	if _, err := svc.SetFileCrop(admin, band.ID, song.ID, file.ID, app.Crop{Pages: map[string]app.CropRect{
		"0": {X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Source: "manual"},
	}}); err != nil {
		t.Fatalf("set crop: %v", err)
	}
	if _, _, err := b.Bake(context.Background(), band.ID, sl.ID, admin, nil, ""); err != nil {
		t.Fatalf("bake 2: %v", err)
	}

	// The note now reads pageChanged (its pin no longer matches the re-rastered page)...
	if pc := pageChanged(); pc == nil || !*pc {
		t.Fatalf("after a re-crop the note must read pageChanged=true, got %v", pc)
	}
	// ...but its OWN captured background is still served (T186), so the viewer keeps working.
	if _, err := svc.RehearsalNoteBackground(admin, band.ID, song.ID, 0); err != nil {
		t.Fatalf("the note's captured background must still be served after the re-crop (T186): %v", err)
	}
}

func sm2Page0(t *testing.T, b *Baker, cb ConcertBundle) PageSource {
	t.Helper()
	sm, ok := b.readSourceMap(cb.ConcertID, cb.ConcertRev)
	if !ok {
		t.Fatal("no sidecar for the cropped rev")
	}
	srcs := sm.Songs[cb.Songs[0].SongID]
	if len(srcs) == 0 {
		t.Fatal("no page sources for the song")
	}
	return srcs[0]
}

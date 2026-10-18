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

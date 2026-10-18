package bake

import (
	"bytes"
	"context"
	"errors"
	"image"
	"math"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// TestCropPreviewPage renders one page under a draft crop (real poppler + a fake overlay worker) and checks
// the composite is a valid PNG, that a crop changes it vs the whole page, and that a generated chart is
// refused (P4). The composeLayers scaling itself is unit-tested in compose_test.go.
func TestCropPreviewPage(t *testing.T) {
	repo := memrepo.New()
	svc := app.NewService(repo).WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, _ := svc.CreateBand(admin, "Band")
	song, _ := svc.CreateSong(admin, band.ID, "Song", "")
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "score.pdf", "application/pdf", []byte(contentPDF))
	if err != nil {
		t.Fatal(err)
	}
	// A mark on page 0 so the overlay path runs.
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindLayerCreate, AuthorID: admin.ID, Layer: &domain.Layer{
		ID: "L1", FileID: file.ID, Name: "Marks", OwnerID: domain.SharedOwner, Zone: domain.ZoneShared, Access: domain.AccessRW,
	}}); err != nil {
		t.Fatal(err)
	}
	o := domain.Object{UUID: "m1", LayerID: "L1", Page: 0, Type: domain.TypeRect, Version: 1,
		Points: []domain.Point{{X: 0.4, Y: 0.4}, {X: 0.5, Y: 0.5}}, Style: domain.Style{Width: 0.01}}
	if _, err := eng.Apply(song.ID, domain.Mutation{Kind: domain.KindCreate, UUID: "m1", Object: &o, AuthorID: admin.ID}); err != nil {
		t.Fatal(err)
	}

	poppler := popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 150}
	b := &Baker{svc: svc, eng: eng, raster: poppler, preview: poppler, overlays: fakeOverlays{png: tinyPNG(t, 10, 14)}, now: func() int64 { return 1 }}
	all := func(string) bool { return true }
	ctx := context.Background()

	decode := func(png []byte) image.Image {
		t.Helper()
		img, _, derr := image.Decode(bytes.NewReader(png))
		if derr != nil {
			t.Fatalf("preview is not a decodable image: %v", derr)
		}
		return img
	}

	// Whole page (no crop).
	whole, err := b.CropPreviewPage(ctx, band.ID, song.ID, file.ID, admin, 0, app.Crop{}, all)
	if err != nil {
		t.Fatalf("whole-page preview: %v", err)
	}
	wimg := decode(whole)
	if wimg.Bounds().Dx() <= 0 {
		t.Fatal("whole-page preview has no pixels")
	}

	// Cropped page — a different image from the whole page.
	crop := app.Crop{LockAR: true, Pages: map[string]app.CropRect{"0": {X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Source: "manual"}}}
	cropped, err := b.CropPreviewPage(ctx, band.ID, song.ID, file.ID, admin, 0, crop, all)
	if err != nil {
		t.Fatalf("cropped preview: %v", err)
	}
	decode(cropped)
	if bytes.Equal(whole, cropped) {
		t.Fatal("a crop must change the preview image")
	}

	// P4: a generated chart cannot be previewed.
	chart, err := svc.CreateTextChart(admin, band.ID, song.ID, "# T\n\n## V\nline\n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := b.CropPreviewPage(ctx, band.ID, song.ID, chart.ID, admin, 0, app.Crop{}, all); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("generated chart preview must be refused (P4), got %v", err)
	}
}

// previewBaker builds a baker (real poppler) + an uploaded contentPDF, for the parity/validation tests.
func previewBaker(t *testing.T) (*Baker, app.User, string, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New()).WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	admin, _ := svc.Register("a", "A", "password123", "")
	band, _ := svc.CreateBand(admin, "B")
	song, _ := svc.CreateSong(admin, band.ID, "S", "")
	file, err := svc.UploadSongFile(admin, band.ID, song.ID, "s.pdf", "application/pdf", []byte(contentPDF))
	if err != nil {
		t.Fatal(err)
	}
	poppler := popplerRasterizer{bin: "pdftoppm", imagesBin: "pdfimages", dpi: 150}
	b := &Baker{svc: svc, eng: eng, raster: poppler, preview: poppler, overlays: fakeOverlays{png: tinyPNG(t, 8, 8)}, now: func() int64 { return 1 }}
	return b, admin, band.ID, song.ID, file.ID
}

// TestPreviewParity: the preview RASTER of a page (no marks) is byte-identical to the bake's staged raster for
// that page + crop — both cropped and uncropped. previewPage is a parallel path to the bake's Rasterize crop
// pass, so this byte-parity is what makes "preview == tablet" true. Red if previewPage skips the native cap or
// the greyscale (verified by sabotage).
func TestPreviewParity(t *testing.T) {
	b, _, _, _, fileID := previewBaker(t)
	_, pdf, err := b.svc.DownloadSongFile(app.User{}, fileID) // blob read; the user is unused by the mem blob store
	if err != nil {
		// DownloadSongFile gates by membership; fetch via the bake-side accessor instead.
		_, pdf, err = b.svc.SongFileBytesForBake(fileID)
		if err != nil {
			t.Fatal(err)
		}
	}
	ctx := context.Background()

	// Uncropped page 0: preview whole == bake whole.
	wholeBake, err := b.raster.Rasterize(ctx, pdf)
	if err != nil {
		t.Fatal(err)
	}
	wholePreview, err := b.preview.previewPage(ctx, pdf, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wholeBake[0], wholePreview) {
		t.Fatalf("uncropped preview raster differs from the bake (%d vs %d bytes)", len(wholeBake[0]), len(wholePreview))
	}

	// Cropped page 0: preview cropped == bake cropped (same rect via the context).
	rect := cropRect{x: 0.25, y: 0.25, w: 0.5, h: 0.5}
	crops := pageCrops{0: rect}
	cropBake, err := b.raster.Rasterize(withPageCrops(ctx, crops), pdf)
	if err != nil {
		t.Fatal(err)
	}
	cropPreview, err := b.preview.previewPage(ctx, pdf, 0, &rect)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cropBake[0], cropPreview) {
		t.Fatalf("cropped preview raster differs from the bake (%d vs %d bytes)", len(cropBake[0]), len(cropPreview))
	}
}

// TestCropPreview_ValidatesDraft: a non-square rect (under policy A) and a non-finite rect are refused with
// ErrInvalidInput → 400, rather than reaching the renderer raw.
func TestCropPreview_ValidatesDraft(t *testing.T) {
	b, admin, bandID, songID, fileID := previewBaker(t)
	all := func(string) bool { return true }
	ctx := context.Background()
	nonSquare := app.Crop{Pages: map[string]app.CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.5, Source: "manual"}}}
	if _, err := b.CropPreviewPage(ctx, bandID, songID, fileID, admin, 0, nonSquare, all); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("non-square draft must be 400, got %v", err)
	}
	nan := app.Crop{Pages: map[string]app.CropRect{"0": {X: math.NaN(), Y: 0.1, W: 0.5, H: 0.5, Source: "manual"}}}
	if _, err := b.CropPreviewPage(ctx, bandID, songID, fileID, admin, 0, nan, all); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("NaN draft must be 400, got %v", err)
	}
}

package app_test

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
)

func cropSvc(t *testing.T) (*app.Service, app.User, string, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New()).WithBlobStore(blob.NewMem())
	u, err := svc.Register("alice", "Alice", "pw123456", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "S", "")
	if err != nil {
		t.Fatal(err)
	}
	f, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture"))
	if err != nil {
		t.Fatal(err)
	}
	return svc, u, band.ID, song.ID, f.ID
}

func TestSetFileCrop(t *testing.T) {
	svc, u, bandID, songID, fileID := cropSvc(t)

	// A valid crop persists, with LockAR forced on, Auto forced off, and the source defaulted to manual.
	crop := app.Crop{Auto: true, Pages: map[string]app.CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}}
	f, err := svc.SetFileCrop(u, bandID, songID, fileID, crop)
	if err != nil {
		t.Fatalf("valid crop rejected: %v", err)
	}
	if f.Crop == nil || !f.Crop.LockAR || f.Crop.Auto || f.Crop.Pages["0"].Source != "manual" {
		t.Fatalf("crop not persisted as expected (LockAR on, Auto off, source manual): %+v", f.Crop)
	}

	// Reset: an empty crop clears it.
	f, err = svc.SetFileCrop(u, bandID, songID, fileID, app.Crop{})
	if err != nil {
		t.Fatalf("reset rejected: %v", err)
	}
	if f.Crop != nil {
		t.Fatalf("reset should clear the crop, got %+v", f.Crop)
	}

	// Invalid geometry refused.
	if _, err := svc.SetFileCrop(u, bandID, songID, fileID, app.Crop{Pages: map[string]app.CropRect{"0": {X: 0.9, Y: 0, W: 0.8, H: 0.8}}}); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("out-of-page crop must be refused, got %v", err)
	}
}

func TestSetFileCrop_RefusesGeneratedChart(t *testing.T) {
	svc, u, bandID, songID, _ := cropSvc(t)
	chart, err := svc.CreateTextChart(u, bandID, songID, "# Title\n\n## Verse\nline\n")
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.SetFileCrop(u, bandID, songID, chart.ID, app.Crop{Pages: map[string]app.CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8, Source: "manual"}}})
	if !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("a crop on a generated chart must be refused (P4), got %v", err)
	}
}

// TestSetFileCrop_ScopeGuards pins that the crop save cannot reach across a band/song boundary or be made by
// a non-member: removing `f.BandID != bandID || f.SongID != songID` (cross-scope) or disabling the GetBand
// member gate must turn one of these red. Each case also asserts the crop never persisted.
func TestSetFileCrop_ScopeGuards(t *testing.T) {
	svc, owner, bandID, songID, fileID := cropSvc(t)
	validRect := map[string]app.CropRect{"0": {X: 0.1, Y: 0.1, W: 0.8, H: 0.8}}

	// A SECOND band the owner also belongs to, and a second song in the first band.
	band2, err := svc.CreateBand(owner, "Band2")
	if err != nil {
		t.Fatal(err)
	}
	song2, err := svc.CreateSong(owner, bandID, "S2", "")
	if err != nil {
		t.Fatal(err)
	}
	// A non-member of the first band.
	stranger, err := svc.Register("mallory", "Mallory", "pw123456", "")
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name       string
		caller     app.User
		band, song string
		wantErr    error
	}{
		{"cross-band: file belongs to another band", owner, band2.ID, songID, app.ErrNotFound},
		{"cross-song: file belongs to another song", owner, bandID, song2.ID, app.ErrNotFound},
		{"non-member: caller is not in the band", stranger, bandID, songID, app.ErrForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.SetFileCrop(tc.caller, tc.band, tc.song, fileID, app.Crop{Pages: validRect})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("got %v, want %v", err, tc.wantErr)
			}
			files, gerr := svc.SongFiles(owner, bandID, songID)
			if gerr != nil {
				t.Fatal(gerr)
			}
			for _, f := range files {
				if f.ID == fileID && f.Crop != nil {
					t.Fatalf("a refused save must not persist a crop, got %+v", f.Crop)
				}
			}
		})
	}
}

func TestCropHiddenMarks(t *testing.T) {
	// One layer on the file; two marks on the cropped page 0 (one inside the crop, one outside), one mark on
	// an uncropped page 1.
	snap := domain.Snapshot{
		Layers: []domain.Layer{{ID: "L1", FileID: "F1"}},
		Objects: []domain.Object{
			{UUID: "inside", LayerID: "L1", Page: 0, Type: domain.TypeRect, Points: []domain.Point{{X: 0.4, Y: 0.4}, {X: 0.5, Y: 0.5}}, Style: domain.Style{Width: 0.01}},
			{UUID: "outside", LayerID: "L1", Page: 0, Type: domain.TypeRect, Points: []domain.Point{{X: 0.95, Y: 0.95}}, Style: domain.Style{Width: 0.01}},
			{UUID: "otherpage", LayerID: "L1", Page: 1, Type: domain.TypeRect, Points: []domain.Point{{X: 0.95, Y: 0.95}}, Style: domain.Style{Width: 0.01}},
			// An empty-points mark on the cropped page: the bake KEEPS it (no extent to place off the page),
			// so the warning must NOT report it hidden — the empty case lives in CropHidesMark so both agree.
			{UUID: "emptypts", LayerID: "L1", Page: 0, Type: domain.TypeRect, Points: nil, Style: domain.Style{Width: 0.01}},
		},
	}
	crop := app.Crop{Pages: map[string]app.CropRect{"0": {X: 0.25, Y: 0.25, W: 0.5, H: 0.5, Source: "manual"}}}
	hidden := app.CropHiddenMarks(snap, "F1", crop)
	if len(hidden) != 1 || hidden[0].UUID != "outside" {
		t.Fatalf("hidden marks = %+v, want just [outside] (empty-points mark must not be reported hidden)", hidden)
	}
}

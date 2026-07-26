package app_test

import (
	"errors"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/chartpdf"
)

// TestBakeTransposeSucceeds_D3: an eligible chart that transposes fine reports success; when
// the transform errors at bake (injected), it reports FAILURE — the signal the bake-warning
// path turns into "chords not transposed at bake" instead of a silent untransposed page (D3).
func TestBakeTransposeSucceeds_D3(t *testing.T) {
	svc := app.NewService(memrepo.New()).WithBlobStore(blob.NewMem())
	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "Stand By Me", "")
	if err != nil {
		t.Fatal(err)
	}
	key := "C"
	if _, err := svc.UpdateSong(u, band.ID, song.ID, app.SongPatch{Key: &key}); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateTextChart(u, band.ID, song.ID, "# Stand By Me\n## Verse\nC            G\nlyrics\n"); err != nil {
		t.Fatal(err)
	}

	// Eligible + a valid transform → success.
	if !svc.BakeTransposeSucceeds(u, band.ID, song.ID, "D") {
		t.Fatal("eligible chart should transpose+render successfully")
	}
	// Force the transform to error at bake → the check reports failure (drives the D3 warning).
	svc.WithBakeTransposeFunc(func(string, chartpdf.Key, chartpdf.Key) (string, error) {
		return "", errors.New("boom")
	})
	if svc.BakeTransposeSucceeds(u, band.ID, song.ID, "D") {
		t.Fatal("a failed transform must report failure so the warning surfaces")
	}
}

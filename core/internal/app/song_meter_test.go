package app_test

import (
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// UpdateSong stores a valid metre canonically and, per T86's leniency, turns anything
// malformed into unset ("") rather than failing the save.
func TestUpdateSong_MeterLenient(t *testing.T) {
	svc := app.NewService(memrepo.New())
	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatal(err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatal(err)
	}
	song, err := svc.CreateSong(u, band.ID, "Amazing Grace", "")
	if err != nil {
		t.Fatal(err)
	}

	set := func(m string) app.Song {
		s, uerr := svc.UpdateSong(u, band.ID, song.ID, app.SongPatch{Meter: &m})
		if uerr != nil {
			t.Fatalf("UpdateSong(meter=%q) errored: %v (a metre typo must never fail a save)", m, uerr)
		}
		return s
	}

	if got := set("3/4").Meter; got != "3/4" {
		t.Errorf("valid metre 3/4 → %q, want 3/4", got)
	}
	if got := set(" 6/8 ").Meter; got != "6/8" {
		t.Errorf("valid metre with spaces → %q, want 6/8", got)
	}
	if got := set("nonsense").Meter; got != "" {
		t.Errorf("malformed metre → %q, want unset (empty)", got)
	}
	if got := set("").Meter; got != "" {
		t.Errorf("cleared metre → %q, want empty", got)
	}
}

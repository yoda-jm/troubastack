package bake

import (
	"context"
	"testing"

	"troubastack/core/internal/app"
)

// effectiveTempo/effectiveKey: the setlist override wins when set, else the song's base
// value (T86). Teeth: removing the fallback in baker.go makes the no-override cases 0/"".
func TestEffectiveTempoKey_T86(t *testing.T) {
	base := app.SetlistItemView{SongTempo: 92, SongKey: "G"}
	if got := effectiveTempo(base); got != 92 {
		t.Errorf("no override → tempo %d, want base 92", got)
	}
	if got := effectiveKey(base); got != "G" {
		t.Errorf("no override → key %q, want base G", got)
	}

	over := app.SetlistItemView{SongTempo: 92, SongKey: "G"}
	over.TempoOverride = 132
	over.KeyOverride = "A"
	if got := effectiveTempo(over); got != 132 {
		t.Errorf("override → tempo %d, want 132", got)
	}
	if got := effectiveKey(over); got != "A" {
		t.Errorf("override → key %q, want A", got)
	}
}

// End-to-end: a song's BASE tempo/key (no setlist override) must reach the baked bundle —
// the bug that baked tempo=0 for every non-overridden song, so the Stage beat never showed.
func TestBake_BaseTempoAndKeyReachBundle_T86(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	songs, err := svc.Songs(u, bandID)
	if err != nil || len(songs) == 0 {
		t.Fatalf("list songs: %v", err)
	}
	tempo, key := 100, "D"
	if _, err := svc.UpdateSong(u, bandID, songs[0].ID, app.SongPatch{Tempo: &tempo, Key: &key}); err != nil {
		t.Fatalf("set base metadata: %v", err)
	}
	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	cb, err := b.Bake(context.Background(), bandID, setlistID, u, nil)
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if len(cb.Songs) != 1 {
		t.Fatalf("want 1 baked song, got %d", len(cb.Songs))
	}
	if cb.Songs[0].Tempo != 100 {
		t.Errorf("baked tempo = %d, want the song's base 100 (no setlist override)", cb.Songs[0].Tempo)
	}
	if cb.Songs[0].Key != "D" {
		t.Errorf("baked key = %q, want the song's base D", cb.Songs[0].Key)
	}
}

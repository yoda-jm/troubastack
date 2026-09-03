package bake

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// P207: the song's artist rides into the baked bundle — a snapshot at bake time, exactly like the
// title (T26). An absent artist must behave exactly as today: omitted from bundle.json (omitempty, the
// same as the title), which the app defaults to "". Checked against the ACTUAL bundle.json, not just
// the Go struct (the done-when).
func TestBake_ArtistReachesBundle_P207(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	songs, err := svc.Songs(u, bandID)
	if err != nil || len(songs) == 0 {
		t.Fatalf("list songs: %v", err)
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
	readBundle := func(rev string) (ConcertBundle, string) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(b.bakesDir, setlistID, rev, "bundle.json"))
		if err != nil {
			t.Fatalf("read bundle.json rev %s: %v", rev, err)
		}
		var parsed ConcertBundle
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("bundle.json rev %s parse: %v", rev, err)
		}
		return parsed, string(data)
	}

	// (1) The seed song was created with no artist. The bundle carries none, and — like the title's
	// omitempty — the "artist" key is ABSENT from bundle.json (an old app defaults it to "").
	cb, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake (no artist): %v", err)
	}
	if cb.Songs[0].Artist != "" {
		t.Errorf("no-artist song: baked artist = %q, want empty", cb.Songs[0].Artist)
	}
	parsed, raw := readBundle("1")
	if parsed.Songs[0].Artist != "" {
		t.Errorf("no-artist bundle.json artist = %q, want empty", parsed.Songs[0].Artist)
	}
	if strings.Contains(raw, `"artist"`) {
		t.Errorf("bundle.json must OMIT the artist key when empty (omitempty, like title), got:\n%s", raw)
	}

	// (2) Give the song an artist → it reaches the bundle, in the actual bundle.json.
	artist := "Placeholder Artist"
	if _, err := svc.UpdateSong(u, bandID, songs[0].ID, app.SongPatch{Artist: &artist}); err != nil {
		t.Fatalf("set artist: %v", err)
	}
	cb, _, err = b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake (with artist): %v", err)
	}
	if cb.Songs[0].Artist != artist {
		t.Errorf("baked artist = %q, want %q", cb.Songs[0].Artist, artist)
	}
	parsed, raw = readBundle("2")
	if parsed.Songs[0].Artist != artist {
		t.Errorf("bundle.json artist = %q, want %q", parsed.Songs[0].Artist, artist)
	}
	if !strings.Contains(raw, `"artist"`) || !strings.Contains(raw, artist) {
		t.Errorf("bundle.json missing the artist key/value %q:\n%s", artist, raw)
	}
}

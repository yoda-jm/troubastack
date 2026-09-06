package bake

import (
	"context"
	"testing"
)

// T153 ⟨R1⟩ — a setlist of song–intermission–song bakes to THREE entries, the middle one an intermission
// carrying exactly one page and no overlays, with the songs around it keeping their positions. This is the
// baker slice: it replaces slice 1's deliberate refusal (the old TestBake_Refuses… test) with the render.
//
// Teeth on "absent ⇒ song": a real song's BakedSong.Kind stays EMPTY (not "song"), so the field is
// additive exactly as fields 5–14 are — a bundle baked before T153 reads as all-songs. If stageSong ever
// stamped Kind:"song" this test goes red.
func TestBake_IntermissionBakesAsOneUnnumberedPageBetweenSongs_T153(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t) // setlist starts as [song1]
	png := tinyPNG(t, 40, 60)

	// A break after song1, then a second song after the break ⇒ [song1, intermission, song2] (both adders
	// append at the end).
	if _, err := svc.AddSetlistIntermission(u, bandID, setlistID, "Entracte"); err != nil {
		t.Fatalf("add intermission: %v", err)
	}
	song2, err := svc.CreateSong(u, bandID, "Closer", "")
	if err != nil {
		t.Fatalf("create song2: %v", err)
	}
	if _, err := svc.UploadSongFile(u, bandID, song2.ID, "score2.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload song2 file: %v", err)
	}
	if _, err := svc.AddSetlistItem(u, bandID, setlistID, song2.ID); err != nil {
		t.Fatalf("add song2: %v", err)
	}

	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	bundle, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}

	if len(bundle.Songs) != 3 {
		t.Fatalf("baked %d entries, want 3 (song, intermission, song)", len(bundle.Songs))
	}

	// Order + kind: the flanking entries are songs (Kind absent), the middle is the break.
	if k := bundle.Songs[0].Kind; k != "" {
		t.Errorf("entry 0 (a song) Kind = %q, want \"\" — the field must stay additive (absent ⇒ song)", k)
	}
	if k := bundle.Songs[2].Kind; k != "" {
		t.Errorf("entry 2 (a song) Kind = %q, want \"\"", k)
	}
	brk := bundle.Songs[1]
	if brk.Kind != "intermission" {
		t.Fatalf("middle entry Kind = %q, want \"intermission\"", brk.Kind)
	}

	// The break: its authored label, no song, exactly one page, no overlays.
	if brk.Label != "Entracte" {
		t.Errorf("break Label = %q, want %q", brk.Label, "Entracte")
	}
	if brk.SongID != "" {
		t.Errorf("break SongID = %q, want empty (a break has no song)", brk.SongID)
	}
	if len(brk.Pages) != 1 {
		t.Fatalf("break has %d pages, want exactly 1", len(brk.Pages))
	}
	if n := len(brk.Pages[0].Overlays); n != 0 {
		t.Errorf("break page carries %d overlays, want 0 (a break has nothing to draw)", n)
	}

	// The flanking songs are intact: real ids and at least their page.
	for _, i := range []int{0, 2} {
		if bundle.Songs[i].SongID == "" {
			t.Errorf("entry %d should be a real song with a SongID", i)
		}
		if len(bundle.Songs[i].Pages) == 0 {
			t.Errorf("entry %d (a song) has no pages", i)
		}
	}
}

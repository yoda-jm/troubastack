package bake

import (
	"context"
	"strings"
	"testing"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
)

// P206 Stage 3 through the REAL bake, not the pure resolver: a jump the studio authored has to arrive in
// the bundle as a hotspot on the source's page. jumps_test.go proves the resolution; this proves the
// seam — that stageSong carries the (reprojected) objects, that assembleSong resolves them where a page's
// pool index is decided, and that the field reaches ConcertBundle.
func jumpPair(t *testing.T, eng *engine.Engine, songID, userID string, destPage int, destUUID, srcUUID string) {
	t.Helper()
	add := func(o domain.Object) {
		if _, err := eng.Apply(songID, domain.Mutation{Kind: domain.KindCreate, UUID: o.UUID, AuthorID: userID, Object: &o}); err != nil {
			t.Fatalf("create %s: %v", o.UUID, err)
		}
	}
	add(domain.Object{
		UUID: destUUID, LayerID: "L1", Type: domain.TypeIcon, Page: destPage, Version: 1, Text: "segno",
		Points: []domain.Point{{X: 0.40, Y: 0.25}, {X: 0.48, Y: 0.31}},
		Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
	})
	add(domain.Object{
		UUID: srcUUID, LayerID: "L1", Type: domain.TypeIcon, Page: 0, Version: 1, Text: "segno",
		Points: []domain.Point{{X: 0.10, Y: 0.60}, {X: 0.18, Y: 0.66}},
		Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
		JumpTo: destUUID,
	})
}

func TestBake_JumpPairBecomesAPageJump(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	songID := ""
	{
		detail, err := svc.Setlist(u, bandID, setlistID)
		if err != nil {
			t.Fatalf("setlist: %v", err)
		}
		songID = detail.Items[0].SongID
	}
	jumpPair(t, eng, songID, u.ID, 1, "dest-1", "src-1") // destination on page 2 of 2

	pngBytes := tinyPNG(t, 40, 56)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 2, png: pngBytes},
		overlays: fakeOverlays{png: pngBytes},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	cb, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	pages := cb.Songs[0].Pages
	if len(pages) != 2 {
		t.Fatalf("want 2 pages, got %d", len(pages))
	}
	if len(pages[0].Jumps) != 1 {
		t.Fatalf("want the jump on the SOURCE's page, got %d jumps on page 1 / %d on page 2", len(pages[0].Jumps), len(pages[1].Jumps))
	}
	if len(pages[1].Jumps) != 0 {
		t.Fatalf("the destination page carries no jump of its own, got %+v", pages[1].Jumps)
	}
	j := pages[0].Jumps[0]
	if j.TargetPage != 1 {
		t.Fatalf("target page = %d, want 1 (the destination's page in this song)", j.TargetPage)
	}
	if j.TargetAnchorYPermille != 250 {
		t.Fatalf("anchor = %d, want the destination landmark's top (250)", j.TargetAnchorYPermille)
	}
	if j.X0Permille != 100 || j.Y0Permille != 600 {
		t.Fatalf("hotspot origin = %d,%d, want the source landmark's (100,600)", j.X0Permille, j.Y0Permille)
	}
	if j.LayerID != "L1" || j.Owner != u.ID {
		t.Fatalf("layer/owner = %q/%q, want L1 / the layer owner — the two visibility filters (A1/P205)", j.LayerID, j.Owner)
	}
}

// Fable, ⟨GO⟩ ef24ec4c: the bake WILL receive dangling pointers (songs authored before the studio swept
// them, and survivors on a layer the deleting user could not edit). It must drop the jump, keep the ink,
// and not fail — and the admin must be told, on the same record T60's warnings use.
func TestBake_DanglingJumpDropsTheJumpKeepsTheInkAndWarns(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	detail, err := svc.Setlist(u, bandID, setlistID)
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	songID := detail.Items[0].SongID
	jumpPair(t, eng, songID, u.ID, 0, "dest-2", "src-2")
	// The destination is deleted — the state the studio's sweep cannot reach on someone else's layer.
	if _, err := eng.Apply(songID, domain.Mutation{Kind: domain.KindDelete, UUID: "dest-2", AuthorID: u.ID, BaseVersion: 1}); err != nil {
		t.Fatalf("delete destination: %v", err)
	}

	pngBytes := tinyPNG(t, 40, 56)
	b := &Baker{
		svc: svc, eng: eng,
		raster:   fakeRaster{pages: 1, png: pngBytes},
		overlays: fakeOverlays{png: pngBytes},
		bakesDir: t.TempDir(),
		progress: newProgressRegistry(nil),
		now:      func() int64 { return 1700000000 },
	}
	cb, bakeID, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("a dangling jump must NOT fail the bake: %v", err)
	}
	if got := len(cb.Songs[0].Pages[0].Jumps); got != 0 {
		t.Fatalf("want the jump dropped, got %d", got)
	}
	// The ink is untouched — the source landmark still renders as the icon it is.
	if got := len(cb.Songs[0].Pages[0].Overlays); got == 0 {
		t.Fatalf("the source landmark's overlay must still be baked (drop the jump, keep the ink)")
	}
	prog, ok := b.Progress(bandID, setlistID, bakeID)
	if !ok {
		t.Fatalf("no progress record for the finished bake")
	}
	if prog.State != BakeSucceeded {
		t.Fatalf("bake state = %v, want succeeded", prog.State)
	}
	if len(prog.Warnings) != 1 || !strings.Contains(prog.Warnings[0], "Song") {
		t.Fatalf("want one warning naming the song on the terminal record, got %v", prog.Warnings)
	}
}

// The two warning producers must COEXIST: the bake publishes its own with the terminal state, and
// bakeapi appends T60's transpose list afterwards. Before P206 setWarnings replaced, so whichever wrote
// last silently ate the other's.
func TestSetWarnings_AppendsRatherThanReplaces(t *testing.T) {
	r := newProgressRegistry(nil)
	r.set("b1", "band", "sl", BakeProgress{State: BakeSucceeded, Warnings: []string{"a jump was dropped"}})
	r.setWarnings("b1", "band", "sl", []string{"a transpose warning"})
	got, ok := r.get("b1", "band", "sl")
	if !ok {
		t.Fatalf("record missing")
	}
	if len(got.Warnings) != 2 || got.Warnings[0] != "a jump was dropped" || got.Warnings[1] != "a transpose warning" {
		t.Fatalf("warnings = %v, want both producers' entries in order", got.Warnings)
	}
}

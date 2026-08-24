package bake

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// countingOverlays wraps fakeOverlays and counts overlay-worker SPAWNS (T97 §3.1 / T98: a spawn only
// happens for a NON-empty batch — the real renderer returns early with no process for an empty one, so
// the fake counts the same way). Proven by injection, not by timing.
type countingOverlays struct {
	fakeOverlays
	calls int
}

func (c *countingOverlays) RenderBatch(ctx context.Context, songs []overlaySong) (map[string][]renderedOverlay, error) {
	if len(songs) > 0 {
		c.calls++
	}
	return c.fakeOverlays.RenderBatch(ctx, songs)
}

// T97 — a bake of a setlist with no annotations must not start the overlay worker at all.
func TestBake_NoAnnotations_ZeroOverlaySpawns(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, _ := svc.Register("admin", "Admin", "password123", "")
	band, _ := svc.CreateBand(u, "Band")
	sl, _ := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	for i := 0; i < 3; i++ { // three songs, NONE annotated
		song, _ := svc.CreateSong(u, band.ID, "Song", "")
		if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
			t.Fatalf("upload: %v", err)
		}
		if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	png := tinyPNG(t, 40, 56)
	ov := &countingOverlays{fakeOverlays: fakeOverlays{png: png}}
	b := &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 1, png: png}, overlays: ov, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}

	if _, _, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil, ""); err != nil {
		t.Fatalf("bake: %v", err)
	}
	if ov.calls != 0 {
		t.Errorf("no-annotation bake spawned the overlay worker %d time(s); want 0 (T97)", ov.calls)
	}
}

// T97 byte-identity guard: the skip is only sound if the REAL overlay worker would itself have
// produced zero overlays for a doc with no objects. Prove it so the skip can't silently drop content.
// (Skips when node / the built CLI is absent.)
func TestOverlayRenderer_EmptyDoc_ZeroOverlays(t *testing.T) {
	cli := os.Getenv("TROUBA_BAKE_CLI")
	if cli == "" {
		cli = "../../../web/bake/dist/cli.js"
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("no node")
	}
	if _, err := os.Stat(cli); err != nil {
		t.Skip("bake CLI not built (" + cli + ")")
	}
	r := nodeOverlayRenderer{node: "node", cli: cli}
	// A doc with a LAYER but ZERO objects — the worst case for the skip's byte-identity claim.
	doc := annotationsDoc{
		Layers:  []docLayer{{ID: "L1", Order: 0}},
		Objects: []docObject{},
	}
	out, err := r.RenderBatch(context.Background(), []overlaySong{{Key: "s0", Doc: doc, Pages: []pageSize{{Index: 0, Width: 800, Height: 1000}}, OverlayWidth: 800}})
	if err != nil {
		t.Fatalf("render empty doc: %v", err)
	}
	if len(out["s0"]) != 0 {
		t.Errorf("overlay worker produced %d overlay(s) for a 0-object doc; the skip would change output", len(out["s0"]))
	}
}

// rectDoc is a one-layer, one-rect doc — enough for the worker to draw exactly one overlay.
func rectDoc(layerID string) annotationsDoc {
	return annotationsDoc{
		Layers: []docLayer{{ID: layerID, Order: 0}},
		Objects: []docObject{{
			UUID: "o-" + layerID, LayerID: layerID, Type: "rect", Page: 0,
			Points: []docPoint{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
			Style:  docStyle{Color: "#e11d48", Opacity: 1, Width: 0.004},
		}},
	}
}

// T98 — the whole point: a bake with N annotated songs spawns the overlay worker ONCE, not N times.
func TestBake_MultipleAnnotated_OneOverlaySpawn(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, _ := svc.Register("admin", "Admin", "password123", "")
	band, _ := svc.CreateBand(u, "Band")
	sl, _ := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	for i := 0; i < 3; i++ { // three songs, EACH annotated
		song, _ := svc.CreateSong(u, band.ID, "Song", "")
		if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
			t.Fatalf("upload: %v", err)
		}
		lid := "L" + string(rune('1'+i))
		if _, err := eng.Apply(song.ID, domain.Mutation{
			Kind:     domain.KindLayerCreate,
			Layer:    &domain.Layer{ID: lid, Name: "Marks", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
			AuthorID: u.ID,
		}); err != nil {
			t.Fatalf("layer: %v", err)
		}
		if _, err := eng.Apply(song.ID, domain.Mutation{
			Kind: domain.KindCreate, UUID: "o" + string(rune('1'+i)), AuthorID: u.ID,
			Object: &domain.Object{
				UUID: "o" + string(rune('1'+i)), LayerID: lid, Type: domain.TypeRect, Page: 0, Version: 1,
				Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}},
				Style:  domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004},
			},
		}); err != nil {
			t.Fatalf("object: %v", err)
		}
		if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	png := tinyPNG(t, 40, 56)
	ov := &countingOverlays{fakeOverlays: fakeOverlays{png: png}}
	b := &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 1, png: png}, overlays: ov, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}

	bundle, _, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if ov.calls != 1 {
		t.Errorf("3-annotated-song bake spawned the overlay worker %d time(s); want 1 (T98 batches them)", ov.calls)
	}
	// And every song still got its overlay (batching must not drop or cross-route content).
	for i, s := range bundle.Songs {
		if len(s.Pages) == 0 || len(s.Pages[0].Overlays) != 1 {
			t.Errorf("song %d: got %d page(s), want 1 page with 1 overlay", i, len(s.Pages))
		}
	}
}

// T98 byte-identity: batching must not change a song's per-song output vs rendering it alone. Render
// song A alone, then in a two-song batch, and assert A's overlay bytes are identical either way (and B
// is isolated). (Skips when node / the built CLI is absent.)
func TestOverlayRenderer_BatchMatchesIsolated(t *testing.T) {
	cli := os.Getenv("TROUBA_BAKE_CLI")
	if cli == "" {
		cli = "../../../web/bake/dist/cli.js"
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("no node")
	}
	if _, err := os.Stat(cli); err != nil {
		t.Skip("bake CLI not built (" + cli + ")")
	}
	r := nodeOverlayRenderer{node: "node", cli: cli}
	pages := []pageSize{{Index: 0, Width: 800, Height: 1000}}
	songA := overlaySong{Key: "s0", Doc: rectDoc("LA"), Pages: pages, OverlayWidth: 800}
	songB := overlaySong{Key: "s1", Doc: rectDoc("LB"), Pages: pages, OverlayWidth: 800}

	alone, err := r.RenderBatch(context.Background(), []overlaySong{songA})
	if err != nil {
		t.Fatalf("render A alone: %v", err)
	}
	both, err := r.RenderBatch(context.Background(), []overlaySong{songA, songB})
	if err != nil {
		t.Fatalf("render A+B: %v", err)
	}
	if len(alone["s0"]) != 1 || len(both["s0"]) != 1 || len(both["s1"]) != 1 {
		t.Fatalf("overlay counts: alone A=%d, batch A=%d, batch B=%d; want 1 each", len(alone["s0"]), len(both["s0"]), len(both["s1"]))
	}
	if a, b := alone["s0"][0].ContentHash, both["s0"][0].ContentHash; a != b {
		t.Errorf("song A overlay changed when batched: alone hash %s, batch hash %s", a, b)
	}
	if !bytes.Equal(alone["s0"][0].PNG, both["s0"][0].PNG) {
		t.Errorf("song A overlay PNG bytes differ alone vs batched")
	}
}

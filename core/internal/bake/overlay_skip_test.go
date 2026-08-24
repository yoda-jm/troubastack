package bake

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// countingOverlays wraps fakeOverlays and counts Render calls (T97 §3.1: a no-annotation bake must
// spawn ZERO overlay workers — proven by injection, not by timing).
type countingOverlays struct {
	fakeOverlays
	calls int
}

func (c *countingOverlays) Render(ctx context.Context, req cliRequest) ([]renderedOverlay, error) {
	c.calls++
	return c.fakeOverlays.Render(ctx, req)
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

	if _, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil); err != nil {
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
	out, err := r.Render(context.Background(), cliRequest{Doc: doc, Pages: []pageSize{{Index: 0, Width: 800, Height: 1000}}, OverlayWidth: 800})
	if err != nil {
		t.Fatalf("render empty doc: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("overlay worker produced %d overlay(s) for a 0-object doc; the skip would change output", len(out))
	}
}

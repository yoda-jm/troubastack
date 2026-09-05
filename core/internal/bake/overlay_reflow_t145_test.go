package bake

import (
	"context"
	"testing"
)

// pageOverlays emits each layer's overlay on a fixed page — page 0 is a normal mark; a page beyond the
// raster set simulates a mark whose page fell off the end of a reflowed render (the chart now has fewer
// pages than when the mark was drawn). That is the bake form of the T145 reflow bug: the overlay exists
// but points at a page the raster set no longer contains.
type pageOverlays struct {
	png  []byte
	page int
}

func (o pageOverlays) RenderBatch(_ context.Context, songs []overlaySong) (map[string][]renderedOverlay, error) {
	out := map[string][]renderedOverlay{}
	for _, s := range songs {
		var ovs []renderedOverlay
		for _, l := range s.Doc.Layers {
			ovs = append(ovs, renderedOverlay{
				Page: o.page, LayerID: l.ID, Order: int32(l.Order), Mandatory: l.Mandatory,
				RoleTag: l.RoleTag, ContentHash: Sha256Hex(o.png), PNG: o.png,
			})
		}
		out[s.Key] = ovs
	}
	return out, nil
}

// TestBake_ReflowOrphanedOverlay_FailsBake (T145): an overlay on a page the render no longer has must NOT
// be silently dropped from the bundle — assembleSong reads overlaysByPage only for pages that exist, so a
// reflow that shortened the chart would drop the mark with no signal ("one overlay vanished"). The bake
// must fail instead, so the orphan is fixed before the rehearsal, not discovered at it.
//
// Teeth: the SAME setup with the mark on page 0 (a real page) bakes fine, so the failure is the orphaned
// page specifically, not the harness. Red-first: without the guard, the orphan case SUCCEEDS (the page-1
// overlay is silently dropped) and the first assertion fails.
func TestBake_ReflowOrphanedOverlay_FailsBake(t *testing.T) {
	png := tinyPNG(t, 40, 56)

	t.Run("orphan page fails the bake", func(t *testing.T) {
		svc, eng, u, bandID, setlistID := seed(t)
		// chart renders ONE page (page 0); the mark is on page 1 → orphaned by reflow.
		b := &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 1, png: png}, overlays: pageOverlays{png: png, page: 1}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
		if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err == nil {
			t.Fatal("bake succeeded despite a mark on a page the chart does not have — a reflow orphan was silently dropped (T145)")
		}
	})

	t.Run("same mark on a real page bakes fine", func(t *testing.T) {
		svc, eng, u, bandID, setlistID := seed(t)
		b := &Baker{svc: svc, eng: eng, raster: fakeRaster{pages: 1, png: png}, overlays: pageOverlays{png: png, page: 0}, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 }}
		if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
			t.Fatalf("a mark on page 0 must bake cleanly, got: %v", err)
		}
	})
}

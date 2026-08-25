package bake

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// T107: a baked concert — bundle.json, page rasters, overlay PNGs and especially the .tstage (the band's
// entire repertoire, copyrighted sheet music) — is user content that is DESIGNED to leave the data dir
// (downloaded to tablets, copied to shares). The 0o700 dir shield does not travel with the file, so every
// baked file must itself be owner-only.
func TestBake_OutputsAreOwnerOnly(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t)
	pngBytes := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: pngBytes},
		overlays: fakeOverlays{png: pngBytes},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}
	if _, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, ""); err != nil {
		t.Fatalf("bake: %v", err)
	}

	concertDir := filepath.Join(b.bakesDir, setlistID)
	if err := filepath.Walk(concertDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if perm := info.Mode().Perm(); perm&0o077 != 0 {
			kind := "file"
			if info.IsDir() {
				kind = "dir"
			}
			t.Errorf("baked %s %s is group/world accessible: %04o", kind, path, perm)
		}
		return nil
	}); err != nil {
		t.Fatalf("walk: %v", err)
	}

	// Spot-check the two headline artefacts by name so the intent is explicit and the exact mode is pinned.
	for _, f := range []string{filepath.Join(concertDir, "1", "bundle.json"), filepath.Join(concertDir, "1.tstage")} {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("stat %s: %v", f, err)
		}
		if got := info.Mode().Perm(); got != 0o600 {
			t.Errorf("%s mode = %04o, want 0600", f, got)
		}
	}
}

package bake

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// T143: the owning band's identity rides into the baked bundle so the on-device library
// can GROUP downloaded bundles by band ("je ne sais pas … quel band"). The baker has the
// band id from the request and the name via GetBand. Checked against the ACTUAL
// bundle.json, not just the Go struct — the app reads the file.
func TestBake_BandIdentityReachesBundle_T143(t *testing.T) {
	svc, eng, u, bandID, setlistID := seed(t) // seed() creates a band named "Band"
	png := tinyPNG(t, 40, 56)
	b := &Baker{
		svc:      svc,
		eng:      eng,
		raster:   fakeRaster{pages: 1, png: png},
		overlays: fakeOverlays{png: png},
		bakesDir: t.TempDir(),
		now:      func() int64 { return 1700000000 },
	}

	cb, _, err := b.Bake(context.Background(), bandID, setlistID, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if cb.BandID != bandID {
		t.Errorf("baked BandID = %q, want %q", cb.BandID, bandID)
	}
	if cb.BandName != "Band" {
		t.Errorf("baked BandName = %q, want %q", cb.BandName, "Band")
	}

	// The done-when: it is in bundle.json on disk, under the proto3 canonical-JSON names.
	data, err := os.ReadFile(filepath.Join(b.bakesDir, setlistID, "1", "bundle.json"))
	if err != nil {
		t.Fatalf("read bundle.json: %v", err)
	}
	var parsed ConcertBundle
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("bundle.json parse: %v", err)
	}
	if parsed.BandID != bandID || parsed.BandName != "Band" {
		t.Errorf("bundle.json band = (%q, %q), want (%q, %q)", parsed.BandID, parsed.BandName, bandID, "Band")
	}
	raw := string(data)
	if !strings.Contains(raw, `"bandId"`) || !strings.Contains(raw, `"bandName"`) {
		t.Errorf("bundle.json must carry bandId + bandName keys, got:\n%s", raw)
	}
}

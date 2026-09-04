package app_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
	"testing/fstest"

	"troubastack/core/internal/app"
)

// canonicalDir builds a minimal canonical v2 band directory as an in-memory fs, plus a stray directory
// that ⟨P3⟩ must ignore.
func canonicalDir() fstest.MapFS {
	content := []byte("%PDF chart bytes")
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Packers",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin", "plays": "guitar"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{"slug": "wonderwall", "title": "Wonderwall",
			"files": []any{map[string]any{"filename": "chart.pdf", "contentType": "application/pdf"}}}},
	})
	ann, _ := json.Marshal(map[string]any{
		"layers":  []any{map[string]any{"id": "L1", "file": "chart.pdf", "owner": "_shared_", "zone": "shared", "access": "rw"}},
		"objects": []any{map[string]any{"uuid": "O1", "layer": "L1", "type": "rect", "page": 0, "style": map[string]any{"color": "#e11d48"}}},
	})
	return fstest.MapFS{
		"band.json":                   {Data: band},
		"repertoire.json":             {Data: rep},
		"annotations/wonderwall.json": {Data: ann},
		"wonderwall/chart.pdf":        {Data: content},
		"__pycache__/junk.pyc":        {Data: []byte("not a song")}, // ⟨P3⟩ stray: must be excluded
	}
}

// TestPack_RoundTripAndDeclaredOnly: ⟨P5⟩ unzip(pack(dir)) reproduces the canonical content byte-for-byte,
// and ⟨P3⟩ a stray directory never rides along.
func TestPack_RoundTripAndDeclaredOnly(t *testing.T) {
	dir := canonicalDir()
	zipBytes, size, err := app.PackBandDir(dir)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	if size != len(zipBytes) || size == 0 {
		t.Fatalf("reported size %d, zip %d", size, len(zipBytes))
	}
	back, err := app.UnpackBandZip(zipBytes)
	if err != nil {
		t.Fatalf("unpack: %v", err)
	}
	want := map[string]bool{"band.json": true, "repertoire.json": true, "annotations/wonderwall.json": true, "wonderwall/chart.pdf": true}
	for name := range back {
		if !want[name] {
			t.Fatalf("packed a non-canonical / stray entry: %q", name)
		}
	}
	for name := range want {
		got, ok := back[name]
		if !ok {
			t.Fatalf("round-trip lost %q", name)
		}
		orig, _ := dir.ReadFile(name)
		if !bytes.Equal(got, orig) {
			t.Fatalf("round-trip changed %q bytes", name)
		}
	}
	if _, stray := back["__pycache__/junk.pyc"]; stray {
		t.Fatal("⟨P3⟩ violated: a stray directory was packed as content")
	}
}

// TestPack_ImportsClean: the packed .tband imports through the SAME path the endpoint uses (⟨P1⟩'s
// "packs and imports" half; the seeding half is stage C).
func TestPack_ImportsClean(t *testing.T) {
	zipBytes, _, err := app.PackBandDir(canonicalDir())
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import of packed folder: %v", err)
	}
	if rep.Songs != 1 || rep.Files != 1 {
		t.Fatalf("packed import counts songs=%d files=%d, want 1/1", rep.Songs, rep.Files)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, _ := tgt.eng.Head(songs[0].ID)
	if len(snap.Layers) != 1 || snap.Layers[0].ID != "L1" {
		t.Fatalf("packed annotations not imported: %d layers", len(snap.Layers))
	}
}

// TestPack_Refusals: ⟨P4⟩ a declared file missing on disk, and ⟨P2⟩ a blobHash that disagrees with the
// bytes, each refuse the pack — naming the file — rather than packing quietly.
func TestPack_Refusals(t *testing.T) {
	t.Run("declared file missing on disk", func(t *testing.T) {
		dir := canonicalDir()
		delete(dir, "wonderwall/chart.pdf") // declared in repertoire, absent on disk
		if _, _, err := app.PackBandDir(dir); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("err=%v, want ErrInvalidInput", err)
		}
	})
	t.Run("blobHash disagrees with bytes", func(t *testing.T) {
		dir := canonicalDir()
		rep, _ := json.Marshal(map[string]any{
			"songs": []any{map[string]any{"slug": "wonderwall", "title": "Wonderwall",
				"files": []any{map[string]any{"filename": "chart.pdf", "blobHash": "sha256:deadbeef"}}}},
		})
		dir["repertoire.json"] = &fstest.MapFile{Data: rep}
		if _, _, err := app.PackBandDir(dir); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("err=%v, want ErrInvalidInput", err)
		}
	})
	t.Run("missing band.json", func(t *testing.T) {
		dir := canonicalDir()
		delete(dir, "band.json")
		if _, _, err := app.PackBandDir(dir); !errors.Is(err, app.ErrInvalidInput) {
			t.Fatalf("err=%v, want ErrInvalidInput", err)
		}
	})
}

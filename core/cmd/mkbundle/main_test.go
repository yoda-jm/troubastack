package main

import (
	"archive/zip"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"
)

const seed = 1700000000

// A valid demo bundle parses and has the structure the loader expects.
func TestValidBundle_structure(t *testing.T) {
	dir := t.TempDir()
	if err := run(dir, "", 2, 3, seed, false); err != nil {
		t.Fatal(err)
	}
	var b concertBundle
	readJSON(t, filepath.Join(dir, "bundle.json"), &b)

	if got := len(b.Songs); got != 2 {
		t.Fatalf("songs = %d, want 2", got)
	}
	for _, s := range b.Songs {
		if got := len(s.Pages); got != 3 {
			t.Fatalf("pages = %d, want 3", got)
		}
		for _, p := range s.Pages {
			if len(p.Overlays) != 2 {
				t.Fatalf("overlays = %d, want 2", len(p.Overlays))
			}
			// Every referenced blob exists and is non-empty.
			mustExistNonEmpty(t, filepath.Join(dir, filepath.FromSlash(p.PageRasterRef)))
			for _, o := range p.Overlays {
				mustExistNonEmpty(t, filepath.Join(dir, filepath.FromSlash(o.ImageRef)))
			}
		}
	}
	// The mandated layer shape: one mandatory layer, one role-tagged layer.
	first := b.Songs[0].Pages[0].Overlays
	if !first[0].Mandatory {
		t.Errorf("expected first overlay mandatory")
	}
	if first[1].RoleTag == "" {
		t.Errorf("expected second overlay to carry a roleTag")
	}
}

// Same flags ⇒ byte-identical output (committed fixtures must stay stable).
func TestDeterministic(t *testing.T) {
	d1, d2 := t.TempDir(), t.TempDir()
	if err := run(d1, "", 2, 3, seed, false); err != nil {
		t.Fatal(err)
	}
	if err := run(d2, "", 2, 3, seed, false); err != nil {
		t.Fatal(err)
	}
	// bundle.json and a raster must match byte-for-byte.
	assertSameBytes(t, filepath.Join(d1, "bundle.json"), filepath.Join(d2, "bundle.json"))
	assertSameBytes(t, filepath.Join(d1, "blobs/s1-p1-raster.png"), filepath.Join(d2, "blobs/s1-p1-raster.png"))
}

// The four torture variants exist and differ from the valid bundle in the intended way.
func TestTortureVariants(t *testing.T) {
	parent := t.TempDir()
	if err := run("", parent, 2, 3, seed, false); err != nil {
		t.Fatal(err)
	}

	// (a) missing-blob: valid manifest, but a raster it references is gone.
	var mb concertBundle
	readJSON(t, filepath.Join(parent, "missing-blob", "bundle.json"), &mb)
	ref := mb.Songs[0].Pages[0].PageRasterRef
	if _, err := os.Stat(filepath.Join(parent, "missing-blob", filepath.FromSlash(ref))); !os.IsNotExist(err) {
		t.Errorf("missing-blob: expected %s to be absent, err=%v", ref, err)
	}

	// (b) bad-json: bundle.json is present but not valid JSON.
	raw, err := os.ReadFile(filepath.Join(parent, "bad-json", "bundle.json"))
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(raw) {
		t.Errorf("bad-json: bundle.json unexpectedly parses as valid JSON")
	}

	// (c) empty: valid manifest, zero songs.
	var empty concertBundle
	readJSON(t, filepath.Join(parent, "empty", "bundle.json"), &empty)
	if len(empty.Songs) != 0 {
		t.Errorf("empty: songs = %d, want 0", len(empty.Songs))
	}

	// (d) no-manifest: blobs present, bundle.json absent.
	if _, err := os.Stat(filepath.Join(parent, "no-manifest", "bundle.json")); !os.IsNotExist(err) {
		t.Errorf("no-manifest: bundle.json should be absent, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(parent, "no-manifest", "blobs")); err != nil {
		t.Errorf("no-manifest: blobs/ should still be present: %v", err)
	}
}

// -zip produces a .tstage whose contents match the directory (bundle.json at the zip root).
func TestZipMatchesDir(t *testing.T) {
	dir := t.TempDir()
	if err := run(dir, "", 1, 1, seed, true); err != nil {
		t.Fatal(err)
	}
	zr, err := zip.OpenReader(dir + ".tstage")
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	foundManifest := false
	for _, f := range zr.File {
		if f.Name == "bundle.json" {
			foundManifest = true
		}
		// Every zip entry must exist on disk with identical bytes.
		onDisk, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(f.Name)))
		if err != nil {
			t.Fatalf("zip entry %s not on disk: %v", f.Name, err)
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		inZip, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(inZip) != string(onDisk) {
			t.Errorf("zip entry %s differs from disk", f.Name)
		}
	}
	if !foundManifest {
		t.Errorf("bundle.json not found at the zip root")
	}
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
}

func mustExistNonEmpty(t *testing.T, path string) {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("blob %s: %v", path, err)
	}
	if fi.Size() == 0 {
		t.Fatalf("blob %s is empty", path)
	}
}

func assertSameBytes(t *testing.T, a, b string) {
	t.Helper()
	da, err := os.ReadFile(a)
	if err != nil {
		t.Fatal(err)
	}
	db, err := os.ReadFile(b)
	if err != nil {
		t.Fatal(err)
	}
	if string(da) != string(db) {
		t.Errorf("%s and %s differ (non-deterministic output)", a, b)
	}
}

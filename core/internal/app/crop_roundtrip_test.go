package app_test

import (
	"encoding/json"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
)

// TestBandV2_RoundTripsFileCrop: a crop set on an uploaded file survives a full .tband v2 export → import,
// rect-for-rect, so a band that cropped its scores keeps them when it moves servers. The generated chart in
// the same song stays uncropped (it never carries a crop).
func TestBandV2_RoundTripsFileCrop(t *testing.T) {
	src := newStack()
	admin, _, bandID, songID, file1, _ := buildSourceBand(t, src)

	crop := app.Crop{Pages: map[string]app.CropRect{
		"0": {X: 0.1, Y: 0.12, W: 0.8, H: 0.76},
		"2": {X: 0.0, Y: 0.0, W: 0.5, H: 0.5},
	}}
	if _, err := src.svc.SetFileCrop(admin, bandID, songID, file1, crop); err != nil {
		t.Fatalf("set crop: %v", err)
	}

	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}

	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	if len(songs) != 1 {
		t.Fatalf("want 1 song, got %d", len(songs))
	}
	files, err := tgt.repo.FilesOfSong(songs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	var uploaded, generated *app.SongFile
	for i := range files {
		if files[i].Generated {
			generated = &files[i]
		} else {
			uploaded = &files[i]
		}
	}
	if uploaded == nil {
		t.Fatal("imported song has no uploaded file")
	}
	if generated != nil && generated.Crop != nil {
		t.Fatalf("a generated chart must never carry a crop, got %+v", generated.Crop)
	}

	got := uploaded.Crop
	if got == nil {
		t.Fatal("the crop did not survive the round trip")
	}
	if !got.LockAR {
		t.Fatalf("LockAR must round-trip as true, got %+v", got)
	}
	if len(got.Pages) != 2 {
		t.Fatalf("want 2 cropped pages, got %+v", got.Pages)
	}
	for key, want := range crop.Pages {
		g, ok := got.Pages[key]
		if !ok {
			t.Fatalf("page %q crop lost", key)
		}
		if g.X != want.X || g.Y != want.Y || g.W != want.W || g.H != want.H {
			t.Fatalf("page %q crop changed: got %+v want %+v", key, g, want)
		}
		if g.Source != "manual" {
			t.Fatalf("page %q source = %q, want manual", key, g.Source)
		}
	}
}

// TestBandImport_DropsBadCrop: a hand-edited .tband whose file carries a geometrically INVALID crop imports
// fine — the crop is sanitized away (importCrop), not injected and not fatal. Bypassing importCrop at the
// SongFile build would let the bad rect through, so this fails if the drop is removed.
func TestBandImport_DropsBadCrop(t *testing.T) {
	content := []byte("%PDF-1.4 bad-crop fixture")
	hash := blob.HashOf(content)
	man := map[string]any{
		"formatVersion": 1,
		"exportedAt":    "2026-01-01T00:00:00Z",
		"band":          map[string]any{"name": "Fixture Band"},
		"members": []any{
			map[string]any{"id": "u-alice", "username": "alice", "displayName": "Alice", "role": "admin"},
		},
		"songs": []any{
			map[string]any{
				"id": "sng-1", "title": "Tune",
				"files": []any{map[string]any{
					"id": "fil-1", "filename": "score.pdf", "contentType": "application/pdf",
					"size": len(content), "blobHash": hash, "displayOrder": 0,
					// x+w = 1.7 > 1: an out-of-page rect ValidateCrop refuses.
					"crop": map[string]any{"pages": map[string]any{"0": map[string]any{"x": 0.9, "y": 0, "w": 0.8, "h": 0.8}}},
				}},
			},
		},
		"setlists":    []any{},
		"annotations": map[string]any{},
	}
	bandJSON, _ := json.Marshal(man)
	zipBytes := rezip(t, map[string][]byte{
		"band.json":     bandJSON,
		"blobs/" + hash: content,
	})

	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("a bad crop must not fail the import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	files, err := tgt.repo.FilesOfSong(songs[0].ID)
	if err != nil || len(files) != 1 {
		t.Fatalf("want 1 imported file, got %d err=%v", len(files), err)
	}
	if files[0].Crop != nil {
		t.Fatalf("an invalid imported crop must be dropped, got %+v", files[0].Crop)
	}
}

package bake

import (
	"testing"

	"troubastack/core/internal/domain"
)

// TestSnapshotToDoc_FileScoping guards the per-file bake-scoping fix: a multi-file song
// carries per-file annotation layers (B11/T40), and the bake rasterizes ONE file (the
// default part). Only that file's layers — plus song-level layers with an empty FileID —
// may composite onto it; another part's ink (e.g. House of the Rising Sun's Drums-part
// notes) must NOT bleed onto the baked guitar page.
func TestSnapshotToDoc_FileScoping(t *testing.T) {
	const guitar, drums = "file-guitar", "file-drums"
	snap := domain.Snapshot{
		Layers: []domain.Layer{
			{ID: "L-guitar", FileID: guitar, Name: "Section markings"},
			{ID: "L-drums", FileID: drums, Name: "Drummer's notes"},
			{ID: "L-song", FileID: "", Name: "Song-level (legacy)"},
		},
		Objects: []domain.Object{
			{UUID: "o-guitar", LayerID: "L-guitar", Type: domain.TypeText, Text: "let it ring"},
			{UUID: "o-drums", LayerID: "L-drums", Type: domain.TypeText, Text: "snare — lay back"},
			{UUID: "o-song", LayerID: "L-song", Type: domain.TypeText, Text: "capo 2"},
		},
	}

	doc := snapshotToDoc(snap, guitar)

	gotLayers := map[string]bool{}
	for _, l := range doc.Layers {
		gotLayers[l.ID] = true
	}
	if !gotLayers["L-guitar"] {
		t.Error("baked file's own layer (L-guitar) was dropped")
	}
	if !gotLayers["L-song"] {
		t.Error("song-level layer (empty FileID) must composite onto whatever is baked")
	}
	if gotLayers["L-drums"] {
		t.Error("another file's layer (L-drums) bled onto the baked page — the bug this guards")
	}

	gotObjs := map[string]bool{}
	for _, o := range doc.Objects {
		gotObjs[o.UUID] = true
	}
	if !gotObjs["o-guitar"] || !gotObjs["o-song"] {
		t.Errorf("in-scope objects dropped: %v", gotObjs)
	}
	if gotObjs["o-drums"] {
		t.Error("out-of-scope object (o-drums, on the Drums part) composited onto the baked guitar page")
	}
}

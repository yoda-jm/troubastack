package app_test

import (
	"encoding/json"
	"errors"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
)

// TestBandExportV2_Layout: an export is the v2 folder format — band.json (formatVersion
// 2) + repertoire.json + setlists.json + annotations/<slug>.json — and it is
// hand-inspectable: name/slug/username based, with NO server UUIDs leaking into the
// human files. (T134 done-when: "unzip it and the JSON is the folder format".)
func TestBandExportV2_Layout(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)
	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	files := unzip(t, zipBytes)

	for _, want := range []string{"band.json", "repertoire.json", "setlists.json", "annotations/the-open-road.json"} {
		if _, ok := files[want]; !ok {
			t.Fatalf("export missing %q; entries=%v", want, keysOf(files))
		}
	}

	var band map[string]any
	mustJSON(t, files["band.json"], &band)
	if v, _ := band["formatVersion"].(float64); int(v) != 2 {
		t.Fatalf("band.json formatVersion=%v, want 2", band["formatVersion"])
	}
	if band["name"] != "The Troubadours" {
		t.Fatalf("band.json name=%v", band["name"])
	}
	members, _ := band["members"].([]any)
	if len(members) != 2 {
		t.Fatalf("band.json members=%d, want 2", len(members))
	}
	for _, m := range members {
		mm := m.(map[string]any)
		if _, leaked := mm["id"]; leaked {
			t.Fatalf("band.json member leaks a server id: %v", mm)
		}
		if mm["username"] == nil {
			t.Fatalf("band.json member has no username: %v", mm)
		}
	}

	var rep struct {
		Songs []struct {
			Slug  string `json:"slug"`
			Title string `json:"title"`
			Files []struct {
				Filename string `json:"filename"`
			} `json:"files"`
		} `json:"songs"`
	}
	mustJSON(t, files["repertoire.json"], &rep)
	if len(rep.Songs) != 1 || rep.Songs[0].Slug != "the-open-road" || rep.Songs[0].Title != "The Open Road" {
		t.Fatalf("repertoire songs=%+v", rep.Songs)
	}
	if len(rep.Songs[0].Files) != 2 {
		t.Fatalf("repertoire song files=%d, want 2", len(rep.Songs[0].Files))
	}

	// annotations/<slug>.json references files by filename and owners by username/sentinel,
	// never a UUID; ids are kept verbatim.
	var ann struct {
		Layers []struct {
			ID    string `json:"id"`
			File  string `json:"file"`
			Owner string `json:"owner"`
		} `json:"layers"`
		Objects []struct {
			UUID  string `json:"uuid"`
			Layer string `json:"layer"`
		} `json:"objects"`
	}
	mustJSON(t, files["annotations/the-open-road.json"], &ann)
	if len(ann.Layers) != 2 || len(ann.Objects) != 2 {
		t.Fatalf("annotations layers=%d objects=%d, want 2/2", len(ann.Layers), len(ann.Objects))
	}
	fileNames := map[string]bool{}
	for _, f := range rep.Songs[0].Files {
		fileNames[f.Filename] = true
	}
	for _, l := range ann.Layers {
		if l.ID != "L-shared" && l.ID != "L-mine" {
			t.Fatalf("layer id not kept verbatim: %q", l.ID)
		}
		if l.File != "" && !fileNames[l.File] {
			t.Fatalf("layer file %q is not a repertoire filename", l.File)
		}
		if l.Owner != "_shared_" && l.Owner != "leo" {
			t.Fatalf("layer owner %q is not a username/sentinel", l.Owner)
		}
	}
	got := map[string]bool{}
	for _, o := range ann.Objects {
		got[o.UUID] = true
	}
	if !got["o1"] || !got["o2"] {
		t.Fatalf("object uuids not kept verbatim: %v", got)
	}
}

// TestBandV2_RoundTripByID: export -> import -> the SAME layers and objects BY ID, not
// merely the same count (the T134 acceptance experiment).
func TestBandV2_RoundTripByID(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)
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
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	layerIDs := map[string]bool{}
	for _, l := range snap.Layers {
		layerIDs[l.ID] = true
	}
	if !layerIDs["L-shared"] || !layerIDs["L-mine"] || len(layerIDs) != 2 {
		t.Fatalf("layer ids not preserved verbatim: %v", layerIDs)
	}
	objByID := map[string]bool{}
	for _, o := range snap.Objects {
		if !o.Deleted {
			objByID[o.UUID] = true
		}
	}
	if !objByID["o1"] || !objByID["o2"] || len(objByID) != 2 {
		t.Fatalf("object uuids not preserved verbatim: %v", objByID)
	}
	// object -> layer linkage survives (layer ids are kept, so LayerID needs no remap).
	for _, o := range snap.Objects {
		if o.Deleted {
			continue
		}
		if o.LayerID != "L-shared" && o.LayerID != "L-mine" {
			t.Fatalf("object %q layer link broken: %q", o.UUID, o.LayerID)
		}
	}
}

// TestBandImport_V1Fixture: a v1 export (single UUID-based band.json, annotations with
// capitalized keys + int enums) STILL imports — v1 -> v2 is a rearrangement, not a
// migration, so refusing v1 would strand exports that already exist. (Done-when.)
func TestBandImport_V1Fixture(t *testing.T) {
	content := []byte("%PDF-1.4 v1 fixture")
	hash := blob.HashOf(content)
	man := map[string]any{
		"formatVersion": 1,
		"exportedAt":    "2026-01-01T00:00:00Z",
		"band":          map[string]any{"name": "V1 Legacy Band"},
		"members": []any{
			map[string]any{"id": "u-alice", "username": "alice", "displayName": "Alice", "role": "admin"},
		},
		"songs": []any{
			map[string]any{
				"id": "sng-1", "title": "Legacy Tune",
				"files": []any{map[string]any{
					"id": "fil-1", "filename": "chart.pdf", "contentType": "application/pdf",
					"size": len(content), "blobHash": hash, "displayOrder": 0,
				}},
			},
		},
		"setlists": []any{},
		// v1 annotations embed the untagged domain types: capitalized keys, integer enums.
		"annotations": map[string]any{
			"sng-1": map[string]any{
				"Layers": []any{map[string]any{
					"ID": "L1", "FileID": "fil-1", "Name": "Cues", "OwnerID": "_shared_",
					"Zone": 2, "Order": 0, "Access": 1,
				}},
				"Objects": []any{map[string]any{
					"UUID": "v1-obj-1", "Type": 3, "Page": 0, "LayerID": "L1", "Version": 1,
					"Points": []any{map[string]any{"X": 0.1, "Y": 0.1}, map[string]any{"X": 0.4, "Y": 0.3}},
					"Style":  map[string]any{"Color": "#e11d48", "Opacity": 1, "Width": 0.004},
				}},
			},
		},
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
		t.Fatalf("v1 fixture import: %v", err)
	}
	if rep.Songs != 1 || rep.Files != 1 {
		t.Fatalf("v1 import counts songs=%d files=%d, want 1/1", rep.Songs, rep.Files)
	}
	songs, _ := tgt.repo.SongsOfBand(rep.Band.ID)
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil || len(snap.Layers) != 1 {
		t.Fatalf("v1 annotations: %d layers err=%v", len(snap.Layers), err)
	}
	if snap.Layers[0].ID != "L1" {
		t.Fatalf("v1 layer id not kept: %q", snap.Layers[0].ID)
	}
	live := 0
	for _, o := range snap.Objects {
		if !o.Deleted {
			live++
			if o.UUID != "v1-obj-1" {
				t.Fatalf("v1 object uuid not kept: %q", o.UUID)
			}
		}
	}
	if live != 1 {
		t.Fatalf("v1 objects=%d, want 1", live)
	}
}

// TestBandImport_UnknownVersion: an unknown formatVersion is refused with a 400
// (ErrInvalidInput), no migration attempted. (Done-when.)
func TestBandImport_UnknownVersion(t *testing.T) {
	bandJSON, _ := json.Marshal(map[string]any{"formatVersion": 999, "name": "X"})
	zipBytes := rezip(t, map[string][]byte{"band.json": bandJSON})
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	if _, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil); !errors.Is(err, app.ErrInvalidInput) {
		t.Fatalf("unknown version err=%v, want ErrInvalidInput", err)
	}
}

// TestBandImport_HandAuthoredV2Folder: a hand-written v2 folder (NOT a server export)
// that includes annotations/<slug>.json imports WITH its annotations — the case that is
// impossible in the seed folder format today (a chart-only song gets none). (Done-when.)
func TestBandImport_HandAuthoredV2Folder(t *testing.T) {
	content := []byte("%PDF hand authored")
	hash := blob.HashOf(content)
	band, _ := json.Marshal(map[string]any{
		"formatVersion": 2, "name": "Hand Band",
		"members": []any{map[string]any{"username": "dana", "displayName": "Dana", "role": "admin"}},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{
			"slug": "wonderwall", "title": "Wonderwall",
			"files": []any{map[string]any{"filename": "chart.pdf", "contentType": "application/pdf", "size": len(content), "blobHash": hash}},
		}},
	})
	ann, _ := json.Marshal(map[string]any{
		"layers": []any{map[string]any{
			"id": "hand-L1", "file": "chart.pdf", "name": "Cues", "owner": "_shared_", "zone": "shared", "access": "rw",
		}},
		"objects": []any{map[string]any{
			"uuid": "hand-O1", "layer": "hand-L1", "type": "rect", "page": 0,
			"points": []any{map[string]any{"x": 0.1, "y": 0.1}, map[string]any{"x": 0.5, "y": 0.4}},
			"style":  map[string]any{"color": "#e11d48", "opacity": 1, "width": 0.004},
		}},
	})
	zipBytes := rezip(t, map[string][]byte{
		"band.json":                   band,
		"repertoire.json":             rep,
		"annotations/wonderwall.json": ann,
		"blobs/" + hash:               content,
	})

	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	report, err := tgt.svc.ImportBand(importer, tgt.eng, zipBytes, nil)
	if err != nil {
		t.Fatalf("hand-authored v2 import: %v", err)
	}
	songs, _ := tgt.repo.SongsOfBand(report.Band.ID)
	if len(songs) != 1 {
		t.Fatalf("want 1 song, got %d", len(songs))
	}
	snap, err := tgt.eng.Head(songs[0].ID)
	if err != nil || len(snap.Layers) != 1 || snap.Layers[0].ID != "hand-L1" {
		t.Fatalf("hand-authored annotations not imported: layers=%d err=%v", len(snap.Layers), err)
	}
	if snap.Layers[0].FileID == "" {
		t.Fatal("hand-authored layer file ref did not resolve to a file id")
	}
	live := 0
	for _, o := range snap.Objects {
		if !o.Deleted {
			live++
			if o.UUID != "hand-O1" || o.LayerID != "hand-L1" {
				t.Fatalf("hand-authored object not kept: %+v", o)
			}
		}
	}
	if live != 1 {
		t.Fatalf("hand-authored objects=%d, want 1", live)
	}
}

// --- small local helpers -------------------------------------------------------------

func mustJSON(t *testing.T, b []byte, v any) {
	t.Helper()
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("json: %v\n%s", err, string(b))
	}
}

func keysOf(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

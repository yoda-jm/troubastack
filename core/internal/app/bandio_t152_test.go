package app_test

import (
	"encoding/json"
	"testing"
)

// T152 — an author-declared band identity (shortname/kind/notes) must SURVIVE a folder round-trip. Before
// this fix ExportBand wrote only {exportedAt, formatVersion, members, name}, so a round-trip silently
// unnamed the band (shortname is the `make band=<shortname>` handle and the key T150's id builds on).
//
// Round-trip a folder that declares all six fields and assert the exported band.json still carries the
// three that used to evaporate — asserting VALUES, not just presence. Red today on all three.
func TestBandV2_ExportCarriesDeclaredIdentity_T152(t *testing.T) {
	src := newStack()
	admin, _, bandID, _, _, _ := buildSourceBand(t, src)
	zipBytes, _, err := src.svc.ExportBand(admin, src.eng, bandID)
	if err != nil {
		t.Fatalf("export: %v", err)
	}

	// A folder on disk declares shortname/kind/notes; inject them into band.json to model that folder.
	files := unzip(t, zipBytes)
	var band map[string]any
	mustJSON(t, files["band.json"], &band)
	band["shortname"] = "altoband"
	band["kind"] = "covers"
	band["notes"] = "rehearsal band"
	injected, err := json.Marshal(band)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	files["band.json"] = injected

	// Import that folder, then RE-EXPORT the imported band and assert the three fields survived storage.
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	rep, err := tgt.svc.ImportBand(importer, tgt.eng, rezip(t, files), nil)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	reexport, _, err := tgt.svc.ExportBand(importer, tgt.eng, rep.Band.ID)
	if err != nil {
		t.Fatalf("re-export: %v", err)
	}
	var out map[string]any
	mustJSON(t, unzip(t, reexport)["band.json"], &out)

	if out["shortname"] != "altoband" {
		t.Errorf("round-trip dropped shortname: got %v, want \"altoband\"", out["shortname"])
	}
	if out["kind"] != "covers" {
		t.Errorf("round-trip dropped kind: got %v, want \"covers\"", out["kind"])
	}
	if out["notes"] != "rehearsal band" {
		t.Errorf("round-trip dropped notes: got %v, want \"rehearsal band\"", out["notes"])
	}
	// And the name (never dropped) still round-trips, so the injection didn't corrupt the manifest.
	if out["name"] == nil || out["name"] == "" {
		t.Errorf("round-trip lost the band name: %v", out["name"])
	}
}

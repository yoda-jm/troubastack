package app_test

import (
	"encoding/json"
	"testing"
	"testing/fstest"

	"troubastack/core/internal/app"
)

// legacyFolder builds a legacy seed folder-vocab band directory: admin beside members, prose `role`,
// conductor flag, repertoire with NO files[] (globbed), a PDF, and a text chart.
func legacyFolder() fstest.MapFS {
	band, _ := json.Marshal(map[string]any{
		"name": "Legacy Band", "shortname": "lb", "kind": "Band", "notes": "n",
		"admin": map[string]any{"username": "marie", "display": "Marie", "role": "keys"},
		"members": []any{
			map[string]any{"username": "leo", "display": "Leo", "role": "lead guitar", "conductor": true},
			map[string]any{"username": "sasha", "display": "Sasha", "role": "drums"},
		},
	})
	rep, _ := json.Marshal(map[string]any{
		"songs": []any{map[string]any{"slug": "the-open-road", "title": "The Open Road", "artist": "Oasis",
			"key": "G", "tempo": 87, "meter": "6/8", "tags": []string{"britpop"}}},
	})
	setl, _ := json.Marshal(map[string]any{
		"setlists": []any{map[string]any{"name": "Gig", "eventDate": "2026-07-04", "venue": "Anchor",
			"items": []any{map[string]any{"song": "the-open-road", "keyOverride": "A", "transposeChords": true}}}},
	})
	return fstest.MapFS{
		"band.json":                {Data: band},
		"repertoire.json":          {Data: rep},
		"setlists.json":            {Data: setl},
		"the-open-road/score.pdf":  {Data: []byte("%PDF-1.4 score")},
		"the-open-road/lyrics.txt": {Data: []byte("# The Open Road\n## Verse\nG   C\nla la\n")},
	}
}

// TestMigrate_LegacyToCanonical_ImportsWithVocabTranslated: the ⟨P1⟩ regression — a legacy folder both
// migrates+packs AND imports, with the member vocabulary translated (display→displayName, prose role→
// plays, conductor→the enum) and the text chart carried as source.
func TestMigrate_LegacyToCanonical_ImportsWithVocabTranslated(t *testing.T) {
	entries, wasLegacy, err := app.MigrateLegacyFolder(legacyFolder())
	if err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if !wasLegacy {
		t.Fatal("a legacy folder should report wasLegacy=true")
	}

	// band.json translated to the canonical vocabulary.
	var band struct {
		FormatVersion int `json:"formatVersion"`
		Members       []struct {
			Username, DisplayName, Role, Plays string
		} `json:"members"`
	}
	if err := json.Unmarshal(entries["band.json"], &band); err != nil {
		t.Fatal(err)
	}
	if band.FormatVersion != 2 || len(band.Members) != 3 {
		t.Fatalf("band.json: version=%d members=%d, want 2/3", band.FormatVersion, len(band.Members))
	}
	byName := map[string]struct{ Username, DisplayName, Role, Plays string }{}
	for _, m := range band.Members {
		byName[m.Username] = m
	}
	if m := byName["marie"]; m.Role != "admin" || m.DisplayName != "Marie" || m.Plays != "keys" {
		t.Fatalf("admin not folded/translated: %+v", m)
	}
	if m := byName["leo"]; m.Role != "conductor" || m.Plays != "lead guitar" {
		t.Fatalf("conductor flag/prose not translated: %+v", m)
	}
	if m := byName["sasha"]; m.Role != "member" || m.Plays != "drums" {
		t.Fatalf("member not translated: %+v", m)
	}

	// repertoire.json gained a derived files[] (the .pdf and the .txt as generated).
	var rep struct {
		Songs []struct {
			Files []struct {
				Filename  string `json:"filename"`
				Generated bool   `json:"generated"`
			} `json:"files"`
		} `json:"songs"`
	}
	json.Unmarshal(entries["repertoire.json"], &rep)
	if len(rep.Songs) != 1 || len(rep.Songs[0].Files) != 2 {
		t.Fatalf("repertoire files not derived: %+v", rep.Songs)
	}
	gen := 0
	for _, f := range rep.Songs[0].Files {
		if f.Generated {
			gen++
			if entries["the-open-road/"+f.Filename] == nil {
				t.Fatalf("generated file %q has no source bytes", f.Filename)
			}
		}
	}
	if gen != 1 {
		t.Fatalf("want exactly 1 generated (text) chart, got %d", gen)
	}

	// It packs and imports (⟨P1⟩ "both seeds and packs" — the pack+import half).
	zip, _, err := app.PackEntries(entries)
	if err != nil {
		t.Fatalf("pack: %v", err)
	}
	tgt := newStack()
	importer, _ := tgt.svc.Register("owner", "Owner", "password123", "")
	report, err := tgt.svc.ImportBand(importer, tgt.eng, zip, map[string]app.ImportDisposition{
		"marie": app.DispositionCreate, "leo": app.DispositionCreate, "sasha": app.DispositionCreate,
	})
	if err != nil {
		t.Fatalf("import of migrated folder: %v", err)
	}
	if report.Songs != 1 || report.Files != 2 {
		t.Fatalf("imported songs=%d files=%d, want 1/2", report.Songs, report.Files)
	}
}

// TestMigrate_Idempotent: a folder already at formatVersion 2 passes through unchanged (wasLegacy=false),
// so migrate-on-read of a migrated folder is a no-op — the property that lets the bridge be safe.
func TestMigrate_Idempotent(t *testing.T) {
	// migrate once, materialise the canonical dir, migrate again.
	entries, _, err := app.MigrateLegacyFolder(legacyFolder())
	if err != nil {
		t.Fatal(err)
	}
	canon := fstest.MapFS{}
	for name, data := range entries {
		canon[name] = &fstest.MapFile{Data: data}
	}
	again, wasLegacy, err := app.MigrateLegacyFolder(canon)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if wasLegacy {
		t.Fatal("a canonical folder must pass through (wasLegacy=false)")
	}
	if len(again) != len(entries) {
		t.Fatalf("passthrough changed the entry set: %d vs %d", len(again), len(entries))
	}
	for name, data := range entries {
		if string(again[name]) != string(data) {
			t.Fatalf("passthrough changed %q", name)
		}
	}
}

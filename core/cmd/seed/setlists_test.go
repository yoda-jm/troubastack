package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// T100 — a local band folder can define its concerts in setlists.json.
//
// The load-bearing behaviours: a MISSING file is normal (back-compat), an UNKNOWN SLUG is a loud
// error naming the slug (a gig list that silently loses a song is the worst failure here), array
// ORDER is the item order, and a plain `seed` still skips personal bands even now that they carry
// concert data.

const twoSetlists = `{
  "setlists": [
    {"name": "Sat @ The Hall", "eventDate": "2026-09-05", "venue": "The Hall", "notes": "60 min",
     "items": [
       {"song": "s2", "keyOverride": "Bm", "notes": "lift it"},
       {"song": "s1", "onCall": true, "transposeChords": true, "tempoOverride": 104}
     ]},
    {"name": "Sun @ The Park", "eventDate": "2026-09-06",
     "items": [{"song": "s1"}]}
  ]
}`

func loadOneBand(t *testing.T, setlists string) groupDef {
	t.Helper()
	dir := t.TempDir()
	writeBand(t, dir, "myband", validManifest, validRepertoire)
	if setlists != "" {
		writeFile(t, filepath.Join(dir, "myband", "setlists.json"), setlists)
	}
	t.Setenv("TROUBA_BANDS_DIR", dir)
	groups, _, err := loadLocalBands()
	if err != nil {
		t.Fatalf("loadLocalBands: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("groups = %d, want 1", len(groups))
	}
	return groups[0]
}

func TestSetlists_missingFileIsNormal(t *testing.T) {
	g := loadOneBand(t, "")
	if len(g.setlists) != 0 {
		t.Errorf("no setlists.json ⇒ %d setlists, want 0 (the band must seed exactly as before)", len(g.setlists))
	}
	if len(g.songs) != 2 {
		t.Errorf("songs = %d, want 2 — the repertoire must be unaffected", len(g.songs))
	}
}

func TestSetlists_twoConcerts_inArrayOrder_withOverrides(t *testing.T) {
	g := loadOneBand(t, twoSetlists)
	if len(g.setlists) != 2 {
		t.Fatalf("setlists = %d, want 2", len(g.setlists))
	}
	if g.setlists[0].name != "Sat @ The Hall" || g.setlists[1].name != "Sun @ The Park" {
		t.Errorf("file order not preserved: %q then %q", g.setlists[0].name, g.setlists[1].name)
	}
	if g.setlists[0].eventDate != "2026-09-05" || g.setlists[0].venue != "The Hall" {
		t.Errorf("date/venue lost: %+v", g.setlists[0])
	}

	items := g.setlists[0].items
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	// ARRAY ORDER is the item order: s2 is listed first even though the repertoire lists s1 first.
	if items[0].song != "Song Two" || items[1].song != "Song One" {
		t.Errorf("array order not honoured: got %q then %q", items[0].song, items[1].song)
	}
	if items[0].keyOverride != "Bm" || items[0].notes != "lift it" {
		t.Errorf("overrides lost on item 0: %+v", items[0])
	}
	// onCall / transposeChords are the .tband manifestItem fields overrideDef previously lacked.
	if !items[1].onCall || !items[1].transposeChords || items[1].tempoOverride != 104 {
		t.Errorf("onCall/transposeChords/tempoOverride lost on item 1: %+v", items[1])
	}
}

func TestSetlists_unknownSlug_failsLoudly_namingIt(t *testing.T) {
	dir := t.TempDir()
	writeBand(t, dir, "myband", validManifest, validRepertoire)
	writeFile(t, filepath.Join(dir, "myband", "setlists.json"),
		`{"setlists":[{"name":"Gig","items":[{"song":"s1"},{"song":"no-such-song"}]}]}`)
	t.Setenv("TROUBA_BANDS_DIR", dir)

	_, _, err := loadLocalBands()
	if err == nil {
		t.Fatal("an unknown slug must be an ERROR — silently dropping a song from a gig list is the failure this guards")
	}
	if !strings.Contains(err.Error(), "no-such-song") {
		t.Errorf("the error must NAME the offending slug, got: %v", err)
	}
}

func TestSetlists_plainSeedStillSkipsPersonalBands(t *testing.T) {
	// This task adds data to the very thing a plain `seed` skips, so assert the gate still holds.
	g := loadOneBand(t, twoSetlists)
	if !g.personal {
		t.Fatal("a discovered band folder must stay personal:true")
	}
	// The demo-isolation property itself is TestSelectGroups' job; this asserts it still holds for a
	// band that now carries concert data — the thing T100 added to the very groups a plain seed skips.
	if _, _, err := selectGroups([]groupDef{g}, nil, "", nil); err == nil {
		t.Error("a plain seed selected a personal band — a real band's gig list must never enter the demo")
	}
	kept, _, err := selectGroups([]groupDef{g}, nil, "", []string{g.shortname})
	if err != nil || len(kept) != 1 {
		t.Errorf("`seed -band %s` must still select it: kept=%d err=%v", g.shortname, len(kept), err)
	}
	if len(kept) == 1 && len(kept[0].setlists) != 2 {
		t.Errorf("the selected band lost its setlists: %d", len(kept[0].setlists))
	}
}

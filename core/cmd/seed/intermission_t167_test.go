package main

import (
	"encoding/json"
	"strings"
	"testing"
)

// T167 — a band folder must be able to express an INTERMISSION, because a real setlist has one.
//
// The archive format learned `kind`/`label` in T153; the folder format did not. A break carries no
// repertoire slug, and loadSetlists treats an unknown slug as a hard error — rightly, since a gig list
// that quietly loses a song is the worst thing it can do — so a folder describing a real setlist with a
// break made `make band=<shortname>` fail outright with `unknown song slug ""`. Found while backporting
// VLL's live setlists into his band library: one band went in, the other could not.

const setlistWithBreak = `{
  "setlists": [
    {"name": "Sat @ The Hall", "eventDate": "2026-09-05", "venue": "The Hall",
     "items": [
       {"song": "s1"},
       {"kind": "intermission", "label": "Entracte"},
       {"song": "s2", "keyOverride": "Bm"}
     ]}
  ]
}`

func TestSetlists_readsAnIntermission_T167(t *testing.T) {
	g := loadOneBand(t, setlistWithBreak)
	if len(g.setlists) != 1 {
		t.Fatalf("setlists = %d, want 1", len(g.setlists))
	}
	items := g.setlists[0].items
	if len(items) != 3 {
		t.Fatalf("items = %d, want 3 (song, break, song) — a break must not be dropped or refused", len(items))
	}
	if items[1].kind != "intermission" {
		t.Errorf("item 1 kind = %q, want %q — the break did not survive the folder", items[1].kind, "intermission")
	}
	if items[1].label != "Entracte" {
		t.Errorf("item 1 label = %q, want %q — the band's own words are content, not decoration", items[1].label, "Entracte")
	}
	if items[1].song != "" {
		t.Errorf("item 1 song = %q, want empty — a break is not a song and must not be slug-resolved", items[1].song)
	}
	// The songs around it are untouched, in array order, with their overrides.
	if items[0].song != "Song One" || items[2].song != "Song Two" {
		t.Errorf("songs around the break = (%q, %q), want (Song One, Song Two)", items[0].song, items[2].song)
	}
	if items[2].keyOverride != "Bm" {
		t.Errorf("the override after a break was lost: %q", items[2].keyOverride)
	}
}

// TEETH for the T153 rule, which this must not break: an item with NO `kind` is a song, and its kind
// stays EMPTY rather than being normalised to the literal "song". A test asserting `kind == "song"` would
// pass on a version that stamped every item, and would hide exactly the drift this task is repairing.
func TestSetlists_anItemWithoutAKindStaysAPlainSong_T167(t *testing.T) {
	g := loadOneBand(t, twoSetlists)
	for si, sl := range g.setlists {
		for ii, it := range sl.items {
			if it.kind != "" {
				t.Errorf("setlist %d item %d: kind = %q, want empty — absent must keep meaning \"song\"", si, ii, it.kind)
			}
			if it.label != "" {
				t.Errorf("setlist %d item %d: label = %q, want empty", si, ii, it.label)
			}
		}
	}
}

func TestSetlists_intermissionWithABlankLabel_T167(t *testing.T) {
	const src = `{"setlists":[{"name":"X","items":[{"song":"s1"},{"kind":"intermission"}]}]}`
	g := loadOneBand(t, src)
	items := g.setlists[0].items
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2", len(items))
	}
	if items[1].kind != "intermission" || items[1].label != "" {
		t.Errorf("blank-label break = (kind %q, label %q), want (intermission, empty) — the default word is the renderer's business, not the seed's", items[1].kind, items[1].label)
	}
}

// The other half: the canonical WRITER must emit the break, or the next canonicalisation strips it and
// re-opens the hole from the other end.
func TestCanonical_emitsTheIntermission_T167(t *testing.T) {
	g := loadOneBand(t, setlistWithBreak)
	entries, err := groupToCanonical(g, map[string]person{})
	if err != nil {
		t.Fatalf("groupToCanonical: %v", err)
	}
	raw, ok := entries["setlists.json"]
	if !ok {
		t.Fatal("no setlists.json in the canonical entries")
	}
	var doc struct {
		Setlists []struct {
			Items []struct {
				Song  string `json:"song"`
				Kind  string `json:"kind"`
				Label string `json:"label"`
			} `json:"items"`
		} `json:"setlists"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("parse emitted setlists.json: %v\n%s", err, raw)
	}
	if len(doc.Setlists) != 1 || len(doc.Setlists[0].Items) != 3 {
		t.Fatalf("emitted %d setlist(s) with %d item(s), want 1 with 3", len(doc.Setlists), len(doc.Setlists[0].Items))
	}
	br := doc.Setlists[0].Items[1]
	if br.Kind != "intermission" || br.Label != "Entracte" || br.Song != "" {
		t.Errorf("emitted break = %+v, want kind=intermission label=Entracte song=empty", br)
	}
	// And a plain song still emits no kind at all — absent keeps its meaning on the wire too.
	if s := doc.Setlists[0].Items[0]; s.Kind != "" || s.Label != "" {
		t.Errorf("a plain song emitted kind=%q label=%q, want both absent", s.Kind, s.Label)
	}
	if !strings.Contains(string(raw), `"slug"`) && !strings.Contains(string(raw), `"song"`) {
		t.Error("the emitted file references no songs at all — the fixture is not exercising the writer")
	}
}

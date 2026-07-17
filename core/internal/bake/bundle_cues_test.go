package bake

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBundleCues_AdditiveCompat is the T50 proto/loader guard for field 10 (cues):
// it is additive metadata exactly like fields 5–9 (B02). An OLD bundle (no cues
// field) must load in the new mirror as empty cues, and a NEW bundle must round-trip
// its cues; a loader that ignores the field is unaffected (the standard compat
// argument, exercised loader-side here).
func TestBundleCues_AdditiveCompat(t *testing.T) {
	// Old bundle: a BakedSong JSON minted before cues existed — no `cues` key.
	const oldJSON = `{
	  "concertId": "c1",
	  "songs": [
	    {"songId": "s1", "sourceRevision": "3", "title": "Wonderwall"}
	  ]
	}`
	var oldCB ConcertBundle
	if err := json.Unmarshal([]byte(oldJSON), &oldCB); err != nil {
		t.Fatalf("old bundle must still load: %v", err)
	}
	if len(oldCB.Songs) != 1 || oldCB.Songs[0].Cues != nil && len(oldCB.Songs[0].Cues) != 0 {
		t.Fatalf("old bundle cues = %+v, want empty", oldCB.Songs[0].Cues)
	}

	// New bundle: a BakedSong carrying cues round-trips, tints preserved and order kept.
	newCB := ConcertBundle{
		ConcertID: "c1",
		Songs: []BakedSong{{
			SongID: "s1",
			Cues: []SongCue{
				{Icon: "mic"},
				{Icon: "guitar-electric", Color: "#e11d48"},
			},
		}},
	}
	data, err := newCB.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"cues"`) {
		t.Fatalf("canonical JSON missing cues field:\n%s", data)
	}
	var round ConcertBundle
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	got := round.Songs[0].Cues
	if len(got) != 2 || got[0].Icon != "mic" || got[0].Color != "" ||
		got[1].Icon != "guitar-electric" || got[1].Color != "#e11d48" {
		t.Fatalf("round-tripped cues = %+v, want [mic, guitar-electric#e11d48] in order", got)
	}

	// A song with no cues omits the field entirely (omitempty) — old loaders see nothing.
	noCues := ConcertBundle{Songs: []BakedSong{{SongID: "s2"}}}
	nd, err := noCues.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal no-cues: %v", err)
	}
	if strings.Contains(string(nd), `"cues"`) {
		t.Fatalf("no-cues bundle should omit the field:\n%s", nd)
	}
}

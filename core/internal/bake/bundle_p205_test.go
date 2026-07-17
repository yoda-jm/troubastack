package bake

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestBundleP205_AdditiveCompat is the P205 proto/loader guard for the band-wide
// bundle fields: LayerImage.owner (8) + default_on (9), BakedSong.member_cues (11),
// ConcertBundle.roster (8) + BundleMember. All additive exactly like T50's field 10:
// an OLD bundle (none of these keys) loads with zero-values, and a NEW bundle
// round-trips them; default_on is proto3 `optional` (a *bool) so ABSENT and
// present-false are distinguishable.
func TestBundleP205_AdditiveCompat(t *testing.T) {
	// --- Old bundle: minted before P205; no owner/defaultOn/memberCues/roster keys.
	const oldJSON = `{
	  "concertId": "c1",
	  "songs": [
	    {"songId": "s1", "title": "Wonderwall",
	     "pages": [{"pageRasterRef": "blobs/p.png",
	                "overlays": [{"layerId": "L1", "name": "Conductor cues"}]}]}
	  ]
	}`
	var oldCB ConcertBundle
	if err := json.Unmarshal([]byte(oldJSON), &oldCB); err != nil {
		t.Fatalf("old bundle must still load: %v", err)
	}
	ov := oldCB.Songs[0].Pages[0].Overlays[0]
	if ov.Owner != "" || ov.DefaultOn != nil {
		t.Fatalf("old overlay owner=%q defaultOn=%v, want \"\"/nil (absent)", ov.Owner, ov.DefaultOn)
	}
	if len(oldCB.Roster) != 0 || len(oldCB.Songs[0].MemberCues) != 0 {
		t.Fatalf("old bundle roster/memberCues should be empty, got %+v / %+v", oldCB.Roster, oldCB.Songs[0].MemberCues)
	}

	// --- New band-wide bundle round-trips every P205 field.
	on, off := true, false
	newCB := ConcertBundle{
		ConcertID: "c1",
		Roster: []BundleMember{
			{MemberID: "m-marie", DisplayName: "Marie", Role: "admin"},
			{MemberID: "m-leo", DisplayName: "Leo", Role: "member"},
		},
		Songs: []BakedSong{{
			SongID: "s1",
			Pages: []PageImages{{PageRasterRef: "blobs/p.png", Overlays: []LayerImage{
				{LayerID: "L1", Name: "Conductor cues", Owner: "", Mandatory: true, DefaultOn: &on}, // shared, on
				{LayerID: "L2", Name: "My notes", Owner: "m-marie", DefaultOn: &off},                // Marie's, off
			}}},
			MemberCues: []MemberCues{
				{MemberID: "m-marie", Cues: []SongCue{{Icon: "mic"}, {Icon: "guitar-electric", Color: "#e11d48"}}},
				{MemberID: "m-leo", Cues: []SongCue{{Icon: "tambourine"}}},
			},
		}},
	}
	data, err := newCB.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	js := string(data)
	for _, want := range []string{`"roster"`, `"owner"`, `"defaultOn"`, `"memberCues"`, `"m-marie"`} {
		if !strings.Contains(js, want) {
			t.Fatalf("canonical JSON missing %s:\n%s", want, js)
		}
	}
	// default_on must be a JSON boolean, not a string (proto3 bool).
	if !strings.Contains(js, `"defaultOn": true`) || !strings.Contains(js, `"defaultOn": false`) {
		t.Fatalf("defaultOn must serialize as JSON bool true/false:\n%s", js)
	}

	var round ConcertBundle
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if len(round.Roster) != 2 || round.Roster[0].MemberID != "m-marie" || round.Roster[1].Role != "member" {
		t.Fatalf("roster round-trip = %+v", round.Roster)
	}
	ovs := round.Songs[0].Pages[0].Overlays
	if ovs[0].Owner != "" || ovs[0].DefaultOn == nil || *ovs[0].DefaultOn != true {
		t.Fatalf("shared overlay round-trip = %+v (defaultOn=%v)", ovs[0], ovs[0].DefaultOn)
	}
	if ovs[1].Owner != "m-marie" || ovs[1].DefaultOn == nil || *ovs[1].DefaultOn != false {
		t.Fatalf("personal overlay round-trip = %+v (defaultOn=%v)", ovs[1], ovs[1].DefaultOn)
	}
	mc := round.Songs[0].MemberCues
	if len(mc) != 2 || mc[0].MemberID != "m-marie" || len(mc[0].Cues) != 2 || mc[1].Cues[0].Icon != "tambourine" {
		t.Fatalf("memberCues round-trip = %+v", mc)
	}

	// --- Absent presence: owner "" and defaultOn nil are omitted (old loaders see nothing).
	bare := ConcertBundle{Songs: []BakedSong{{SongID: "s2",
		Pages: []PageImages{{Overlays: []LayerImage{{LayerID: "L9"}}}}}}}
	bd, err := bare.MarshalCanonical()
	if err != nil {
		t.Fatalf("marshal bare: %v", err)
	}
	if strings.Contains(string(bd), `"owner"`) || strings.Contains(string(bd), `"defaultOn"`) ||
		strings.Contains(string(bd), `"roster"`) || strings.Contains(string(bd), `"memberCues"`) {
		t.Fatalf("bare bundle must omit all P205 fields:\n%s", bd)
	}
}

package httpapi

import (
	"encoding/json"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/bake"
)

// viewOf must produce the proto AvailableConcert manifest shape (B03): 64-bit ints
// as JSON STRINGS (so A02's Kotlin AvailableConcert mirror parses it), per-song
// revs from each song's source revision, final_locked passthrough, + the download
// URL extra. White-box (same package) since viewOf is unexported.
func TestViewOf_availableConcertShape(t *testing.T) {
	cb := bake.ConcertBundle{
		ConcertID:   "sl1",
		Name:        "Spring Gig",
		ConcertRev:  7,
		BakedAt:     1700000000,
		FinalLocked: true,
		Songs: []bake.BakedSong{
			{SongID: "songA", SourceRevision: 3, Title: "Opener"},
			{SongID: "songB", SourceRevision: 5, Title: "Encore"},
		},
	}
	// T200: the base setlist supplies archived + eventDate + venue for the manifest row.
	base := app.Setlist{Archived: true, EventDate: "2026-05-01", Venue: "The Hall"}
	got, err := json.Marshal(viewOf("bandX", cb, base))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// Decode loosely to assert the wire types: 64-bit ints are STRINGS.
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(got, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if string(raw["currentRev"]) != `"7"` {
		t.Fatalf("currentRev = %s, want \"7\" (string-encoded)", raw["currentRev"])
	}
	if string(raw["updatedAt"]) != `"1700000000"` {
		t.Fatalf("updatedAt = %s, want string-encoded", raw["updatedAt"])
	}
	if string(raw["finalLocked"]) != "true" {
		t.Fatalf("finalLocked = %s, want true", raw["finalLocked"])
	}
	if string(raw["downloadUrl"]) != `"/api/bands/bandX/concerts/sl1/bundle"` {
		t.Fatalf("downloadUrl = %s", raw["downloadUrl"])
	}

	// Per-song revs are string-encoded too, in order; T200 adds the per-song title.
	var songs []struct {
		SongID string `json:"songId"`
		Rev    string `json:"rev"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(raw["songs"], &songs); err != nil {
		t.Fatalf("songs unmarshal: %v", err)
	}
	if len(songs) != 2 || songs[0].SongID != "songA" || songs[0].Rev != "3" || songs[1].Rev != "5" {
		t.Fatalf("per-song revs = %+v, want [{songA 3} {songB 5}]", songs)
	}
	if songs[0].Title != "Opener" || songs[1].Title != "Encore" {
		t.Fatalf("per-song titles = %+v, want [Opener Encore] (T200)", songs)
	}

	// T200: archived flag + the base setlist's eventDate/venue ride the manifest row.
	if string(raw["archived"]) != "true" {
		t.Fatalf("archived = %s, want true", raw["archived"])
	}
	if string(raw["eventDate"]) != `"2026-05-01"` {
		t.Fatalf("eventDate = %s, want \"2026-05-01\"", raw["eventDate"])
	}
	if string(raw["venue"]) != `"The Hall"` {
		t.Fatalf("venue = %s, want \"The Hall\"", raw["venue"])
	}
}

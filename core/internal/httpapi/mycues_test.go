package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// myCuesSetup registers+logs in alice, creates a band and a song, and returns the
// band + song. The caller is left authenticated as alice.
func myCuesSetup(t *testing.T, c *client) (app.Band, app.Song) {
	t.Helper()
	c.registerLogin("alice", "pw")
	_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
	var band app.Band
	unmarshalField(t, body, "band", &band)
	_, body = c.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "The Open Road"})
	var song app.Song
	unmarshalField(t, body, "song", &song)
	return band, song
}

func decodeCues(t *testing.T, body map[string]json.RawMessage) []app.SongCue {
	t.Helper()
	var cues []app.SongCue
	unmarshalField(t, body, "cues", &cues)
	return cues
}

func equalCues(a, b []app.SongCue) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestMyCuesDefaultEmpty: with nothing set, GET returns an empty list (not null).
func TestMyCuesDefaultEmpty(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			resp, body := c.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); len(got) != 0 {
				t.Fatalf("default cues = %v, want empty", got)
			}
			// And it must be [] not null in the wire bytes.
			if string(body["cues"]) != "[]" {
				t.Fatalf("cues wire = %s, want []", body["cues"])
			}
		})
	}
}

// TestMyCuesPutRoundtripOrder: PUT [mic, red guitar-electric] → GET returns exactly
// those, in that order, with tints preserved (deterministic order, both backends).
func TestMyCuesPutRoundtripOrder(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			want := []app.SongCue{
				{Icon: "mic", Color: ""},
				{Icon: "guitar-electric", Color: "#e11d48"},
			}
			resp, body := c.do(http.MethodPut, myCues, map[string]any{"cues": want})
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); !equalCues(got, want) {
				t.Fatalf("PUT response cues = %v, want %v", got, want)
			}

			resp, body = c.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); !equalCues(got, want) {
				t.Fatalf("GET cues = %v, want %v", got, want)
			}
		})
	}
}

// TestMyCuesEmptyClears: PUT an empty list clears the cues → GET is empty.
func TestMyCuesEmptyClears(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			resp, _ := c.do(http.MethodPut, myCues, map[string]any{"cues": []app.SongCue{{Icon: "mic"}}})
			mustStatus(t, resp, http.StatusOK)

			resp, body := c.do(http.MethodPut, myCues, map[string]any{"cues": []app.SongCue{}})
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); len(got) != 0 {
				t.Fatalf("after empty PUT cues = %v, want empty", got)
			}

			resp, body = c.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); len(got) != 0 {
				t.Fatalf("GET after clear cues = %v, want empty", got)
			}
		})
	}
}

// TestMyCuesValidation: an empty icon or a malformed color → 400.
func TestMyCuesValidation(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			resp, _ := c.do(http.MethodPut, myCues, map[string]any{"cues": []app.SongCue{{Icon: "", Color: "#ff0000"}}})
			mustStatus(t, resp, http.StatusBadRequest)

			resp, _ = c.do(http.MethodPut, myCues, map[string]any{"cues": []app.SongCue{{Icon: "mic", Color: "red"}}})
			mustStatus(t, resp, http.StatusBadRequest)

			// Nothing was stored on the rejected writes.
			resp, body := c.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); len(got) != 0 {
				t.Fatalf("cues after rejected writes = %v, want empty", got)
			}
		})
	}
}

// TestMyCuesUnknownIconAccepted: the model accepts any icon id (unknown ones render
// as the `note` fallback client-side, never a server error). Round-trips verbatim.
func TestMyCuesUnknownIconAccepted(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			want := []app.SongCue{{Icon: "theremin-2099", Color: "#00ffcc"}}
			resp, body := c.do(http.MethodPut, myCues, map[string]any{"cues": want})
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); !equalCues(got, want) {
				t.Fatalf("cues = %v, want %v (unknown icon must round-trip)", got, want)
			}
		})
	}
}

// TestMyCuesPerUserIsolation: userA's cues are private — userB never sees them and
// userA can only ever write their own (self-only by construction, no userId in path).
func TestMyCuesPerUserIsolation(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			userA := newClient(t, repo)
			band, song := myCuesSetup(t, userA)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			// userB joins the band.
			userB := newClient(t, repo)
			userB.registerLogin("bob", "pw")
			_, ibody := userA.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "bob", "kind": "username"})
			var inv app.Invite
			unmarshalField(t, ibody, "invite", &inv)
			resp, _ := userB.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			// userA sets cues.
			aWant := []app.SongCue{{Icon: "mic"}, {Icon: "guitar-electric", Color: "#e11d48"}}
			resp, _ = userA.do(http.MethodPut, myCues, map[string]any{"cues": aWant})
			mustStatus(t, resp, http.StatusOK)

			// userB sees NONE of A's cues.
			resp, body := userB.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); len(got) != 0 {
				t.Fatalf("userB sees cues %v, want empty (isolation breach)", got)
			}

			// userB sets their own; userA's remain unchanged.
			bWant := []app.SongCue{{Icon: "cajon"}}
			resp, _ = userB.do(http.MethodPut, myCues, map[string]any{"cues": bWant})
			mustStatus(t, resp, http.StatusOK)

			resp, body = userA.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusOK)
			if got := decodeCues(t, body); !equalCues(got, aWant) {
				t.Fatalf("userA cues = %v, want %v (B's write must not touch A's)", got, aWant)
			}
		})
	}
}

// TestMyCuesInListSongs: the caller's own cues ride each listSongs row as `myCues`.
func TestMyCuesInListSongs(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song := myCuesSetup(t, c)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			want := []app.SongCue{{Icon: "mic"}, {Icon: "bass", Color: "#2563eb"}}
			resp, _ := c.do(http.MethodPut, myCues, map[string]any{"cues": want})
			mustStatus(t, resp, http.StatusOK)

			resp, body := c.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
			mustStatus(t, resp, http.StatusOK)
			var songs []struct {
				ID     string        `json:"id"`
				MyCues []app.SongCue `json:"myCues"`
			}
			unmarshalField(t, body, "songs", &songs)
			if len(songs) != 1 || songs[0].ID != song.ID {
				t.Fatalf("listSongs = %+v, want the one song", songs)
			}
			if !equalCues(songs[0].MyCues, want) {
				t.Fatalf("row myCues = %v, want %v", songs[0].MyCues, want)
			}
		})
	}
}

// TestMyCuesNonMember: a non-member is forbidden on both verbs → 403.
func TestMyCuesNonMember(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			owner := newClient(t, repo)
			band, song := myCuesSetup(t, owner)
			myCues := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-cues"

			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")

			resp, _ := outsider.do(http.MethodGet, myCues, nil)
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = outsider.do(http.MethodPut, myCues, map[string]any{"cues": []app.SongCue{{Icon: "mic"}}})
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

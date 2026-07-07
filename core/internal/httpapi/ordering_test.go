package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestListingOrder_Deterministic (T22) pins that listings come back in a stable,
// sorted order on BOTH backends — not the Go-map-randomized order VLL saw. Songs
// sort by title (ci), setlists + bands by name; and the order must not change
// between identical requests.
func TestListingOrder_Deterministic(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Zeta Band")

			// Create songs in deliberately NON-alphabetical order, mixed case.
			for _, title := range []string{"Charlie", "alpha", "Bravo", "delta"} {
				admin.makeSong(band.ID, title)
			}
			wantSongs := []string{"alpha", "Bravo", "Charlie", "delta"} // ci lexicographic

			// Two identical requests must return the SAME sorted order.
			var first []string
			for call := 0; call < 2; call++ {
				_, body := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
				var songs []app.Song
				unmarshalField(t, body, "songs", &songs)
				got := make([]string, len(songs))
				for i, s := range songs {
					got[i] = s.Title
				}
				if call == 0 {
					first = got
					if !equal(got, wantSongs) {
						t.Fatalf("songs order = %v, want %v", got, wantSongs)
					}
				} else if !equal(got, first) {
					t.Fatalf("songs order not stable across calls: %v then %v", first, got)
				}
			}

			// Setlists sort by name (ci).
			for _, name := range []string{"Zulu Night", "afternoon", "Matinee"} {
				_, _ = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": name})
			}
			_, sb := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/setlists", nil)
			var setlists []app.Setlist
			unmarshalField(t, sb, "setlists", &setlists)
			gotSL := make([]string, len(setlists))
			for i, s := range setlists {
				gotSL[i] = s.Name
			}
			if !equal(gotSL, []string{"afternoon", "Matinee", "Zulu Night"}) {
				t.Fatalf("setlists order = %v, want [afternoon Matinee Zulu Night]", gotSL)
			}

			// Bands the caller belongs to sort by name (ci) — alice also owns "Zeta Band";
			// add an alphabetically-earlier one.
			admin.do(http.MethodPost, "/api/bands", map[string]string{"name": "Alpha Band"})
			_, bb := admin.do(http.MethodGet, "/api/bands", nil)
			var bands []app.Band
			unmarshalField(t, bb, "bands", &bands)
			gotB := make([]string, len(bands))
			for i, b := range bands {
				gotB[i] = b.Name
			}
			if !equal(gotB, []string{"Alpha Band", "Zeta Band"}) {
				t.Fatalf("bands order = %v, want [Alpha Band Zeta Band]", gotB)
			}

			// Invites + invite-links are user-visible on the band page and must come
			// back in a stable (CreatedAt, ID) order too (T22 fix-forward).
			for _, name := range []string{"bob", "carol", "dave", "erin"} {
				resp, _ := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
					map[string]string{"identifier": name, "kind": "username"})
				mustStatus(t, resp, http.StatusCreated)
			}
			assertInvitesStableSorted(t, admin, band.ID)

			for i := 0; i < 4; i++ {
				resp, _ := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invite-links",
					map[string]any{"role": "member", "maxUses": 0})
				mustStatus(t, resp, http.StatusCreated)
			}
			assertInviteLinksStableSorted(t, admin, band.ID)
		})
	}
}

// assertInvitesStableSorted fetches the band's invites twice and asserts the order
// is stable across calls and sorted by (CreatedAt, ID).
func assertInvitesStableSorted(t *testing.T, admin *client, bandID string) {
	t.Helper()
	var first []string
	for call := 0; call < 2; call++ {
		_, body := admin.do(http.MethodGet, "/api/bands/"+bandID+"/invites", nil)
		var invites []app.Invite
		unmarshalField(t, body, "invites", &invites)
		if len(invites) != 4 {
			t.Fatalf("want 4 invites, got %d", len(invites))
		}
		for i := 1; i < len(invites); i++ {
			p, c := invites[i-1], invites[i]
			if c.CreatedAt.Before(p.CreatedAt) || (c.CreatedAt.Equal(p.CreatedAt) && c.ID < p.ID) {
				t.Fatalf("invites not sorted by (CreatedAt, ID): %+v then %+v", p, c)
			}
		}
		ids := make([]string, len(invites))
		for i, inv := range invites {
			ids[i] = inv.ID
		}
		if call == 0 {
			first = ids
		} else if !equal(ids, first) {
			t.Fatalf("invites order not stable across calls: %v then %v", first, ids)
		}
	}
}

// assertInviteLinksStableSorted fetches the band's invite links twice and asserts
// the order is stable across calls and sorted by (CreatedAt, ID).
func assertInviteLinksStableSorted(t *testing.T, admin *client, bandID string) {
	t.Helper()
	var first []string
	for call := 0; call < 2; call++ {
		_, body := admin.do(http.MethodGet, "/api/bands/"+bandID+"/invite-links", nil)
		var links []struct {
			ID        string `json:"id"`
			CreatedAt string `json:"createdAt"`
		}
		unmarshalField(t, body, "links", &links)
		if len(links) != 4 {
			t.Fatalf("want 4 invite links, got %d", len(links))
		}
		for i := 1; i < len(links); i++ {
			p, c := links[i-1], links[i]
			// createdAt is RFC3339; lexicographic compare matches chronological, with
			// the ID tiebreak on equal timestamps.
			if c.CreatedAt < p.CreatedAt || (c.CreatedAt == p.CreatedAt && c.ID < p.ID) {
				t.Fatalf("invite links not sorted by (CreatedAt, ID): %+v then %+v", p, c)
			}
		}
		ids := make([]string, len(links))
		for i, l := range links {
			ids[i] = l.ID
		}
		if call == 0 {
			first = ids
		} else if !equal(ids, first) {
			t.Fatalf("invite links order not stable across calls: %v then %v", first, ids)
		}
	}
}

func equal(a, b []string) bool {
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

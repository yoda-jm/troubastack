package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestLineupAPI (T203 stage 1): the musicians/parts endpoints + their gate — the roster is admin, a member
// may create a part and edit a line-up.
func TestLineupAPI(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			_, member := admin.addMember(t, repo, admin, band.ID, "bob")
			song := admin.makeSong(band.ID, "Song")

			base := "/api/bands/" + band.ID

			// Roster is admin: a member can't add a musician; an admin can.
			resp, _ := member.do(http.MethodPost, base+"/musicians", map[string]string{"name": "Guest"})
			mustStatus(t, resp, http.StatusForbidden)
			resp, mbody := admin.do(http.MethodPost, base+"/musicians", map[string]string{"name": "Guest"})
			mustStatus(t, resp, http.StatusCreated)
			var guest app.Musician
			unmarshalField(t, mbody, "musician", &guest)

			// The roster lists members (auto-appearing) + the guest.
			resp, lbody := member.do(http.MethodGet, base+"/musicians", nil)
			mustStatus(t, resp, http.StatusOK)
			var roster []app.MusicianView
			unmarshalField(t, lbody, "musicians", &roster)
			if len(roster) < 3 { // alice + bob + guest
				t.Fatalf("roster should list the two members + the guest, got %d", len(roster))
			}

			// A member MAY create a part...
			resp, pbody := member.do(http.MethodPost, base+"/parts", map[string]string{"name": "Autoharp"})
			mustStatus(t, resp, http.StatusCreated)
			var part app.Part
			unmarshalField(t, pbody, "part", &part)
			// ...but not edit its metadata.
			resp, _ = member.do(http.MethodPatch, base+"/parts/"+part.ID, map[string]any{"kind": "voice"})
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = admin.do(http.MethodPatch, base+"/parts/"+part.ID, map[string]any{"input": "di"})
			mustStatus(t, resp, http.StatusOK)

			// A member edits a line-up via the song PATCH.
			resp, sbody := member.do(http.MethodPatch, base+"/songs/"+song.ID, map[string]any{
				"lineup":    []map[string]any{{"musicianId": guest.ID, "partIds": []string{part.ID}}},
				"soundNote": "autoharp only",
			})
			mustStatus(t, resp, http.StatusOK)
			var out struct {
				Song app.Song `json:"song"`
			}
			unmarshalField(t, sbody, "song", &out.Song)
			if out.Song.SoundNote != "autoharp only" || len(out.Song.Lineup) != 1 {
				t.Fatalf("lineup PATCH not applied: %+v", out.Song)
			}
		})
	}
}

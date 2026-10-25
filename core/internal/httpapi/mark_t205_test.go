package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// T205: the "Made with TroubaStack" setting is admin-only, carried on PATCH /api/bands/{id} as the optional
// hidePdfMark field, and a name-only PATCH leaves it intact (the fields are independently optional).
func TestUpdateBand_HidePdfMark_T205(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			_, member := admin.addMember(t, repo, admin, band.ID, "bob")
			base := "/api/bands/" + band.ID

			// A member may not change it.
			resp, _ := member.do(http.MethodPatch, base, map[string]any{"hidePdfMark": true})
			mustStatus(t, resp, http.StatusForbidden)

			// The admin can; the band reflects it. (Fresh struct per decode — hidePdfMark is omitempty, so a
			// false value is simply absent from the response, and the Studio reads an absent flag as "shown".)
			resp, body := admin.do(http.MethodPatch, base, map[string]any{"hidePdfMark": true})
			mustStatus(t, resp, http.StatusOK)
			var set app.Band
			unmarshalField(t, body, "band", &set)
			if !set.HidePdfMark {
				t.Fatal("hidePdfMark not set after the admin PATCH")
			}

			// A name-only PATCH must not reset the flag (independently optional fields).
			resp, body = admin.do(http.MethodPatch, base, map[string]any{"name": "Renamed"})
			mustStatus(t, resp, http.StatusOK)
			var renamed app.Band
			unmarshalField(t, body, "band", &renamed)
			if renamed.Name != "Renamed" || !renamed.HidePdfMark {
				t.Fatalf("name-only PATCH changed the flag: name=%q hidePdfMark=%v", renamed.Name, renamed.HidePdfMark)
			}

			// And it can be turned back on (shown): the flag clears (absent from the omitempty response).
			resp, body = admin.do(http.MethodPatch, base, map[string]any{"hidePdfMark": false})
			mustStatus(t, resp, http.StatusOK)
			var shown app.Band
			unmarshalField(t, body, "band", &shown)
			if shown.HidePdfMark {
				t.Fatal("hidePdfMark still set after turning it off")
			}
		})
	}
}

package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// T131: the concert list carries songCount for every setlist, and lastBakedAt + downloadUrl ONLY when a
// bake exists. The never-baked null case is the one that breaks UIs (a menu item leading to a 404), so
// it is asserted first; then a bake makes the state appear on that setlist alone.
func TestListSetlists_CountsAndBakeState_T131(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")

	mkSetlist := func(name string) string {
		_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": name})
		var sl app.Setlist
		unmarshalField(t, body, "setlist", &sl)
		return sl.ID
	}
	aID := mkSetlist("Has A Song")
	seedSongInSetlist(t, admin, band.ID, aID) // one song, not yet baked
	bID := mkSetlist("Empty")                 // zero songs

	type row struct {
		ID          string `json:"id"`
		SongCount   int    `json:"songCount"`
		LastBakedAt *int64 `json:"lastBakedAt"`
		DownloadURL string `json:"downloadUrl"`
	}
	list := func() map[string]row {
		t.Helper()
		_, body := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/setlists", nil)
		var rows []row
		unmarshalField(t, body, "setlists", &rows)
		by := map[string]row{}
		for _, r := range rows {
			by[r.ID] = r
		}
		return by
	}

	// (1) Never baked: counts are present; the bake state is ABSENT (the null case a UI must handle).
	by := list()
	if by[aID].SongCount != 1 {
		t.Errorf("setlist A songCount = %d, want 1", by[aID].SongCount)
	}
	if by[bID].SongCount != 0 {
		t.Errorf("empty setlist B songCount = %d, want 0", by[bID].SongCount)
	}
	if by[aID].LastBakedAt != nil || by[aID].DownloadURL != "" {
		t.Errorf("never-baked setlist carries bake state: lastBakedAt=%v downloadUrl=%q",
			by[aID].LastBakedAt, by[aID].DownloadURL)
	}

	// (2) After a bake: lastBakedAt + downloadUrl appear on THAT setlist only.
	awaitBake(t, admin, band.ID, aID)
	by = list()
	if by[aID].LastBakedAt == nil || *by[aID].LastBakedAt == 0 {
		t.Errorf("baked setlist A: lastBakedAt = %v, want a timestamp", by[aID].LastBakedAt)
	}
	if by[aID].DownloadURL == "" {
		t.Errorf("baked setlist A: downloadUrl empty, want a link")
	}
	if by[bID].LastBakedAt != nil || by[bID].DownloadURL != "" {
		t.Errorf("unbaked setlist B gained bake state: lastBakedAt=%v downloadUrl=%q",
			by[bID].LastBakedAt, by[bID].DownloadURL)
	}
}

// TestSetlistArchive_HTTP (T200): the archived flag over PATCH is admin-only, while the metadata fields
// stay member-level. (The archive side effects — stamp + live off — are covered at the app layer.)
func TestSetlistArchive_HTTP(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			_, member := admin.addMember(t, repo, admin, band.ID, "bob")

			_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)
			base := "/api/bands/" + band.ID + "/setlists/" + sl.ID

			// A non-admin member cannot archive.
			resp, _ := member.do(http.MethodPatch, base, map[string]any{"archived": true})
			mustStatus(t, resp, http.StatusForbidden)

			// ...but may still edit metadata.
			resp, _ = member.do(http.MethodPatch, base, map[string]string{"name": "Renamed"})
			mustStatus(t, resp, http.StatusOK)

			// An admin archives → 200 and the flag is set.
			resp, abody := admin.do(http.MethodPatch, base, map[string]any{"archived": true})
			mustStatus(t, resp, http.StatusOK)
			var got app.Setlist
			unmarshalField(t, abody, "setlist", &got)
			if !got.Archived {
				t.Fatalf("admin archive: setlist.archived = false, want true")
			}

			// ...and unarchives. (archived:omitempty omits the false field, so decode into a FRESH
			// zero-valued struct — reusing `got` would keep the stale true.)
			resp, abody = admin.do(http.MethodPatch, base, map[string]any{"archived": false})
			mustStatus(t, resp, http.StatusOK)
			var after app.Setlist
			unmarshalField(t, abody, "setlist", &after)
			if after.Archived {
				t.Fatalf("admin unarchive: setlist.archived = true, want false")
			}
		})
	}
}

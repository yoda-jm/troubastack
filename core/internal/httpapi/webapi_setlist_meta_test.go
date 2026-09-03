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

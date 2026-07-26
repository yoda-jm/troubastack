package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestT60Endpoints_Authz (T64): the two T60 endpoints — chart-source:transpose and the
// setlist-item chart-preview — reject a non-member (403/404) and 404 a cross-band file/item
// id used under a band the caller DOES belong to. Enforcement already lives in the service;
// these lock it against a future refactor (the audit flagged the missing negative coverage).
func TestT60Endpoints_Authz(t *testing.T) {
	srv := bakeServer(t)

	// Band A: alice's band with a generated chart + a setlist item.
	alice := &client{t: t, srv: srv}
	bandA := alice.makeBand("alice", "A")
	songA := alice.makeSong(bandA.ID, "Song A")
	fileA := makeChart(t, alice, bandA.ID, songA.ID)
	itemA := makeSetlistItem(t, alice, bandA.ID, songA.ID)

	// Band B: dave's separate band, its own chart + item (the cross-band ids).
	dave := &client{t: t, srv: srv}
	bandB := dave.makeBand("dave", "B")
	songB := dave.makeSong(bandB.ID, "Song B")
	fileB := makeChart(t, dave, bandB.ID, songB.ID)
	itemB := makeSetlistItem(t, dave, bandB.ID, songB.ID)

	// Outsider: a real account in NO band.
	outsider := &client{t: t, srv: srv}
	outsider.registerLogin("mallory", "pw")

	transposeURL := func(bandID, songID, fileID string) string {
		return "/api/bands/" + bandID + "/songs/" + songID + "/files/" + fileID + "/chart-source:transpose"
	}
	previewURL := func(bandID, setlistID, itemID string) string {
		return "/api/bands/" + bandID + "/setlists/" + setlistID + "/items/" + itemID + "/chart-preview"
	}

	// --- chart-source:transpose ---
	// Non-member → 403/404 (never the transposed source).
	resp, _ := outsider.do(http.MethodPost, transposeURL(bandA.ID, songA.ID, fileA.ID),
		map[string]any{"targetKey": "D", "dryRun": true})
	mustDenied(t, resp)
	// A member passing a file id from ANOTHER band under their own band → 404.
	resp, _ = alice.do(http.MethodPost, transposeURL(bandA.ID, songA.ID, fileB.ID),
		map[string]any{"targetKey": "D", "dryRun": true})
	mustStatus(t, resp, http.StatusNotFound)

	// --- item chart-preview ---
	resp, _ = outsider.do(http.MethodGet, previewURL(bandA.ID, itemA.SetlistID, itemA.ID), nil)
	mustDenied(t, resp)
	// A cross-band item id under alice's band → 404.
	resp, _ = alice.do(http.MethodGet, previewURL(bandA.ID, itemA.SetlistID, itemB.ID), nil)
	mustStatus(t, resp, http.StatusNotFound)
}

// makeChart creates a generated text chart on a song and returns the file.
func makeChart(t *testing.T, c *client, bandID, songID string) app.SongFile {
	t.Helper()
	resp, body := c.do(http.MethodPost, "/api/bands/"+bandID+"/songs/"+songID+"/text-charts",
		map[string]string{"source": "# S\n## Verse\nC            G\nwords\n"})
	mustStatus(t, resp, http.StatusCreated)
	var f app.SongFile
	unmarshalField(t, body, "file", &f)
	return f
}

// makeSetlistItem creates a one-item setlist for a song and returns the item.
func makeSetlistItem(t *testing.T, c *client, bandID, songID string) app.SetlistItem {
	t.Helper()
	_, sb := c.do(http.MethodPost, "/api/bands/"+bandID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, sb, "setlist", &sl)
	resp, ib := c.do(http.MethodPost, "/api/bands/"+bandID+"/setlists/"+sl.ID+"/items",
		map[string]string{"songId": songID})
	mustStatus(t, resp, http.StatusCreated)
	var item app.SetlistItem
	unmarshalField(t, ib, "item", &item)
	item.SetlistID = sl.ID
	return item
}

// mustDenied asserts a non-member was refused (403 or 404 — both hide the resource).
func mustDenied(t *testing.T, resp *http.Response) {
	t.Helper()
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 403 or 404 (denied)", resp.StatusCode)
	}
}

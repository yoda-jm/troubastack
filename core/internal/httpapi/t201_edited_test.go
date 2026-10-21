package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// editView reads the T201 staleness fields off a concert in GET /concerts.
type editView struct {
	ConcertID       string `json:"concertId"`
	EditedSinceBake *bool  `json:"editedSinceBake"`
	Changes         *struct {
		Setlist bool `json:"setlist"`
		Songs   []struct {
			SongID string `json:"songId"`
			Title  string `json:"title"`
			What   string `json:"what"`
		} `json:"songs"`
	} `json:"changes"`
}

func concertEdit(t *testing.T, c *client, bandID, concertID string) editView {
	t.Helper()
	resp, body := c.do(http.MethodGet, "/api/bands/"+bandID+"/concerts", nil)
	mustStatus(t, resp, http.StatusOK)
	var cs []editView
	unmarshalField(t, body, "concerts", &cs)
	for _, cv := range cs {
		if cv.ConcertID == concertID {
			return cv
		}
	}
	t.Fatalf("concert %s not found in manifest", concertID)
	return editView{}
}

func seedSong(t *testing.T, c *client, bandID, setlistID, title string) string {
	t.Helper()
	_, body := c.do(http.MethodPost, "/api/bands/"+bandID+"/songs", map[string]string{"title": title})
	var song app.Song
	unmarshalField(t, body, "song", &song)
	if resp, _ := c.upload("/api/bands/"+bandID+"/songs/"+song.ID+"/files", "score.pdf", "application/pdf", smallPDF); resp.StatusCode >= 300 {
		t.Fatalf("upload: %d", resp.StatusCode)
	}
	if resp, _ := c.do(http.MethodPost, "/api/bands/"+bandID+"/setlists/"+setlistID+"/items", map[string]string{"songId": song.ID}); resp.StatusCode >= 300 {
		t.Fatalf("add item: %d", resp.StatusCode)
	}
	return song.ID
}

// TestEditedSinceBake exercises the whole T201 signal through GET /concerts: a fresh bake is not edited, a
// NO-OP save stays false (the regression VLL hit), real edits flip it with the right song + what, a re-bake
// clears it, an edit to a song not in the setlist does not, archived is not reported, and a member sees
// nothing.
func TestEditedSinceBake(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	songA := seedSong(t, admin, band.ID, sl.ID, "First")
	awaitBake(t, admin, band.ID, sl.ID)

	// Fresh bake → not edited.
	if e := concertEdit(t, admin, band.ID, sl.ID); e.EditedSinceBake == nil || *e.EditedSinceBake {
		t.Fatalf("fresh bake must be not-edited, got %v", e.EditedSinceBake)
	}

	// THE regression: a save that writes the SAME title changes nothing → stays false.
	admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+songA, map[string]any{"title": "First"})
	if e := concertEdit(t, admin, band.ID, sl.ID); e.EditedSinceBake == nil || *e.EditedSinceBake {
		t.Fatalf("a no-op save must stay false, got %v", e.EditedSinceBake)
	}

	// A real edit: rename the song → content, for that song, not a setlist change.
	admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+songA, map[string]any{"title": "Renamed"})
	e := concertEdit(t, admin, band.ID, sl.ID)
	if e.EditedSinceBake == nil || !*e.EditedSinceBake {
		t.Fatal("renaming a song must read as edited")
	}
	if e.Changes == nil || e.Changes.Setlist || len(e.Changes.Songs) != 1 || e.Changes.Songs[0].SongID != songA || e.Changes.Songs[0].What != "content" {
		t.Fatalf("want one content change for the song, got %+v", e.Changes)
	}

	// Re-bake → cleared.
	awaitBake(t, admin, band.ID, sl.ID)
	if e := concertEdit(t, admin, band.ID, sl.ID); e.EditedSinceBake == nil || *e.EditedSinceBake {
		t.Fatalf("a re-bake must clear the signal, got %v", e.EditedSinceBake)
	}

	// An edit to a song NOT in the setlist does not flip this concert.
	other := admin.makeSong(band.ID, "Loose")
	admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+other.ID, map[string]any{"title": "Loose II"})
	if e := concertEdit(t, admin, band.ID, sl.ID); e.EditedSinceBake == nil || *e.EditedSinceBake {
		t.Fatalf("an edit to a song not in the setlist must not flip the concert, got %v", e.EditedSinceBake)
	}

	// Adding a song → added + setlist:true.
	songB := seedSong(t, admin, band.ID, sl.ID, "Second")
	e = concertEdit(t, admin, band.ID, sl.ID)
	if e.EditedSinceBake == nil || !*e.EditedSinceBake || e.Changes == nil || !e.Changes.Setlist {
		t.Fatalf("adding a song must read edited + setlist:true, got %+v", e)
	}
	foundAdded := false
	for _, s := range e.Changes.Songs {
		if s.SongID == songB && s.What == "added" {
			foundAdded = true
		}
	}
	if !foundAdded {
		t.Fatalf("want an 'added' change for the new song, got %+v", e.Changes.Songs)
	}

	// A member sees the concert but NOT the staleness fields (admin-only; no leak).
	member := &client{t: t, srv: srv}
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")
	if e := concertEdit(t, member, band.ID, sl.ID); e.EditedSinceBake != nil || e.Changes != nil {
		t.Fatalf("a member must not see editedSinceBake/changes, got %+v", e)
	}

	// An archived concert is not reported (even to an admin).
	admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/setlists/"+sl.ID, map[string]any{"archived": true})
	if e := concertEdit(t, admin, band.ID, sl.ID); e.EditedSinceBake != nil {
		t.Fatalf("an archived concert must not report editedSinceBake, got %v", *e.EditedSinceBake)
	}
}

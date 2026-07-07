package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// ---- test helpers specific to the settings & setlists features ----

// makeBand registers+logs in `username` and creates a band, returning it.
func (c *client) makeBand(username, name string) app.Band {
	c.t.Helper()
	c.clearCookies()
	c.registerLogin(username, "pw")
	_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": name})
	var b app.Band
	unmarshalField(c.t, body, "band", &b)
	return b
}

// makeSong creates a song in a band (caller's cookie must already be set).
func (c *client) makeSong(bandID, title string) app.Song {
	c.t.Helper()
	_, body := c.do(http.MethodPost, "/api/bands/"+bandID+"/songs", map[string]string{"title": title})
	var s app.Song
	unmarshalField(c.t, body, "song", &s)
	return s
}

// invite admin invites identifier (username) and the joiner accepts it, becoming a
// member. `joiner` must be a logged-in client for that username.
func inviteAndAccept(t *testing.T, admin, joiner *client, bandID, username string) {
	t.Helper()
	resp, body := admin.do(http.MethodPost, "/api/bands/"+bandID+"/invites",
		map[string]string{"identifier": username, "kind": "username"})
	mustStatus(t, resp, http.StatusCreated)
	var inv app.Invite
	unmarshalField(t, body, "invite", &inv)
	resp, _ = joiner.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
	mustStatus(t, resp, http.StatusOK)
}

// ---- songs: edit metadata + delete ----

func TestSongEditAndDelete(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Wonderwall")

			// member (bob) joins
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			inviteAndAccept(t, admin, bob, band.ID, "bob")

			// member edits metadata → 200
			resp, body := bob.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+song.ID, map[string]any{
				"key": "G", "tempo": 120, "tags": []string{"rock", "90s"}, "notes": "capo 2", "artist": "Oasis",
			})
			mustStatus(t, resp, http.StatusOK)
			var got app.Song
			unmarshalField(t, body, "song", &got)
			if got.Key != "G" || got.Tempo != 120 || got.Artist != "Oasis" || got.Notes != "capo 2" {
				t.Fatalf("patched song wrong: %+v", got)
			}
			if len(got.Tags) != 2 || got.Tags[0] != "rock" {
				t.Fatalf("tags wrong: %+v", got.Tags)
			}

			// empty title → 400
			resp, _ = bob.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+song.ID, map[string]any{"title": "  "})
			mustStatus(t, resp, http.StatusBadRequest)

			// non-member edit → 403
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodPatch, "/api/bands/"+band.ID+"/songs/"+song.ID, map[string]any{"key": "C"})
			mustStatus(t, resp, http.StatusForbidden)

			// member (bob) delete → 403 (delete is admin-only)
			resp, _ = bob.do(http.MethodDelete, "/api/bands/"+band.ID+"/songs/"+song.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)

			// admin delete → 204
			resp, _ = admin.do(http.MethodDelete, "/api/bands/"+band.ID+"/songs/"+song.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)

			// gone from list
			resp, body = admin.do(http.MethodGet, "/api/bands/"+band.ID+"/songs", nil)
			mustStatus(t, resp, http.StatusOK)
			var songs []app.Song
			unmarshalField(t, body, "songs", &songs)
			if len(songs) != 0 {
				t.Fatalf("expected 0 songs after delete, got %d", len(songs))
			}
		})
	}
}

// ---- song files: rename, reorder, delete (with blob dereference) ----

func TestSongFileRenameReorderDelete(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			member := newClient(t, repo)
			band := member.makeBand("alice", "Band")
			song := member.makeSong(band.ID, "Wonderwall")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"

			resp, body := member.upload(base, "a.pdf", "application/pdf", smallPDF)
			mustStatus(t, resp, http.StatusCreated)
			var f app.SongFile
			unmarshalField(t, body, "file", &f)

			// rename + reorder → 200
			resp, body = member.do(http.MethodPatch, base+"/"+f.ID, map[string]any{
				"filename": "renamed.pdf", "displayOrder": 5,
			})
			mustStatus(t, resp, http.StatusOK)
			var got app.SongFile
			unmarshalField(t, body, "file", &got)
			if got.Filename != "renamed.pdf" || got.DisplayOrder != 5 {
				t.Fatalf("rename/reorder wrong: %+v", got)
			}

			// empty filename → 400
			resp, _ = member.do(http.MethodPatch, base+"/"+f.ID, map[string]any{"filename": " "})
			mustStatus(t, resp, http.StatusBadRequest)

			// delete file → 204; blob now unreferenced and dereferenced (download 404)
			resp, _ = member.do(http.MethodDelete, base+"/"+f.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = member.getRaw("/api/files/" + f.ID)
			mustStatus(t, resp, http.StatusNotFound)

			// non-member delete → 403
			resp2, body2 := member.upload(base, "b.pdf", "application/pdf", smallPDF)
			mustStatus(t, resp2, http.StatusCreated)
			var f2 app.SongFile
			unmarshalField(t, body2, "file", &f2)
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodDelete, base+"/"+f2.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

// blob stays referenced when a second file shares the same bytes.
func TestSongFileDeleteKeepsSharedBlob(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			member := newClient(t, repo)
			band := member.makeBand("alice", "Band")
			song := member.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"

			_, b1 := member.upload(base, "one.pdf", "application/pdf", smallPDF)
			var f1 app.SongFile
			unmarshalField(t, b1, "file", &f1)
			_, b2 := member.upload(base, "two.pdf", "application/pdf", smallPDF) // identical bytes
			var f2 app.SongFile
			unmarshalField(t, b2, "file", &f2)

			// delete f1; f2 still references the shared blob, so f2 downloads fine
			resp, _ := member.do(http.MethodDelete, base+"/"+f1.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = member.getRaw("/api/files/" + f2.ID)
			mustStatus(t, resp, http.StatusOK)
		})
	}
}

// ---- band rename + delete ----

func TestBandRenameAndDelete(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Old Name")

			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			inviteAndAccept(t, admin, bob, band.ID, "bob")

			// member rename → 403
			resp, _ := bob.do(http.MethodPatch, "/api/bands/"+band.ID, map[string]string{"name": "Hacked"})
			mustStatus(t, resp, http.StatusForbidden)

			// admin rename → 200
			resp, body := admin.do(http.MethodPatch, "/api/bands/"+band.ID, map[string]string{"name": "New Name"})
			mustStatus(t, resp, http.StatusOK)
			var b app.Band
			unmarshalField(t, body, "band", &b)
			if b.Name != "New Name" {
				t.Fatalf("rename wrong: %+v", b)
			}

			// empty name → 400
			resp, _ = admin.do(http.MethodPatch, "/api/bands/"+band.ID, map[string]string{"name": " "})
			mustStatus(t, resp, http.StatusBadRequest)

			// member delete → 403
			resp, _ = bob.do(http.MethodDelete, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)

			// admin delete → 204, then gone (404)
			resp, _ = admin.do(http.MethodDelete, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = admin.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}

// cascade: deleting a band removes its songs, files (blobs) and setlists.
func TestBandDeleteCascade(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			song := admin.makeSong(band.ID, "Song")
			base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"
			_, fb := admin.upload(base, "x.pdf", "application/pdf", smallPDF)
			var f app.SongFile
			unmarshalField(t, fb, "file", &f)
			_, sb := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Show"})
			var sl app.Setlist
			unmarshalField(t, sb, "setlist", &sl)

			resp, _ := admin.do(http.MethodDelete, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)

			// blob bytes dereferenced (file download 404)
			resp, _ = admin.getRaw("/api/files/" + f.ID)
			mustStatus(t, resp, http.StatusNotFound)
			// band fully gone
			resp, _ = admin.do(http.MethodGet, "/api/bands/"+band.ID+"/setlists/"+sl.ID, nil)
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}

// ---- members: role change + last-admin protections ----

func TestMemberRoleAndLastAdminProtections(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")

			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			inviteAndAccept(t, admin, bob, band.ID, "bob")

			// resolve bob's userId from the members list
			_, body := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/members", nil)
			var members []app.MemberView
			unmarshalField(t, body, "members", &members)
			var bobID, aliceID string
			for _, m := range members {
				switch m.User.Username {
				case "bob":
					bobID = m.User.ID
				case "alice":
					aliceID = m.User.ID
				}
			}
			if bobID == "" || aliceID == "" {
				t.Fatalf("could not resolve member ids: %+v", members)
			}

			// member (bob) cannot change roles → 403
			resp, _ := bob.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+aliceID, map[string]string{"role": "member"})
			mustStatus(t, resp, http.StatusForbidden)

			// invalid role → 400
			resp, _ = admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+bobID, map[string]string{"role": "wizard"})
			mustStatus(t, resp, http.StatusBadRequest)

			// admin promotes bob to admin → 200
			resp, body = admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+bobID, map[string]string{"role": "admin"})
			mustStatus(t, resp, http.StatusOK)
			var m app.Membership
			unmarshalField(t, body, "member", &m)
			if m.Role != app.RoleAdmin {
				t.Fatalf("bob role = %q, want admin", m.Role)
			}

			// now demote bob back to member → 200 (alice is still admin)
			resp, _ = admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+bobID, map[string]string{"role": "member"})
			mustStatus(t, resp, http.StatusOK)

			// alice is the LAST admin: cannot demote self → 409
			resp, _ = admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+aliceID, map[string]string{"role": "member"})
			mustStatus(t, resp, http.StatusConflict)

			// alice cannot be removed as last admin → 409
			resp, _ = admin.do(http.MethodDelete, "/api/bands/"+band.ID+"/members/"+aliceID, nil)
			mustStatus(t, resp, http.StatusConflict)

			// alice cannot leave as last admin → 409
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/leave", nil)
			mustStatus(t, resp, http.StatusConflict)

			// admin removes bob (a member) → 204
			resp, _ = admin.do(http.MethodDelete, "/api/bands/"+band.ID+"/members/"+bobID, nil)
			mustStatus(t, resp, http.StatusNoContent)
			// bob can no longer read the band
			resp, _ = bob.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

// a non-last admin CAN leave.
func TestLeaveBandNonLastAdmin(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			inviteAndAccept(t, admin, bob, band.ID, "bob")

			// promote bob → two admins
			_, body := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/members", nil)
			var members []app.MemberView
			unmarshalField(t, body, "members", &members)
			var bobID string
			for _, m := range members {
				if m.User.Username == "bob" {
					bobID = m.User.ID
				}
			}
			resp, _ := admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/members/"+bobID, map[string]string{"role": "admin"})
			mustStatus(t, resp, http.StatusOK)

			// alice leaves (bob remains admin) → 204
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/leave", nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = admin.do(http.MethodGet, "/api/bands/"+band.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

// ---- band invites: list + revoke (admin only) ----

func TestBandInvitesListAndRevoke(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")

			// create two pending invites
			resp, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "bob", "kind": "username"})
			mustStatus(t, resp, http.StatusCreated)
			var inv app.Invite
			unmarshalField(t, body, "invite", &inv)
			resp, _ = admin.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "carol", "kind": "username"})
			mustStatus(t, resp, http.StatusCreated)

			// admin lists this band's pending invites → 2
			resp, body = admin.do(http.MethodGet, "/api/bands/"+band.ID+"/invites", nil)
			mustStatus(t, resp, http.StatusOK)
			var invites []app.Invite
			unmarshalField(t, body, "invites", &invites)
			if len(invites) != 2 {
				t.Fatalf("expected 2 pending invites, got %d", len(invites))
			}

			// non-admin/non-member cannot list → 403
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodGet, "/api/bands/"+band.ID+"/invites", nil)
			mustStatus(t, resp, http.StatusForbidden)
			// non-admin cannot revoke → 403
			resp, _ = outsider.do(http.MethodDelete, "/api/bands/"+band.ID+"/invites/"+inv.ID, nil)
			mustStatus(t, resp, http.StatusForbidden)

			// admin revokes one → 204, then list shows 1
			resp, _ = admin.do(http.MethodDelete, "/api/bands/"+band.ID+"/invites/"+inv.ID, nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, body = admin.do(http.MethodGet, "/api/bands/"+band.ID+"/invites", nil)
			mustStatus(t, resp, http.StatusOK)
			unmarshalField(t, body, "invites", &invites)
			if len(invites) != 1 {
				t.Fatalf("expected 1 pending invite after revoke, got %d", len(invites))
			}
		})
	}
}

// ---- setlists: full lifecycle ----

func TestSetlistLifecycle(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			s1 := admin.makeSong(band.ID, "Song One")
			s2 := admin.makeSong(band.ID, "Song Two")
			s3 := admin.makeSong(band.ID, "Song Three")

			// member bob joins
			bob := newClient(t, repo)
			bob.registerLogin("bob", "pw")
			inviteAndAccept(t, admin, bob, band.ID, "bob")

			// non-member 403 on list
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ := outsider.do(http.MethodGet, "/api/bands/"+band.ID+"/setlists", nil)
			mustStatus(t, resp, http.StatusForbidden)

			// member creates a setlist → 201
			resp, body := bob.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{
				"name": "Friday Gig", "eventDate": "2026-07-04", "venue": "The Club",
			})
			mustStatus(t, resp, http.StatusCreated)
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)
			if sl.Name != "Friday Gig" || sl.EventDate != "2026-07-04" || sl.Venue != "The Club" {
				t.Fatalf("bad setlist: %+v", sl)
			}

			// bad eventDate → 400
			resp, _ = bob.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{
				"name": "Bad", "eventDate": "07/04/2026",
			})
			mustStatus(t, resp, http.StatusBadRequest)

			slBase := "/api/bands/" + band.ID + "/setlists/" + sl.ID

			// add 3 songs (appends at end)
			ids := make([]string, 0, 3)
			for _, sng := range []app.Song{s1, s2, s3} {
				resp, body = bob.do(http.MethodPost, slBase+"/items", map[string]string{"songId": sng.ID})
				mustStatus(t, resp, http.StatusCreated)
				var it app.SetlistItem
				unmarshalField(t, body, "item", &it)
				ids = append(ids, it.ID)
			}

			// GET detail → items sorted by position, enriched with song titles
			resp, body = bob.do(http.MethodGet, slBase, nil)
			mustStatus(t, resp, http.StatusOK)
			var items []struct {
				ID        string `json:"id"`
				Position  int    `json:"position"`
				SongTitle string `json:"songTitle"`
			}
			unmarshalField(t, body, "items", &items)
			if len(items) != 3 {
				t.Fatalf("expected 3 items, got %d", len(items))
			}
			for i, it := range items {
				if it.Position != i {
					t.Fatalf("item %d position = %d, want %d", i, it.Position, i)
				}
			}
			if items[0].SongTitle != "Song One" {
				t.Fatalf("first item title = %q, want Song One", items[0].SongTitle)
			}

			// reorder: reverse → 200, positions reassigned
			reversed := []string{ids[2], ids[1], ids[0]}
			resp, body = bob.do(http.MethodPost, slBase+"/reorder", map[string]any{"orderedItemIds": reversed})
			mustStatus(t, resp, http.StatusOK)
			var reordered []app.SetlistItem
			unmarshalField(t, body, "items", &reordered)
			if len(reordered) != 3 || reordered[0].ID != ids[2] || reordered[0].Position != 0 {
				t.Fatalf("reorder result wrong: %+v", reordered)
			}

			// reorder with wrong set → 400
			resp, _ = bob.do(http.MethodPost, slBase+"/reorder", map[string]any{"orderedItemIds": []string{ids[0], ids[1]}})
			mustStatus(t, resp, http.StatusBadRequest)

			// override key/tempo on an item → 200
			resp, body = bob.do(http.MethodPatch, slBase+"/items/"+ids[0], map[string]any{
				"keyOverride": "Bb", "tempoOverride": 140, "notes": "half-time feel",
			})
			mustStatus(t, resp, http.StatusOK)
			var it app.SetlistItem
			unmarshalField(t, body, "item", &it)
			if it.KeyOverride != "Bb" || it.TempoOverride != 140 || it.Notes != "half-time feel" {
				t.Fatalf("override wrong: %+v", it)
			}

			// remove one item → 204, list shrinks
			resp, _ = bob.do(http.MethodDelete, slBase+"/items/"+ids[1], nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, body = bob.do(http.MethodGet, slBase, nil)
			mustStatus(t, resp, http.StatusOK)
			unmarshalField(t, body, "items", &items)
			if len(items) != 2 {
				t.Fatalf("expected 2 items after remove, got %d", len(items))
			}

			// patch setlist metadata → 200
			resp, body = bob.do(http.MethodPatch, slBase, map[string]string{"name": "Saturday Gig"})
			mustStatus(t, resp, http.StatusOK)
			unmarshalField(t, body, "setlist", &sl)
			if sl.Name != "Saturday Gig" {
				t.Fatalf("patched setlist name = %q", sl.Name)
			}

			// member delete → 403 (delete is admin-only)
			resp, _ = bob.do(http.MethodDelete, slBase, nil)
			mustStatus(t, resp, http.StatusForbidden)

			// admin delete → 204, then 404
			resp, _ = admin.do(http.MethodDelete, slBase, nil)
			mustStatus(t, resp, http.StatusNoContent)
			resp, _ = admin.do(http.MethodGet, slBase, nil)
			mustStatus(t, resp, http.StatusNotFound)
		})
	}
}

// adding a song from ANOTHER band to a setlist must fail (400); unknown song 400.
func TestSetlistItemForeignSong(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)

			// band A (alice) with a setlist
			alice := newClient(t, repo)
			bandA := alice.makeBand("alice", "Band A")
			_, body := alice.do(http.MethodPost, "/api/bands/"+bandA.ID+"/setlists", map[string]string{"name": "A list"})
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)

			// band B (bob) with its own song
			bob := newClient(t, repo)
			bandB := bob.makeBand("bob", "Band B")
			foreign := bob.makeSong(bandB.ID, "Foreign Song")

			slItems := "/api/bands/" + bandA.ID + "/setlists/" + sl.ID + "/items"

			// alice adds band B's song → 400 (does not belong to band A)
			resp, _ := alice.do(http.MethodPost, slItems, map[string]string{"songId": foreign.ID})
			mustStatus(t, resp, http.StatusBadRequest)

			// alice adds a non-existent song → 400
			resp, _ = alice.do(http.MethodPost, slItems, map[string]string{"songId": "nope"})
			mustStatus(t, resp, http.StatusBadRequest)

			// a non-member of band A cannot touch its setlist → 403
			resp, _ = bob.do(http.MethodGet, "/api/bands/"+bandA.ID+"/setlists", nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

// ---- non-member 403 sweep across the new endpoints ----

func TestNonMemberForbiddenSweep(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			owner := newClient(t, repo)
			band := owner.makeBand("alice", "Band")
			song := owner.makeSong(band.ID, "Song")
			_, body := owner.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Show"})
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)

			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			b := "/api/bands/" + band.ID

			type call struct {
				method, path string
				body         any
			}
			calls := []call{
				{http.MethodPatch, b, map[string]string{"name": "x"}},
				{http.MethodDelete, b, nil},
				{http.MethodPatch, b + "/songs/" + song.ID, map[string]string{"key": "C"}},
				{http.MethodDelete, b + "/songs/" + song.ID, nil},
				{http.MethodGet, b + "/setlists", nil},
				{http.MethodPost, b + "/setlists", map[string]string{"name": "x"}},
				{http.MethodGet, b + "/setlists/" + sl.ID, nil},
				{http.MethodPatch, b + "/setlists/" + sl.ID, map[string]string{"name": "x"}},
				{http.MethodDelete, b + "/setlists/" + sl.ID, nil},
				{http.MethodPost, b + "/setlists/" + sl.ID + "/items", map[string]string{"songId": song.ID}},
				{http.MethodGet, b + "/invites", nil},
			}
			for _, c := range calls {
				resp, _ := outsider.do(c.method, c.path, c.body)
				if resp.StatusCode != http.StatusForbidden {
					t.Fatalf("%s %s: got %d, want 403", c.method, c.path, resp.StatusCode)
				}
			}
		})
	}
}

// ensure error bodies keep the {"error": string} shape.
func TestErrorBodyShape(t *testing.T) {
	repo := backends()[0].make(t)
	c := newClient(t, repo)
	c.registerLogin("alice", "pw")
	resp, body := c.do(http.MethodGet, "/api/bands/does-not-exist/setlists", nil)
	mustStatus(t, resp, http.StatusNotFound)
	var raw json.RawMessage
	unmarshalField(t, body, "error", &raw)
}

// ---- setlists: duplicate (T20) ----

func TestSetlistDuplicate(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			s1 := admin.makeSong(band.ID, "Song One")
			s2 := admin.makeSong(band.ID, "Song Two")

			// A setlist with two items; override the first.
			_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Show"})
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)
			base := "/api/bands/" + band.ID + "/setlists/" + sl.ID
			for _, sng := range []app.Song{s1, s2} {
				resp, _ := admin.do(http.MethodPost, base+"/items", map[string]string{"songId": sng.ID})
				mustStatus(t, resp, http.StatusCreated)
			}
			// Overrides on the first item.
			var items []app.SetlistItem
			_, ib := admin.do(http.MethodGet, base, nil)
			unmarshalField(t, ib, "items", &items)
			resp, _ := admin.do(http.MethodPatch, base+"/items/"+items[0].ID, map[string]any{"keyOverride": "Bb", "tempoOverride": 140, "notes": "half-time"})
			mustStatus(t, resp, http.StatusOK)
			// Bench the second item (T23) — the copy must carry the flag too.
			resp, _ = admin.do(http.MethodPatch, base+"/items/"+items[1].ID, map[string]any{"onCall": true})
			mustStatus(t, resp, http.StatusOK)

			// Duplicate → 201, name "(copy)", distinct id.
			resp, cb := admin.do(http.MethodPost, base+"/duplicate", nil)
			mustStatus(t, resp, http.StatusCreated)
			var copySL app.Setlist
			unmarshalField(t, cb, "setlist", &copySL)
			if copySL.ID == sl.ID {
				t.Fatal("duplicate reused the source setlist id")
			}
			if copySL.Name != "Show (copy)" {
				t.Fatalf("copy name = %q, want \"Show (copy)\"", copySL.Name)
			}

			// The copy's items match the source (song, position, overrides).
			var got []app.SetlistItem
			_, gb := admin.do(http.MethodGet, "/api/bands/"+band.ID+"/setlists/"+copySL.ID, nil)
			unmarshalField(t, gb, "items", &got)
			if len(got) != 2 || got[0].SongID != s1.ID || got[1].SongID != s2.ID {
				t.Fatalf("copy items wrong: %+v", got)
			}
			if got[0].Position != 0 || got[1].Position != 1 {
				t.Fatalf("copy order wrong: %+v", got)
			}
			if got[0].KeyOverride != "Bb" || got[0].TempoOverride != 140 || got[0].Notes != "half-time" {
				t.Fatalf("copy override not carried: %+v", got[0])
			}
			if got[0].OnCall {
				t.Fatalf("copy main item should not be on-call: %+v", got[0])
			}
			if !got[1].OnCall {
				t.Fatalf("copy did not carry the bench/on-call flag: %+v", got[1])
			}
			// Copy items are independent (fresh ids).
			if got[0].ID == items[0].ID {
				t.Fatal("copy item reused a source item id")
			}

			// Source is untouched: still 2 items, original name.
			var srcItems []app.SetlistItem
			_, sb := admin.do(http.MethodGet, base, nil)
			unmarshalField(t, sb, "items", &srcItems)
			if len(srcItems) != 2 {
				t.Fatalf("source item count changed: %d", len(srcItems))
			}

			// An outsider (not a band member) cannot duplicate.
			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")
			resp, _ = outsider.do(http.MethodPost, base+"/duplicate", nil)
			if resp.StatusCode < 400 {
				t.Fatalf("outsider duplicate should be denied, got %d", resp.StatusCode)
			}
		})
	}
}

// TestSetlistBench (T23): benching an item (onCall) round-trips on both backends,
// and a benched item sorts AFTER the whole main order regardless of its position;
// moving it back restores the running order. Main numbering is by main-order index,
// so the bench item never perturbs it.
func TestSetlistBench(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			admin := newClient(t, repo)
			band := admin.makeBand("alice", "Band")
			a := admin.makeSong(band.ID, "Aaa")
			b := admin.makeSong(band.ID, "Bbb")
			c := admin.makeSong(band.ID, "Ccc")

			_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Show"})
			var sl app.Setlist
			unmarshalField(t, body, "setlist", &sl)
			base := "/api/bands/" + band.ID + "/setlists/" + sl.ID
			for _, sng := range []app.Song{a, b, c} {
				resp, _ := admin.do(http.MethodPost, base+"/items", map[string]string{"songId": sng.ID})
				mustStatus(t, resp, http.StatusCreated)
			}

			get := func() []app.SetlistItem {
				var items []app.SetlistItem
				_, ib := admin.do(http.MethodGet, base, nil)
				unmarshalField(t, ib, "items", &items)
				return items
			}
			items := get() // a(0), b(1), c(2)

			// Bench the MIDDLE item (b, position 1). It must sort last despite its
			// low position, with onCall set; the other two keep their order.
			resp, _ := admin.do(http.MethodPatch, base+"/items/"+items[1].ID, map[string]any{"onCall": true})
			mustStatus(t, resp, http.StatusOK)
			after := get()
			if len(after) != 3 {
				t.Fatalf("want 3 items, got %d", len(after))
			}
			if after[0].SongID != a.ID || after[1].SongID != c.ID {
				t.Fatalf("main order should be [a c], got [%s %s]", after[0].SongID, after[1].SongID)
			}
			if after[0].OnCall || after[1].OnCall {
				t.Fatalf("main items must not be on-call: %+v", after[:2])
			}
			if after[2].SongID != b.ID || !after[2].OnCall {
				t.Fatalf("benched item should sort last with onCall=true, got %+v", after[2])
			}

			// Move it back into the running order → original position order restored.
			resp, _ = admin.do(http.MethodPatch, base+"/items/"+items[1].ID, map[string]any{"onCall": false})
			mustStatus(t, resp, http.StatusOK)
			restored := get()
			if restored[0].SongID != a.ID || restored[1].SongID != b.ID || restored[2].SongID != c.ID {
				t.Fatalf("moving back should restore [a b c], got [%s %s %s]",
					restored[0].SongID, restored[1].SongID, restored[2].SongID)
			}
			for _, it := range restored {
				if it.OnCall {
					t.Fatalf("no item should be on-call after moving back: %+v", it)
				}
			}
		})
	}
}

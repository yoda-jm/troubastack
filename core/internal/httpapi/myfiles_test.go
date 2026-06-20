package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// myFilesSetup registers+logs in alice, creates a band and a song, uploads n PDF
// files to that song's pool, and returns the band, song, and uploaded files (in
// upload order). The caller is left authenticated as alice.
func myFilesSetup(t *testing.T, c *client, n int) (app.Band, app.Song, []app.SongFile) {
	t.Helper()
	c.registerLogin("alice", "pw")
	_, body := c.do(http.MethodPost, "/api/bands", map[string]string{"name": "Band"})
	var band app.Band
	unmarshalField(t, body, "band", &band)
	_, body = c.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "Wonderwall"})
	var song app.Song
	unmarshalField(t, body, "song", &song)

	base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"
	files := make([]app.SongFile, 0, n)
	for i := 0; i < n; i++ {
		resp, fbody := c.upload(base, "f.pdf", "application/pdf", smallPDF)
		mustStatus(t, resp, http.StatusCreated)
		var f app.SongFile
		unmarshalField(t, fbody, "file", &f)
		// Give each file a distinct displayOrder matching upload order so the
		// default ordering is deterministic.
		resp, _ = c.do(http.MethodPatch, base+"/"+f.ID, map[string]int{"displayOrder": i})
		mustStatus(t, resp, http.StatusOK)
		f.DisplayOrder = i
		files = append(files, f)
	}
	return band, song, files
}

// decodeMyFiles unmarshals the {files, customized} response shape.
func decodeMyFiles(t *testing.T, body map[string]json.RawMessage) ([]app.SongFile, bool) {
	t.Helper()
	var files []app.SongFile
	unmarshalField(t, body, "files", &files)
	var customized bool
	unmarshalField(t, body, "customized", &customized)
	return files, customized
}

func fileIDs(files []app.SongFile) []string {
	ids := make([]string, len(files))
	for i, f := range files {
		ids[i] = f.ID
	}
	return ids
}

func equalIDs(a, b []string) bool {
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

// TestMyFilesDefaultUnset: with no saved selection, GET returns all pool files in
// displayOrder, customized=false.
func TestMyFilesDefaultUnset(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, files := myFilesSetup(t, c, 3)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			resp, body := c.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if customized {
				t.Fatalf("customized = true, want false (no selection set)")
			}
			if !equalIDs(fileIDs(got), fileIDs(files)) {
				t.Fatalf("default files = %v, want all pool in displayOrder %v", fileIDs(got), fileIDs(files))
			}
		})
	}
}

// TestMyFilesPutSubsetReorder: PUT a 2-of-3 subset in non-default order → GET
// returns exactly those, in that order, customized=true.
func TestMyFilesPutSubsetReorder(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, files := myFilesSetup(t, c, 3)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			// Pick files[2] then files[0] — a 2-of-3 subset in non-default order.
			want := []string{files[2].ID, files[0].ID}
			resp, body := c.do(http.MethodPut, myFiles, map[string][]string{"fileIds": want})
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if !customized {
				t.Fatalf("PUT response customized = false, want true")
			}
			if !equalIDs(fileIDs(got), want) {
				t.Fatalf("PUT response files = %v, want %v", fileIDs(got), want)
			}

			// GET reflects it.
			resp, body = c.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			got, customized = decodeMyFiles(t, body)
			if !customized {
				t.Fatalf("GET customized = false, want true")
			}
			if !equalIDs(fileIDs(got), want) {
				t.Fatalf("GET files = %v, want %v", fileIDs(got), want)
			}
		})
	}
}

// TestMyFilesDeletedFileDropsOut: deleting a pool file that is part of a saved
// selection drops it from the selection on next GET, with no error.
func TestMyFilesDeletedFileDropsOut(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, files := myFilesSetup(t, c, 3)
			base := "/api/bands/" + band.ID + "/songs/" + song.ID + "/files"
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			// Select all three in order [0,1,2].
			want := []string{files[0].ID, files[1].ID, files[2].ID}
			resp, _ := c.do(http.MethodPut, myFiles, map[string][]string{"fileIds": want})
			mustStatus(t, resp, http.StatusOK)

			// Delete the middle file from the pool.
			resp, _ = c.do(http.MethodDelete, base+"/"+files[1].ID, nil)
			mustStatus(t, resp, http.StatusNoContent)

			// GET drops it gracefully, keeps the rest in order, still customized.
			resp, body := c.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if !customized {
				t.Fatalf("customized = false, want true")
			}
			wantAfter := []string{files[0].ID, files[2].ID}
			if !equalIDs(fileIDs(got), wantAfter) {
				t.Fatalf("after delete files = %v, want %v", fileIDs(got), wantAfter)
			}
		})
	}
}

// TestMyFilesForeignFileRejected: PUT with a fileId from another song → 400.
func TestMyFilesForeignFileRejected(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, files := myFilesSetup(t, c, 2)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			// Create a second song in the same band and upload a file to it.
			_, body := c.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": "Other"})
			var other app.Song
			unmarshalField(t, body, "song", &other)
			resp, fbody := c.upload("/api/bands/"+band.ID+"/songs/"+other.ID+"/files", "o.pdf", "application/pdf", smallPDF)
			mustStatus(t, resp, http.StatusCreated)
			var foreign app.SongFile
			unmarshalField(t, fbody, "file", &foreign)

			// PUT mixing a valid file with the foreign one → 400.
			resp, _ = c.do(http.MethodPut, myFiles, map[string][]string{
				"fileIds": {files[0].ID, foreign.ID},
			})
			mustStatus(t, resp, http.StatusBadRequest)
		})
	}
}

// TestMyFilesPerUserIsolation: userA's PUT must not change userB's GET (default).
func TestMyFilesPerUserIsolation(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			userA := newClient(t, repo)
			band, song, files := myFilesSetup(t, userA, 3)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			// userB joins the band (invited by userA).
			userB := newClient(t, repo)
			userB.registerLogin("bob", "pw")
			_, ibody := userA.do(http.MethodPost, "/api/bands/"+band.ID+"/invites",
				map[string]string{"identifier": "bob", "kind": "username"})
			var inv app.Invite
			unmarshalField(t, ibody, "invite", &inv)
			resp, _ := userB.do(http.MethodPost, "/api/invites/"+inv.ID+"/accept", nil)
			mustStatus(t, resp, http.StatusOK)

			// userA customizes to a single file.
			resp, _ = userA.do(http.MethodPut, myFiles, map[string][]string{"fileIds": {files[1].ID}})
			mustStatus(t, resp, http.StatusOK)

			// userB still sees the full default, customized=false.
			resp, body := userB.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if customized {
				t.Fatalf("userB customized = true, want false (isolation breach)")
			}
			if !equalIDs(fileIDs(got), fileIDs(files)) {
				t.Fatalf("userB files = %v, want all pool %v", fileIDs(got), fileIDs(files))
			}

			// userA's own selection is unchanged.
			resp, body = userA.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			gotA, customizedA := decodeMyFiles(t, body)
			if !customizedA || !equalIDs(fileIDs(gotA), []string{files[1].ID}) {
				t.Fatalf("userA selection = %v customized=%v, want [%s] customized=true", fileIDs(gotA), customizedA, files[1].ID)
			}
		})
	}
}

// TestMyFilesDeleteRevertsToDefault: DELETE clears the customization → default-all,
// customized=false.
func TestMyFilesDeleteRevertsToDefault(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, files := myFilesSetup(t, c, 3)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			resp, _ := c.do(http.MethodPut, myFiles, map[string][]string{"fileIds": {files[0].ID}})
			mustStatus(t, resp, http.StatusOK)

			resp, _ = c.do(http.MethodDelete, myFiles, nil)
			mustStatus(t, resp, http.StatusNoContent)

			resp, body := c.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if customized {
				t.Fatalf("customized = true after DELETE, want false")
			}
			if !equalIDs(fileIDs(got), fileIDs(files)) {
				t.Fatalf("after DELETE files = %v, want all pool %v", fileIDs(got), fileIDs(files))
			}
		})
	}
}

// TestMyFilesEmptySelectionAllowed: PUT an empty list is valid (show nothing),
// customized=true.
func TestMyFilesEmptySelectionAllowed(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			c := newClient(t, be.make(t))
			band, song, _ := myFilesSetup(t, c, 2)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			resp, body := c.do(http.MethodPut, myFiles, map[string][]string{"fileIds": {}})
			mustStatus(t, resp, http.StatusOK)
			got, customized := decodeMyFiles(t, body)
			if !customized {
				t.Fatalf("customized = false, want true for an explicit empty selection")
			}
			if len(got) != 0 {
				t.Fatalf("files = %v, want empty", fileIDs(got))
			}
		})
	}
}

// TestMyFilesNonMember: a non-member is forbidden on all three verbs → 403.
func TestMyFilesNonMember(t *testing.T) {
	for _, be := range backends() {
		t.Run(be.name, func(t *testing.T) {
			repo := be.make(t)
			owner := newClient(t, repo)
			band, song, files := myFilesSetup(t, owner, 2)
			myFiles := "/api/bands/" + band.ID + "/songs/" + song.ID + "/my-files"

			outsider := newClient(t, repo)
			outsider.registerLogin("mallory", "pw")

			resp, _ := outsider.do(http.MethodGet, myFiles, nil)
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = outsider.do(http.MethodPut, myFiles, map[string][]string{"fileIds": {files[0].ID}})
			mustStatus(t, resp, http.StatusForbidden)
			resp, _ = outsider.do(http.MethodDelete, myFiles, nil)
			mustStatus(t, resp, http.StatusForbidden)
		})
	}
}

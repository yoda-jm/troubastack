package httpapi_test

import (
	"bytes"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// TestFilePDF_endpoint covers the FILEPDF edge: a member prints one song file as application/pdf with a
// named attachment; a non-member is refused; an unknown file and another song's file are 404; the layers
// parameter (present, empty, absent) all print.
func TestFilePDF_endpoint(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}
	stranger := &client{t: t, srv: srv}

	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")
	stranger.registerLogin("eve", "pw")

	mkSong := func(title string) (string, string) {
		_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": title})
		var song app.Song
		unmarshalField(t, body, "song", &song)
		resp, fbody := admin.upload("/api/bands/"+band.ID+"/songs/"+song.ID+"/files", "Lead sheet.pdf", "application/pdf", smallPDF)
		if resp.StatusCode >= 300 {
			t.Fatalf("upload: %d", resp.StatusCode)
		}
		var f app.SongFile
		unmarshalField(t, fbody, "file", &f)
		return song.ID, f.ID
	}
	songID, fileID := mkSong("Sound Check")
	_, otherFileID := mkSong("Other")

	url := "/api/bands/" + band.ID + "/songs/" + songID + "/files/" + fileID + "/pdf"

	resp, pdf := rawGet(member, url)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("body is not a PDF (%d bytes)", len(pdf))
	}
	if cl := resp.Header.Get("Content-Length"); cl != strconv.Itoa(len(pdf)) {
		t.Fatalf("Content-Length = %q, want %d", cl, len(pdf))
	}
	cd := resp.Header.Get("Content-Disposition")
	if !strings.HasPrefix(cd, "attachment;") || !strings.Contains(cd, `filename="Sound Check - Lead sheet.pdf"`) {
		t.Fatalf("Content-Disposition = %q, want an attachment named \"Sound Check - Lead sheet.pdf\"", cd)
	}

	// The layers parameter: present-empty (no optional annotations) and a list both print.
	for _, q := range []string{"?layers=", "?layers=L1,unknown"} {
		resp, body := rawGet(member, url+q)
		mustStatus(t, resp, http.StatusOK)
		if !bytes.HasPrefix(body, []byte("%PDF")) {
			t.Fatalf("%s: body is not a PDF", q)
		}
	}

	// A non-member is refused (GetBand's convention: 403).
	resp, _ = rawGet(stranger, url)
	mustStatus(t, resp, http.StatusForbidden)

	// Unknown file, and a file of ANOTHER song addressed under this one → 404.
	resp, _ = rawGet(member, "/api/bands/"+band.ID+"/songs/"+songID+"/files/nope/pdf")
	mustStatus(t, resp, http.StatusNotFound)
	resp, _ = rawGet(member, "/api/bands/"+band.ID+"/songs/"+songID+"/files/"+otherFileID+"/pdf")
	mustStatus(t, resp, http.StatusNotFound)
}

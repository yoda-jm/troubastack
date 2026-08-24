package httpapi_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// rawGet issues a GET carrying the client's session jar and returns the raw body
// bytes (the JSON-decoding `do` helper can't handle a binary PDF response).
func rawGet(c *client, path string) (*http.Response, []byte) {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.srv.URL+path, nil)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp, b
}

// TestConcertPDF_endpoint covers the T57 /pdf edge: same gating as the bundle
// download, an application/pdf body, and the admin-only ?member= override. The
// empty-setlist bake yields a valid zero-pages PDF (no poppler/web-bake needed).
func TestConcertPDF_endpoint(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}

	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	// An empty setlist, baked (zero pages → the "no pages" PDF path).
	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	awaitBake(t, admin, band.ID, sl.ID) // T103: kick + poll to terminal before reading the concert

	pdfURL := "/api/bands/" + band.ID + "/concerts/" + sl.ID + "/pdf"

	// A member can print the band concert (same access as the bundle download).
	resp, pdf := rawGet(member, pdfURL)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF-")) {
		t.Fatalf("body is not a PDF (%d bytes)", len(pdf))
	}

	// ?member= is admin-only: a plain member printing on someone else's behalf is 403.
	resp, _ = rawGet(member, pdfURL+"?member=someone-else")
	mustStatus(t, resp, http.StatusForbidden)
	// The admin may (prints that member's view — here just proves the gate opens).
	resp, _ = rawGet(admin, pdfURL+"?member=someone-else")
	mustStatus(t, resp, http.StatusOK)

	// A setlist that exists but was never baked → 404 (ConcertPDF ErrNotExist).
	_, ubody := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Unbaked"})
	var usl app.Setlist
	unmarshalField(t, ubody, "setlist", &usl)
	resp, _ = rawGet(admin, "/api/bands/"+band.ID+"/concerts/"+usl.ID+"/pdf")
	mustStatus(t, resp, http.StatusNotFound)
}

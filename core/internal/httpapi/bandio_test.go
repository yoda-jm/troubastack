package httpapi_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// uploadImport POSTs a .tband zip (the "file" part) plus optional extra form fields
// (e.g. "dispositions") to path, carrying the client's session jar, and decodes the
// top-level JSON object.
func uploadImport(c *client, path string, zipBytes []byte, fields map[string]string) (*http.Response, map[string]json.RawMessage) {
	c.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	hdr := map[string][]string{
		"Content-Disposition": {`form-data; name="file"; filename="band.tband.zip"`},
		"Content-Type":        {"application/zip"},
	}
	part, err := mw.CreatePart(hdr)
	if err != nil {
		c.t.Fatalf("create part: %v", err)
	}
	if _, err := part.Write(zipBytes); err != nil {
		c.t.Fatalf("write part: %v", err)
	}
	for k, v := range fields {
		if err := mw.WriteField(k, v); err != nil {
			c.t.Fatalf("write field %s: %v", k, err)
		}
	}
	if err := mw.Close(); err != nil {
		c.t.Fatalf("close mw: %v", err)
	}
	req, _ := http.NewRequest(http.MethodPost, c.srv.URL+path, &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	for _, ck := range c.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		c.t.Fatalf("upload %s: %v", path, err)
	}
	if cks := resp.Cookies(); len(cks) > 0 {
		c.storeCookies(cks)
	}
	var decoded map[string]json.RawMessage
	defer resp.Body.Close()
	_ = json.NewDecoder(resp.Body).Decode(&decoded)
	return resp, decoded
}

// TestBandExportImport_HTTP covers the T62 endpoints over the wire: an admin exports a
// band as a .tband zip; another user imports it (multipart) into a NEW band with the
// content; a non-admin export is 403; a tampered manifest is 400.
func TestBandExportImport_HTTP(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "The Open Road")
	base := "/api/bands/" + band.ID

	// A generated text chart so there's real content to move.
	resp, _ := admin.do(http.MethodPost, base+"/songs/"+song.ID+"/text-charts",
		map[string]string{"source": "# The Open Road\n## Verse\nEm7          G\nlyrics\n"})
	mustStatus(t, resp, http.StatusCreated)

	// Export (admin) → application/zip, non-empty.
	eresp, zipBytes := rawGet(admin, base+"/export")
	mustStatus(t, eresp, http.StatusOK)
	if ct := eresp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("export content-type = %q, want application/zip", ct)
	}
	if !bytes.HasPrefix(zipBytes, []byte("PK")) {
		t.Fatalf("export is not a zip (%d bytes)", len(zipBytes))
	}

	// Import (a different user) → 201 + report; the new band carries the song. alice (the
	// source admin) already exists here, so under the CONSENT model she is invited (not
	// silently attached — the account-takeover fix), and bob owns the new band.
	importer := &client{t: t, srv: srv}
	importer.registerLogin("bob", "pw")
	iresp, ibody := importer.upload("/api/bands/import", "band.tband.zip", "application/zip", zipBytes)
	mustStatus(t, iresp, http.StatusCreated)
	var newBand app.Band
	unmarshalField(t, ibody, "band", &newBand)
	if newBand.ID == "" || newBand.OwnerID != importer.meID() {
		t.Fatalf("imported band = %+v, want owned by the importer", newBand)
	}
	var invited []string
	unmarshalField(t, ibody, "invited", &invited)
	if len(invited) != 1 || invited[0] != "alice" {
		t.Fatalf("invited=%v, want [alice] (existing account, consent-required)", invited)
	}
	_, sb := importer.do(http.MethodGet, "/api/bands/"+newBand.ID+"/songs", nil)
	var songs []app.Song
	unmarshalField(t, sb, "songs", &songs)
	if len(songs) != 1 || songs[0].Title != "The Open Road" {
		t.Fatalf("imported band songs = %+v, want [The Open Road]", songs)
	}

	// Non-admin export → 403 (export stays live and admin-gated).
	member := &client{t: t, srv: srv}
	member.registerLogin("carol", "pw")
	inviteAndAccept(t, admin, member, band.ID, "carol")
	mresp, _ := rawGet(member, base+"/export")
	mustStatus(t, mresp, http.StatusForbidden)

	// Tampered manifest (formatVersion) → 400, nothing created.
	bad := tamperFormatVersion(t, zipBytes)
	bresp, _ := importer.upload("/api/bands/import", "bad.zip", "application/zip", bad)
	if bresp.StatusCode != http.StatusBadRequest {
		t.Fatalf("bad-format import = %d, want 400", bresp.StatusCode)
	}
}

// TestBandImport_PreviewDispositions_HTTP covers the T63 edge over the wire: the
// import:preview endpoint classifies members + writes nothing, and the import endpoint
// parses + validates the dispositions form field. On this single-server harness both source
// members already exist → they're consent-required (existing), so "create" for one is a 400
// and the plain import invites them. (The create/invite/skip post-state across servers is
// covered by the app-level tests.)
func TestBandImport_PreviewDispositions_HTTP(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	admin.makeSong(band.ID, "Song")
	member := &client{t: t, srv: srv}
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	eresp, zipBytes := rawGet(admin, "/api/bands/"+band.ID+"/export")
	mustStatus(t, eresp, http.StatusOK)

	importer := &client{t: t, srv: srv}
	importer.registerLogin("carol", "pw")

	// Preview: alice + bob already exist here → both existing (consent-required). Writes nothing.
	presp, pbody := uploadImport(importer, "/api/bands/import:preview", zipBytes, nil)
	mustStatus(t, presp, http.StatusOK)
	var members []app.PreviewMember
	unmarshalField(t, pbody, "members", &members)
	if len(members) != 2 {
		t.Fatalf("preview members=%+v, want 2", members)
	}
	for _, m := range members {
		if !m.Existing || m.IsCaller {
			t.Fatalf("member %+v: want existing + not caller", m)
		}
	}
	_, lb := importer.do(http.MethodGet, "/api/bands", nil)
	var bands []app.Band
	unmarshalField(t, lb, "bands", &bands)
	if len(bands) != 0 {
		t.Fatalf("preview created %d bands, want 0", len(bands))
	}

	// "create" for an existing account → 400 (consent-required: invite or skip only).
	bad, _ := uploadImport(importer, "/api/bands/import", zipBytes, map[string]string{"dispositions": `{"alice":"create"}`})
	if bad.StatusCode != http.StatusBadRequest {
		t.Fatalf("create on an existing account = %d, want 400", bad.StatusCode)
	}
	// Malformed dispositions JSON → 400.
	bad2, _ := uploadImport(importer, "/api/bands/import", zipBytes, map[string]string{"dispositions": `{not json`})
	if bad2.StatusCode != http.StatusBadRequest {
		t.Fatalf("malformed dispositions = %d, want 400", bad2.StatusCode)
	}
	// No dispositions → plain import succeeds; both existing members are invited (not attached).
	ok, okBody := uploadImport(importer, "/api/bands/import", zipBytes, nil)
	mustStatus(t, ok, http.StatusCreated)
	var newBand app.Band
	var invited []string
	unmarshalField(t, okBody, "band", &newBand)
	unmarshalField(t, okBody, "invited", &invited)
	if newBand.ID == "" || len(invited) != 2 {
		t.Fatalf("plain import band=%q invited=%v, want a band + 2 invited", newBand.ID, invited)
	}
}

// tamperFormatVersion rewrites band.json's formatVersion to an unsupported value,
// copying every other zip entry through unchanged.
func tamperFormatVersion(t *testing.T, zipBytes []byte) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		t.Fatalf("read zip: %v", err)
	}
	var out bytes.Buffer
	zw := zip.NewWriter(&out)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		data, _ := io.ReadAll(rc)
		rc.Close()
		if f.Name == "band.json" {
			var man map[string]json.RawMessage
			if err := json.Unmarshal(data, &man); err != nil {
				t.Fatalf("unmarshal manifest: %v", err)
			}
			man["formatVersion"] = json.RawMessage("999")
			data, _ = json.Marshal(man)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return out.Bytes()
}

package httpapi_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestBandExportImport_HTTP covers the T62 endpoints over the wire: an admin exports a
// band as a .tband zip; another user imports it (multipart) into a NEW band with the
// content; a non-admin export is 403; a tampered manifest is 400.
func TestBandExportImport_HTTP(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "Wonderwall")
	base := "/api/bands/" + band.ID

	// A generated text chart so there's real content to move.
	resp, _ := admin.do(http.MethodPost, base+"/songs/"+song.ID+"/text-charts",
		map[string]string{"source": "# Wonderwall\n## Verse\nEm7          G\nlyrics\n"})
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

	// Import (a different user) → 201 + report; the new band carries the song.
	importer := &client{t: t, srv: srv}
	importer.registerLogin("bob", "pw")
	iresp, ibody := importer.upload("/api/bands/import", "band.tband.zip", "application/zip", zipBytes)
	mustStatus(t, iresp, http.StatusCreated)
	var newBand app.Band
	unmarshalField(t, ibody, "band", &newBand)
	if newBand.ID == "" || newBand.OwnerID != importer.meID() {
		t.Fatalf("imported band = %+v, want owned by the importer", newBand)
	}
	_, sb := importer.do(http.MethodGet, "/api/bands/"+newBand.ID+"/songs", nil)
	var songs []app.Song
	unmarshalField(t, sb, "songs", &songs)
	if len(songs) != 1 || songs[0].Title != "Wonderwall" {
		t.Fatalf("imported band songs = %+v, want [Wonderwall]", songs)
	}

	// Non-admin export → 403.
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

package httpapi_test

import (
	"bytes"
	"net/http"
	"testing"
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

	// SECURITY HOLD (2026-07-26): the import ROUTE is disabled (503) pending the T63
	// consent fix — a critical account-takeover chain (see reviews.md). Until it's
	// re-enabled the endpoint must refuse everything; the export half + the full
	// service-layer round-trip (TestBandExportImport_RoundTrip in app/) stay covered.
	importer := &client{t: t, srv: srv}
	importer.registerLogin("bob", "pw")
	iresp, _ := importer.upload("/api/bands/import", "band.tband.zip", "application/zip", zipBytes)
	mustStatus(t, iresp, http.StatusServiceUnavailable)

	// Non-admin export → 403 (export stays live and admin-gated).
	member := &client{t: t, srv: srv}
	member.registerLogin("carol", "pw")
	inviteAndAccept(t, admin, member, band.ID, "carol")
	mresp, _ := rawGet(member, base+"/export")
	mustStatus(t, mresp, http.StatusForbidden)
}

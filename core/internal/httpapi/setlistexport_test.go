package httpapi_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// T158 — the export endpoint serves the running order as a printable A4 PDF, gated by band membership. The
// NUMBERING rule itself is proven by the shared-vector test in internal/runningorder; this asserts the wire:
// a member gets a real PDF as an attachment, a non-member is refused.
func TestSetlistExport_PDF_T158(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Café Band") // accented → exercises the cp1252 path end-to-end

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Fête Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	// Optional header lines present (they must render; absent ones are covered by the renderer's unit test).
	if resp, _ := admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/setlists/"+sl.ID,
		map[string]any{"venue": "Le Sous-sol", "eventDate": "2026-09-21"}); resp.StatusCode >= 300 {
		t.Fatalf("patch setlist: %d", resp.StatusCode)
	}

	// Two songs: one in the running order, one on the bench (on-call).
	addSong := func(title string) string {
		_, b := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/songs", map[string]string{"title": title})
		var s app.Song
		unmarshalField(t, b, "song", &s)
		_, ib := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists/"+sl.ID+"/items", map[string]string{"songId": s.ID})
		var it app.SetlistItem
		unmarshalField(t, ib, "item", &it)
		return it.ID
	}
	addSong("Opener")
	benchItem := addSong("Encore Maybe")
	if resp, _ := admin.do(http.MethodPatch, "/api/bands/"+band.ID+"/setlists/"+sl.ID+"/items/"+benchItem,
		map[string]any{"onCall": true}); resp.StatusCode >= 300 {
		t.Fatalf("set on-call: %d", resp.StatusCode)
	}

	resp, pdf := rawGet(admin, "/api/bands/"+band.ID+"/setlists/"+sl.ID+"/export")
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("Content-Type = %q, want application/pdf", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "attachment") || !strings.Contains(cd, ".pdf") {
		t.Fatalf("Content-Disposition = %q, want an attachment .pdf", cd)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) || len(pdf) < 500 {
		t.Fatalf("body is not a real PDF (%d bytes)", len(pdf))
	}

	// A non-member is refused (membership-gated read).
	stranger := &client{t: t, srv: srv}
	stranger.registerLogin("bob", "pw")
	r2, _ := rawGet(stranger, "/api/bands/"+band.ID+"/setlists/"+sl.ID+"/export")
	if r2.StatusCode != http.StatusForbidden && r2.StatusCode != http.StatusNotFound {
		t.Fatalf("non-member export = %d, want 403 or 404", r2.StatusCode)
	}
}

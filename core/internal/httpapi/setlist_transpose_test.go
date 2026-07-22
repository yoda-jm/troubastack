package httpapi_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"troubastack/core/internal/app"
)

// TestSetlistItemTranspose (T60 surface 2) covers the setlist-item transpose flags and
// the chart-preview over the HTTP edge: transposeChords round-trips on the item PATCH;
// chart-preview renders a PDF for a chart song and 404s for a PDF-only song (the greyed
// path). The bake-time decision + eligibility reasons are covered in the bake package.
func TestSetlistItemTranspose(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "Stand By Me")
	base := "/api/bands/" + band.ID

	// Song key = C + a generated chart (so the item is transpose-eligible).
	resp, _ := admin.do(http.MethodPatch, base+"/songs/"+song.ID, map[string]any{"key": "C"})
	mustStatus(t, resp, http.StatusOK)
	resp, _ = admin.do(http.MethodPost, base+"/songs/"+song.ID+"/text-charts",
		map[string]string{"source": "# S\n## V\nC            G\nlyrics under the chords\n"})
	mustStatus(t, resp, http.StatusCreated)

	// Setlist + item.
	_, slb := admin.do(http.MethodPost, base+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, slb, "setlist", &sl)
	slBase := base + "/setlists/" + sl.ID
	_, ib := admin.do(http.MethodPost, slBase+"/items", map[string]string{"songId": song.ID})
	var item app.SetlistItem
	unmarshalField(t, ib, "item", &item)

	// PATCH transposeChords + keyOverride → round-trips.
	resp, pb := admin.do(http.MethodPatch, slBase+"/items/"+item.ID,
		map[string]any{"keyOverride": "D", "transposeChords": true})
	mustStatus(t, resp, http.StatusOK)
	var patched app.SetlistItem
	unmarshalField(t, pb, "item", &patched)
	if !patched.TransposeChords || patched.KeyOverride != "D" {
		t.Fatalf("patched item = %+v, want transposeChords=true keyOverride=D", patched)
	}

	// chart-preview renders a PDF (transposed to D; here we assert it renders).
	presp, prev := rawGet(admin, slBase+"/items/"+item.ID+"/chart-preview")
	mustStatus(t, presp, http.StatusOK)
	if ct := presp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Fatalf("preview content-type = %q, want application/pdf", ct)
	}
	if !bytes.HasPrefix(prev, []byte("%PDF-")) {
		t.Fatalf("preview body is not a PDF (%d bytes)", len(prev))
	}

	// Turning transpose off still previews (identity — "show me the chart from here").
	resp, _ = admin.do(http.MethodPatch, slBase+"/items/"+item.ID, map[string]any{"transposeChords": false})
	mustStatus(t, resp, http.StatusOK)
	presp, _ = rawGet(admin, slBase+"/items/"+item.ID+"/chart-preview")
	mustStatus(t, presp, http.StatusOK)

	// A PDF-only song has no generated chart → chart-preview 404 (the greyed path).
	song2 := admin.makeSong(band.ID, "PDF Only")
	uresp, _ := admin.upload(base+"/songs/"+song2.ID+"/files", "u.pdf", "application/pdf", smallPDF)
	mustStatus(t, uresp, http.StatusCreated)
	_, ib2 := admin.do(http.MethodPost, slBase+"/items", map[string]string{"songId": song2.ID})
	var item2 app.SetlistItem
	unmarshalField(t, ib2, "item", &item2)
	presp, _ = rawGet(admin, slBase+"/items/"+item2.ID+"/chart-preview")
	mustStatus(t, presp, http.StatusNotFound)
}

// TestBakeTransposeWarnings (T60 surface 2) covers the bake response's per-song
// `warnings`: an item that asked to transpose but wasn't eligible at bake time is
// reported (baked untransposed, not failed). Uses a metadata-only song (no chart) so
// the band bake succeeds without a rasterizer — the "no text chart" degraded path, and
// the same transposeWarnings derivation the key-degrade path uses.
func TestBakeTransposeWarnings(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "No Chart Song") // metadata only — no generated chart
	base := "/api/bands/" + band.ID

	_, slb := admin.do(http.MethodPost, base+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, slb, "setlist", &sl)
	slBase := base + "/setlists/" + sl.ID
	_, ib := admin.do(http.MethodPost, slBase+"/items", map[string]string{"songId": song.ID})
	var item app.SetlistItem
	unmarshalField(t, ib, "item", &item)

	// Ask to transpose (with an override) — but there's no chart, so it's ineligible.
	resp, _ := admin.do(http.MethodPatch, slBase+"/items/"+item.ID,
		map[string]any{"keyOverride": "D", "transposeChords": true})
	mustStatus(t, resp, http.StatusOK)

	// Band bake succeeds (metadata-only song ⇒ no rasterize) and reports the warning.
	resp, bb := admin.do(http.MethodPost, slBase+"/bake", nil)
	mustStatus(t, resp, http.StatusOK)
	warns := bakeWarnings(t, bb)
	if len(warns) != 1 || !strings.Contains(warns[0], "no text chart on this song") {
		t.Fatalf("bake warnings = %v, want one 'no text chart' warning", warns)
	}

	// Turning transpose off → no warning (the field is omitempty ⇒ absent).
	resp, _ = admin.do(http.MethodPatch, slBase+"/items/"+item.ID, map[string]any{"transposeChords": false})
	mustStatus(t, resp, http.StatusOK)
	resp, bb2 := admin.do(http.MethodPost, slBase+"/bake", nil)
	mustStatus(t, resp, http.StatusOK)
	if warns2 := bakeWarnings(t, bb2); len(warns2) != 0 {
		t.Fatalf("no-transpose bake should have no warnings, got %v", warns2)
	}
}

// bakeWarnings reads the bake response's optional (omitempty) `warnings` array,
// tolerating its absence (unlike unmarshalField, which fatals on a missing field).
func bakeWarnings(t *testing.T, body map[string]json.RawMessage) []string {
	t.Helper()
	raw, ok := body["warnings"]
	if !ok || len(raw) == 0 {
		return nil
	}
	var warns []string
	if err := json.Unmarshal(raw, &warns); err != nil {
		t.Fatalf("warnings unmarshal: %v", err)
	}
	return warns
}

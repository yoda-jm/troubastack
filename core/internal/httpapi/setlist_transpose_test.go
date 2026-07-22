package httpapi_test

import (
	"bytes"
	"net/http"
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

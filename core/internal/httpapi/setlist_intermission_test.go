package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestSetlistIntermissionEndpoints_T153 covers slice 4's HTTP surface: an intermission can be
// CREATED, LABELLED, and REMOVED over the edge (slice 1 exposed no route, so a break could only be
// made from Go). Create rides the existing items endpoint with a kind discriminator; label rides the
// existing item PATCH; remove is the existing DELETE.
func TestSetlistIntermissionEndpoints_T153(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "Opener")
	base := "/api/bands/" + band.ID

	_, slb := admin.do(http.MethodPost, base+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, slb, "setlist", &sl)
	slBase := base + "/setlists/" + sl.ID

	// Control: a plain song add (no kind ⇒ song) still works.
	resp, _ := admin.do(http.MethodPost, slBase+"/items", map[string]string{"songId": song.ID})
	mustStatus(t, resp, http.StatusCreated)

	// CREATE an intermission via the items endpoint.
	resp, ib := admin.do(http.MethodPost, slBase+"/items", map[string]any{"kind": "intermission", "label": "Entracte"})
	mustStatus(t, resp, http.StatusCreated)
	var brk app.SetlistItem
	unmarshalField(t, ib, "item", &brk)
	if !brk.IsIntermission() {
		t.Fatalf("created item kind = %q, want intermission", brk.Kind)
	}
	if brk.Label != "Entracte" {
		t.Errorf("label = %q, want %q", brk.Label, "Entracte")
	}
	if brk.SongID != "" {
		t.Errorf("intermission SongID = %q, want empty", brk.SongID)
	}

	// LABEL edit via the item PATCH.
	resp, pb := admin.do(http.MethodPatch, slBase+"/items/"+brk.ID, map[string]any{"label": "Set break"})
	mustStatus(t, resp, http.StatusOK)
	var patched app.SetlistItem
	unmarshalField(t, pb, "item", &patched)
	if patched.Label != "Set break" {
		t.Errorf("patched label = %q, want %q", patched.Label, "Set break")
	}
	if !patched.IsIntermission() {
		t.Error("a label patch must not change the kind")
	}

	// An empty-label intermission is allowed — the rendered page supplies the default.
	resp, _ = admin.do(http.MethodPost, slBase+"/items", map[string]any{"kind": "intermission"})
	mustStatus(t, resp, http.StatusCreated)

	// REMOVE works for an intermission (the existing DELETE, by item id).
	resp, _ = admin.do(http.MethodDelete, slBase+"/items/"+brk.ID, nil)
	mustStatus(t, resp, http.StatusNoContent)
}

// TestSetlistAddItem_KindDiscriminator_T153: absent ⇒ song (the additive contract), an explicit
// unknown kind is a client error (400), NOT silently a song — a typo like "intermision" with a valid
// songId must not create the wrong entry with a quiet 201.
func TestSetlistAddItem_KindDiscriminator_T153(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")
	song := admin.makeSong(band.ID, "Opener")
	base := "/api/bands/" + band.ID
	_, slb := admin.do(http.MethodPost, base+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, slb, "setlist", &sl)
	items := base + "/setlists/" + sl.ID + "/items"

	// Absent kind ⇒ song.
	resp, _ := admin.do(http.MethodPost, items, map[string]any{"songId": song.ID})
	mustStatus(t, resp, http.StatusCreated)

	// Explicit kind:"song" ⇒ song.
	resp, _ = admin.do(http.MethodPost, items, map[string]any{"kind": "song", "songId": song.ID})
	mustStatus(t, resp, http.StatusCreated)

	// A typo'd kind with a VALID songId must be rejected, not coerced to a song.
	resp, _ = admin.do(http.MethodPost, items, map[string]any{"kind": "intermision", "songId": song.ID})
	mustStatus(t, resp, http.StatusBadRequest)
}

package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// TestBakePersonalVariant_scopeAndAuthz covers B07's distribution contract with
// empty setlists (so poppler/web-bake are never invoked): a member can bake
// their OWN variant of a setlist (scope=mine), the variant is a distinct concert
// that revs independently of the band bake, and a member sees + downloads band
// concerts plus only THEIR OWN variants — never another member's.
func TestBakePersonalVariant_scopeAndAuthz(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	bob := &client{t: t, srv: srv}
	carol := &client{t: t, srv: srv}

	band := admin.makeBand("alice", "Band")
	bob.registerLogin("bob", "pw")
	carol.registerLogin("carol", "pw")
	inviteAndAccept(t, admin, bob, band.ID, "bob")
	inviteAndAccept(t, admin, carol, band.ID, "carol")
	bobID := bob.meID()
	carolID := carol.meID()

	// An (empty) setlist.
	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	setlistURL := "/api/bands/" + band.ID + "/setlists/" + sl.ID

	// Admin bakes the band-wide concert → concertId == setlist id, rev 1.
	resp, cb := admin.do(http.MethodPost, setlistURL+"/bake", nil)
	mustStatus(t, resp, http.StatusOK)
	var bandConcertID, bandRev string
	unmarshalField(t, cb, "concertId", &bandConcertID)
	unmarshalField(t, cb, "currentRev", &bandRev)
	if bandConcertID != sl.ID || bandRev != "1" {
		t.Fatalf("band bake concertId=%q rev=%q, want %q / 1", bandConcertID, bandRev, sl.ID)
	}

	// Bob bakes HIS variant (scope=mine) → concertId == <setlist>~<bob>, rev 1
	// (independent of the band bake's rev).
	resp, vb := bob.do(http.MethodPost, setlistURL+"/bake?scope=mine", nil)
	mustStatus(t, resp, http.StatusOK)
	var bobConcertID, bobRev string
	unmarshalField(t, vb, "concertId", &bobConcertID)
	unmarshalField(t, vb, "currentRev", &bobRev)
	if bobConcertID != sl.ID+"~"+bobID {
		t.Fatalf("bob variant concertId=%q, want %q", bobConcertID, sl.ID+"~"+bobID)
	}
	if bobRev != "1" {
		t.Fatalf("bob variant rev=%q, want 1 (independent per-variant numbering)", bobRev)
	}

	// Carol bakes HER own variant too.
	resp, _ = carol.do(http.MethodPost, setlistURL+"/bake?scope=mine", nil)
	mustStatus(t, resp, http.StatusOK)

	// Bob's concert list: the band concert + HIS variant, NOT carol's.
	resp, lb := bob.do(http.MethodGet, "/api/bands/"+band.ID+"/concerts", nil)
	mustStatus(t, resp, http.StatusOK)
	var concerts []struct {
		ConcertID string `json:"concertId"`
	}
	unmarshalField(t, lb, "concerts", &concerts)
	seen := map[string]bool{}
	for _, c := range concerts {
		seen[c.ConcertID] = true
	}
	if !seen[sl.ID] || !seen[sl.ID+"~"+bobID] {
		t.Fatalf("bob should see band concert + his variant; saw %v", seen)
	}
	if seen[sl.ID+"~"+carolID] {
		t.Fatalf("bob must NOT see carol's variant; saw %v", seen)
	}

	// Bob downloads his own variant → 200.
	resp, _ = bob.do(http.MethodGet, "/api/bands/"+band.ID+"/concerts/"+sl.ID+"~"+bobID+"/bundle", nil)
	mustStatus(t, resp, http.StatusOK)

	// Bob CANNOT download carol's variant (the negative) → forbidden.
	resp, _ = bob.do(http.MethodGet, "/api/bands/"+band.ID+"/concerts/"+sl.ID+"~"+carolID+"/bundle", nil)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("bob fetching carol's variant = %d, want 403", resp.StatusCode)
	}

	// A non-member cannot bake even their own variant (no band membership).
	outsider := &client{t: t, srv: srv}
	outsider.registerLogin("mallory", "pw")
	resp, _ = outsider.do(http.MethodPost, setlistURL+"/bake?scope=mine", nil)
	if resp.StatusCode < 400 {
		t.Fatalf("outsider scope=mine bake = %d, want >=400", resp.StatusCode)
	}
}

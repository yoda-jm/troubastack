package httpapi_test

import (
	"net/http"
	"testing"

	"troubastack/core/internal/app"
)

// T96 — the bake-progress read endpoint. An empty setlist bakes without the toolchain
// (succeeded 0/0), which is all this needs: the flow is header → GET → 404s → auth, not
// the per-song counting (that's proven in internal/bake).
func TestBakeProgress_endpoint(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}

	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)

	base := "/api/bands/" + band.ID + "/setlists/" + sl.ID
	resp, _ := admin.do(http.MethodPost, base+"/bake", nil)
	mustStatus(t, resp, http.StatusOK)
	bakeID := resp.Header.Get("X-Trouba-Bake-Id")
	if bakeID == "" {
		t.Fatal("bake response missing X-Trouba-Bake-Id header (T96)")
	}

	progURL := base + "/bakes/" + bakeID + "/progress"

	// Admin reads the terminal progress for its own bake.
	resp, pbody := admin.do(http.MethodGet, progURL, nil)
	mustStatus(t, resp, http.StatusOK)
	var state string
	var done, total int
	unmarshalField(t, pbody, "state", &state)
	unmarshalField(t, pbody, "done", &done)
	unmarshalField(t, pbody, "total", &total)
	if state != "succeeded" || done != 0 || total != 0 {
		t.Errorf("progress = {state:%q done:%d total:%d}, want succeeded 0/0 (empty setlist)", state, done, total)
	}

	// Unknown bake id → 404 (distinct from an empty 200).
	resp, _ = admin.do(http.MethodGet, base+"/bakes/deadbeefdeadbeef/progress", nil)
	mustStatus(t, resp, http.StatusNotFound)

	// A non-admin member cannot read progress — SAME authorisation as the bake (admin-only).
	resp, _ = member.do(http.MethodGet, progURL, nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("non-admin member read bake progress, got 200")
	}

	// Cross-band: another band's admin cannot read this bake id, even through their own band
	// path with the real id — the registry is scoped to the owning band, closing the leak.
	other := &client{t: t, srv: srv}
	otherBand := other.makeBand("carol", "Other")
	resp, _ = other.do(http.MethodGet, "/api/bands/"+otherBand.ID+"/setlists/"+sl.ID+"/bakes/"+bakeID+"/progress", nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("cross-band admin read another band's bake progress, got 200")
	}
}

// T99/B: the handler honours a client-supplied X-Trouba-Bake-Id REQUEST header — it echoes it back and
// the progress is readable under it (the round-trip the network-free e2e can't exercise).
func TestBakeProgress_suppliedIdHonoured(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	band := admin.makeBand("alice", "Band")

	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)
	base := "/api/bands/" + band.ID + "/setlists/" + sl.ID

	supplied := "12345678-1234-4234-8234-123456789abc"
	req, err := http.NewRequest(http.MethodPost, srv.URL+base+"/bake", nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	for _, ck := range admin.jar {
		req.AddCookie(ck)
	}
	req.Header.Set("X-Trouba-Bake-Id", supplied)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bake POST: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("bake status = %d, want 200", resp.StatusCode)
	}
	if got := resp.Header.Get("X-Trouba-Bake-Id"); got != supplied {
		t.Fatalf("echoed bake id = %q, want the supplied %q", got, supplied)
	}
	// The client can read progress under the id IT chose.
	resp2, pbody := admin.do(http.MethodGet, base+"/bakes/"+supplied+"/progress", nil)
	mustStatus(t, resp2, http.StatusOK)
	var state string
	unmarshalField(t, pbody, "state", &state)
	if state != "succeeded" {
		t.Errorf("progress under supplied id = %q, want succeeded", state)
	}
}

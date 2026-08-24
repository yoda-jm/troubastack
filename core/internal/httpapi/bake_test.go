package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/httpapi"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// awaitBake kicks the async bake (T103: POST returns 202 + the bake id) and polls the progress endpoint
// to a terminal state, returning the bake id + the terminal progress record. Fails the test on `failed`
// or timeout. This is the shape every client now uses: the poll is the source of truth for the outcome.
func awaitBake(t *testing.T, c *client, band, sl string) (string, map[string]json.RawMessage) {
	t.Helper()
	resp, _ := c.do(http.MethodPost, "/api/bands/"+band+"/setlists/"+sl+"/bake", nil)
	mustStatus(t, resp, http.StatusAccepted)
	id := resp.Header.Get("X-Trouba-Bake-Id")
	if id == "" {
		t.Fatal("bake 202 carried no X-Trouba-Bake-Id header")
	}
	prog := "/api/bands/" + band + "/setlists/" + sl + "/bakes/" + id + "/progress"
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r, body := c.do(http.MethodGet, prog, nil)
		if r.StatusCode == http.StatusOK {
			var state string
			unmarshalField(t, body, "state", &state)
			switch state {
			case "succeeded":
				return id, body
			case "failed":
				var e string
				if _, ok := body["error"]; ok {
					unmarshalField(t, body, "error", &e)
				}
				t.Fatalf("bake failed: %s", e)
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("bake did not reach a terminal state in time")
	return "", nil
}

// bakeServer builds ONE server with a real Baker (temp bakesDir) that all clients
// share — so an admin's bake is visible to a member's list/download. Bakes here use
// an EMPTY setlist, so poppler/web-bake are never invoked (no toolchain needed).
func bakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	baker := bake.New(svc, eng, bake.Config{BakesDir: t.TempDir()})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel) // stop the P201 autobaker goroutine when the test ends
	h, err := httpapi.Router(ctx, svc, eng, baker, false, "")
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}

func TestBakeEndpoints_authAndFlow(t *testing.T) {
	srv := bakeServer(t)
	admin := &client{t: t, srv: srv}
	member := &client{t: t, srv: srv}

	band := admin.makeBand("alice", "Band")
	member.registerLogin("bob", "pw")
	inviteAndAccept(t, admin, member, band.ID, "bob")

	// An (empty) setlist to bake.
	_, body := admin.do(http.MethodPost, "/api/bands/"+band.ID+"/setlists", map[string]string{"name": "Spring Gig"})
	var sl app.Setlist
	unmarshalField(t, body, "setlist", &sl)

	bakeURL := "/api/bands/" + band.ID + "/setlists/" + sl.ID + "/bake"

	_ = bakeURL
	// Non-admin member cannot bake (admin gate, I11 — auth happens before the async kick, still 403).
	resp, _ := member.do(http.MethodPost, bakeURL, nil)
	mustStatus(t, resp, http.StatusForbidden)

	// Admin bakes → rev 1, then re-bakes → rev 2. Each is now a KICK (202) whose outcome the poll
	// reports; the published concert's currentRev is verified via the concerts list below.
	awaitBake(t, admin, band.ID, sl.ID)
	awaitBake(t, admin, band.ID, sl.ID)

	// Member lists concerts (member-only) and sees this one, in the AvailableConcert
	// manifest shape: currentRev string, a songs array, downloadUrl.
	resp, lbody := member.do(http.MethodGet, "/api/bands/"+band.ID+"/concerts", nil)
	mustStatus(t, resp, http.StatusOK)
	var concerts []struct {
		ConcertID   string        `json:"concertId"`
		CurrentRev  string        `json:"currentRev"`
		Songs       []interface{} `json:"songs"`
		DownloadURL string        `json:"downloadUrl"`
	}
	unmarshalField(t, lbody, "concerts", &concerts)
	if len(concerts) != 1 || concerts[0].ConcertID != sl.ID || concerts[0].CurrentRev != "2" {
		t.Fatalf("concerts list = %+v, want one concert (rev \"2\") for setlist %s", concerts, sl.ID)
	}
	if concerts[0].Songs == nil || concerts[0].DownloadURL == "" {
		t.Fatalf("manifest concert missing songs array / downloadUrl: %+v", concerts[0])
	}

	// Member downloads the .tstage.
	resp, _ = member.do(http.MethodGet, concerts[0].DownloadURL, nil)
	mustStatus(t, resp, http.StatusOK)
	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Fatalf("download Content-Type = %q, want application/zip", ct)
	}

	// An outsider (non-member) is scoped out of the band's concerts.
	outsider := &client{t: t, srv: srv}
	outsider.registerLogin("carol", "pw")
	resp, _ = outsider.do(http.MethodGet, "/api/bands/"+band.ID+"/concerts", nil)
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("outsider should not list a band's concerts, got 200")
	}
}

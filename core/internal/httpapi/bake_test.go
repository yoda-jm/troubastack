package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/bake"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/httpapi"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// bakeServer builds ONE server with a real Baker (temp bakesDir) that all clients
// share — so an admin's bake is visible to a member's list/download. Bakes here use
// an EMPTY setlist, so poppler/web-bake are never invoked (no toolchain needed).
func bakeServer(t *testing.T) *httptest.Server {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	baker := bake.New(svc, eng, bake.Config{BakesDir: t.TempDir()})
	h, err := httpapi.Router(svc, eng, baker, false)
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

	// Non-admin member cannot bake (admin gate, I11).
	resp, _ := member.do(http.MethodPost, bakeURL, nil)
	mustStatus(t, resp, http.StatusForbidden)

	// Admin bakes → rev 1. currentRev is canonical (uint64 as a JSON STRING) so the
	// app deserializes it with A02's AvailableConcert mirror (B03).
	resp, cbody := admin.do(http.MethodPost, bakeURL, nil)
	mustStatus(t, resp, http.StatusOK)
	var rev string
	unmarshalField(t, cbody, "currentRev", &rev)
	if rev != "1" {
		t.Fatalf("first bake currentRev = %q, want \"1\"", rev)
	}

	// Re-bake bumps the rev.
	resp, cbody = admin.do(http.MethodPost, bakeURL, nil)
	mustStatus(t, resp, http.StatusOK)
	unmarshalField(t, cbody, "currentRev", &rev)
	if rev != "2" {
		t.Fatalf("re-bake currentRev = %q, want \"2\"", rev)
	}

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

package httpapi_test

import (
	"bytes"
	"context"
	"image"
	"image/png"
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

// gateRaster blocks Rasterize until `release` closes — OR returns ctx.Err() if the context is cancelled
// first. That second branch is the whole point of the T103 regression test: if a client hang-up were to
// cancel the bake's context (the old r.Context() shape), the gated raster would see Done and the bake
// would fail. Under the async design the bake runs on the SERVER ctx, so only `release` unblocks it.
type gateRaster struct {
	release <-chan struct{}
	png     []byte
}

func (r gateRaster) Rasterize(ctx context.Context, _ []byte) ([][]byte, error) {
	select {
	case <-r.release:
		return [][]byte{r.png}, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func tinyPNGBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// bakeServerWithRaster is bakeServer with an injected rasterizer (T103 DI seam) — and it returns the
// shared service so a test can set up a real song+file+setlist directly, then bake over HTTP.
func bakeServerWithRaster(t *testing.T, r bake.Rasterizer) (*httptest.Server, *app.Service) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	baker := bake.New(svc, eng, bake.Config{BakesDir: t.TempDir(), Raster: r})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	h, err := httpapi.Router(ctx, svc, eng, baker, false, "")
	if err != nil {
		t.Fatalf("Router: %v", err)
	}
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv, svc
}

// T103 §5.1 — the core regression guard. The bake POST must return BEFORE the bake completes, and a
// client that hangs up immediately must NOT cancel the bake. Both assertions fail against the old
// synchronous handler: it would block here on the gated raster (no prompt 202), and it ran the bake on
// r.Context() (the hang-up would cancel it).
func TestBakeAsync_hangUpDoesNotCancelBake(t *testing.T) {
	release := make(chan struct{})
	srv, svc := bakeServerWithRaster(t, gateRaster{release: release, png: tinyPNGBytes(t)})
	admin := &client{t: t, srv: srv}

	// Set up a band with one PDF-bearing song in a setlist — so the bake actually rasterises (hits the
	// gate). Done through the service; the admin then logs in over HTTP to drive the bake.
	u, err := svc.Register("admin", "Admin", "pw-admin", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, _ := svc.CreateBand(u, "Band")
	song, _ := svc.CreateSong(u, band.ID, "Test Song", "")
	if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	sl, _ := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if r, _ := admin.do(http.MethodPost, "/api/auth/login", map[string]string{"username": "admin", "password": "pw-admin"}); r.StatusCode != http.StatusOK {
		t.Fatalf("login: status %d", r.StatusCode)
	}

	// Fire the bake POST on a CANCELLABLE context (a real socket).
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL+"/api/bands/"+band.ID+"/setlists/"+sl.ID+"/bake", nil)
	for _, ck := range admin.jar {
		req.AddCookie(ck)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("bake POST: %v", err)
	}
	// GUARD 1: the POST returned while the raster is STILL GATED — the bake has not completed. A
	// synchronous handler could not have gotten here.
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("bake status = %d, want 202 (async kick before completion)", resp.StatusCode)
	}
	bakeID := resp.Header.Get("X-Trouba-Bake-Id")
	resp.Body.Close()
	if bakeID == "" {
		t.Fatal("202 carried no bake id")
	}

	// HANG UP: cancel the request context (a dropped socket), let it propagate, THEN release the raster.
	cancel()
	time.Sleep(80 * time.Millisecond)
	close(release)

	// GUARD 2: the bake still reaches succeeded — the hang-up did not cancel it.
	prog := "/api/bands/" + band.ID + "/setlists/" + sl.ID + "/bakes/" + bakeID + "/progress"
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		r, body := admin.do(http.MethodGet, prog, nil)
		if r.StatusCode == http.StatusOK {
			var state string
			unmarshalField(t, body, "state", &state)
			if state == "succeeded" {
				return
			}
			if state == "failed" {
				var e string
				if _, ok := body["error"]; ok {
					unmarshalField(t, body, "error", &e)
				}
				t.Fatalf("the hung-up bake was cancelled/failed: %s", e)
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatal("bake never reached succeeded after the hang-up")
}

package bake

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// The raw overlay-worker failure VLL actually saw: a multi-line Node stack trace carrying an ABSOLUTE
// server path — the thing that must never reach a band member on either channel.
const rawNodeTrace = "web/bake worker (node /home/yoda/dev/git/troubastack/web/bake/dist/cli.js): exit status 1: " +
	"node:internal/modules/cjs/loader:1503\n  throw err;\n  ^\nError: Cannot find module '/home/yoda/x/cli.js'\n" +
	"    at Module._resolveFilename (node:internal/modules/cjs/loader:1500:15)\n"

type failingOverlays struct{}

func (failingOverlays) RenderBatch(context.Context, []overlaySong) (map[string][]renderedOverlay, error) {
	return nil, fmt.Errorf("%s", rawNodeTrace)
}

// seed1Annotated: one titled song with one annotation object, so the bake produces a NON-empty overlay
// batch — i.e. RenderBatch is actually invoked and a failing renderer is reached (an un-annotated song
// skips the spawn entirely, T97).
func seed1Annotated(t *testing.T) (*app.Service, *engine.Engine, app.User, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, _ := svc.Register("admin", "Admin", "password123", "")
	band, _ := svc.CreateBand(u, "Band")
	sl, _ := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	song, _ := svc.CreateSong(u, band.ID, "Dirty Old Town", "")
	if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
		t.Fatalf("upload: %v", err)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind:     domain.KindLayerCreate,
		Layer:    &domain.Layer{ID: "L1", Name: "Marks", OwnerID: u.ID, Zone: domain.ZonePersonal, Order: 0, Access: domain.AccessRW},
		AuthorID: u.ID,
	}); err != nil {
		t.Fatalf("layer: %v", err)
	}
	if _, err := eng.Apply(song.ID, domain.Mutation{
		Kind: domain.KindCreate, UUID: "o1", AuthorID: u.ID,
		Object: &domain.Object{UUID: "o1", LayerID: "L1", Type: domain.TypeRect, Page: 0, Version: 1,
			Points: []domain.Point{{X: 0.2, Y: 0.1}, {X: 0.5, Y: 0.3}}, Style: domain.Style{Color: "#e11d48", Opacity: 1, Width: 0.004}},
	}); err != nil {
		t.Fatalf("object: %v", err)
	}
	if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add: %v", err)
	}
	return svc, eng, u, band.ID, sl.ID
}

// bakerWith builds a Baker with the given raster + overlays, capturing the server log into logbuf and
// every progress publish into rec. bakeID is fixed so the terminal record is readable.
func bakerWith(t *testing.T, svc *app.Service, eng *engine.Engine, r Rasterizer, ov OverlayRenderer, rec *progRec, logbuf *bytes.Buffer) *Baker {
	t.Helper()
	return &Baker{
		svc: svc, eng: eng, raster: r, overlays: ov, bakesDir: t.TempDir(), now: func() int64 { return 1700000000 },
		progress: newProgressRegistry(nil), newBakeID: func() string { return "bake-1" },
		onProgress: func(id string, p BakeProgress) { rec.record(id, p) },
		logf:       func(f string, a ...any) { fmt.Fprintf(logbuf, f+"\n", a...) },
	}
}

// assertUserSafe: no newline, no /-rooted path, no `at ` stack frame — a band member never sees those.
func assertUserSafe(t *testing.T, where, msg string) {
	t.Helper()
	if msg == "" {
		t.Errorf("%s: empty message", where)
	}
	if strings.Contains(msg, "\n") {
		t.Errorf("%s: message contains a newline: %q", where, msg)
	}
	if strings.Contains(msg, "/") {
		t.Errorf("%s: message contains a /-rooted path: %q", where, msg)
	}
	if strings.Contains(msg, "at ") {
		t.Errorf("%s: message contains a stack frame (\"at \"): %q", where, msg)
	}
}

// T102 §4, channel 1 — the POST response body. writeErr ships Bake's returned error verbatim, so this
// asserts on that returned error. And the full detail must be in the server log for the same run.
func TestBakeErrors_rendererFailure_returnedErrorIsUserSafe(t *testing.T) {
	svc, eng, u, band, sl := seed1Annotated(t)
	var logbuf bytes.Buffer
	b := bakerWith(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, failingOverlays{}, newProgRec(), &logbuf)

	_, _, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err == nil {
		t.Fatal("bake should have failed (renderer errors)")
	}
	assertUserSafe(t, "POST body", err.Error())
	if !strings.Contains(err.Error(), "renderer") {
		t.Errorf("message should name the renderer problem, got %q", err.Error())
	}
	// The detail — stderr + the absolute path — went to the server log, and nowhere else.
	if !strings.Contains(logbuf.String(), "Cannot find module") || !strings.Contains(logbuf.String(), "cli.js") {
		t.Errorf("full detail was not logged server-side: %q", logbuf.String())
	}
}

// T102 §4, channel 2 — BakeProgress.Error (a SEPARATE code path: baker.go publishes it in the defer).
func TestBakeErrors_rendererFailure_progressRecordIsUserSafe(t *testing.T) {
	svc, eng, u, band, sl := seed1Annotated(t)
	var logbuf bytes.Buffer
	b := bakerWith(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, failingOverlays{}, newProgRec(), &logbuf)

	_, id, _ := b.Bake(context.Background(), band, sl, u, nil, "")
	p, ok := b.Progress(band, sl, id)
	if !ok || p.State != BakeFailed {
		t.Fatalf("expected a terminal failed progress record, got %+v ok=%v", p, ok)
	}
	assertUserSafe(t, "progress record", p.Error)
	if !strings.Contains(p.Error, "renderer") {
		t.Errorf("progress error should name the renderer problem, got %q", p.Error)
	}
}

// T124 — an empty setlist (no songs) must end in a terminal FAILED record, not a song-less "success".
// This is the defect the 2026-08-28 flow check actually hit: with the add-song beat broken, an empty
// setlist produced a bundle of nothing and reported success. The terminal state is now derived from the
// artefact (0 songs produced), not from a clean pipeline return.
func TestBakeErrors_emptySetlist_isTerminalFailed(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, _ := svc.Register("admin", "Admin", "password123", "")
	band, _ := svc.CreateBand(u, "Band")
	sl, _ := svc.CreateSetlist(u, band.ID, "Empty Gig", "", "", "") // NO items added
	var logbuf bytes.Buffer
	b := bakerWith(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, fakeOverlays{png: tinyPNG(t, 40, 56)}, newProgRec(), &logbuf)

	_, id, err := b.Bake(context.Background(), band.ID, sl.ID, u, nil, "")
	if err == nil {
		t.Fatal("baking an empty setlist should FAIL, not produce a song-less concert")
	}
	p, ok := b.Progress(band.ID, sl.ID, id)
	if !ok || p.State != BakeFailed {
		t.Fatalf("empty setlist: expected a terminal FAILED record, got %+v ok=%v", p, ok)
	}
	if !strings.Contains(p.Error, "no songs") {
		t.Errorf("failure should name the empty-setlist cause, got %q", p.Error)
	}
	assertUserSafe(t, "progress record", p.Error)
}

// A song-scoped failure (poppler couldn't rasterise the sheet) names the song and stays user-safe.
func TestBakeErrors_rasterFailure_namesTheSong(t *testing.T) {
	svc, eng, u, band, sl := seed1Annotated(t)
	var logbuf bytes.Buffer
	rawPoppler := fmt.Errorf("pdftoppm (/usr/bin/pdftoppm): exit status 1: Syntax Error:\n  broken xref\n")
	b := bakerWith(t, svc, eng, errRaster{err: rawPoppler}, fakeOverlays{png: tinyPNG(t, 40, 56)}, newProgRec(), &logbuf)

	_, _, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err == nil {
		t.Fatal("bake should have failed (raster errors)")
	}
	assertUserSafe(t, "raster POST body", err.Error())
	if !strings.Contains(err.Error(), "Dirty Old Town") || !strings.Contains(err.Error(), "sheet music") {
		t.Errorf("message should name the song and the sheet-music problem, got %q", err.Error())
	}
	if !strings.Contains(logbuf.String(), "broken xref") {
		t.Errorf("full poppler detail was not logged: %q", logbuf.String())
	}
}

// The humanize choke point is the guarantee that no FUTURE failure point can leak a stderr blob: an
// unanticipated raw error (never wrapped by fail) is still sanitised on the wire and logged in full.
func TestBakeErrors_unanticipatedRawError_isSanitised(t *testing.T) {
	var logbuf bytes.Buffer
	b := &Baker{logf: func(f string, a ...any) { fmt.Fprintf(&logbuf, f+"\n", a...) }}
	got := b.humanize(fmt.Errorf("leaked stderr\n  at /server/secret/path.go:9\n"))
	assertUserSafe(t, "humanize fallback", got.Error())
	if !strings.Contains(logbuf.String(), "leaked stderr") {
		t.Errorf("humanize should log the raw error it swallowed: %q", logbuf.String())
	}
}

// End-to-end proof that the defer's humanize guards a path that does NOT call fail: a raw OS error
// (which carries a filesystem PATH) from the infra layer must still be sanitised on both channels.
// Here the bakes dir is a FILE, so MkdirAll fails with "mkdir <path>: not a directory".
func TestBakeErrors_infraFailure_sanitisedByDefer(t *testing.T) {
	svc, eng, u, band, sl := seed1Annotated(t)
	var logbuf bytes.Buffer
	notADir := filepath.Join(t.TempDir(), "bakes-is-a-file")
	if err := os.WriteFile(notADir, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := bakerWith(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, fakeOverlays{png: tinyPNG(t, 40, 56)}, newProgRec(), &logbuf)
	b.bakesDir = notADir

	_, id, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err == nil {
		t.Fatal("bake should have failed (bakes dir is a file)")
	}
	assertUserSafe(t, "infra POST body", err.Error()) // the raw OS error names a path — humanize must strip it
	p, ok := b.Progress(band, sl, id)
	if !ok || p.State != BakeFailed {
		t.Fatalf("expected failed progress, got %+v ok=%v", p, ok)
	}
	assertUserSafe(t, "infra progress record", p.Error)
	if logbuf.Len() == 0 {
		t.Error("the raw infra error was not logged server-side")
	}
}

package bake

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/app/memrepo"
	"troubastack/core/internal/engine"
	"troubastack/core/internal/store"
	"troubastack/core/internal/store/memstore"
)

// seedN builds a band with an n-song setlist (each song a distinct title + a PDF), the
// substrate for the progress tests. No annotations needed — progress is per song.
func seedN(t *testing.T, n int) (*app.Service, *engine.Engine, app.User, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, err := svc.Register("admin", "Admin", "password123", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := svc.CreateBand(u, "Band")
	if err != nil {
		t.Fatalf("band: %v", err)
	}
	sl, err := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	for i := 0; i < n; i++ {
		song, err := svc.CreateSong(u, band.ID, fmt.Sprintf("Song %d", i+1), "")
		if err != nil {
			t.Fatalf("song: %v", err)
		}
		if _, err := svc.UploadSongFile(u, band.ID, song.ID, "score.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
			t.Fatalf("upload: %v", err)
		}
		if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
			t.Fatalf("add item: %v", err)
		}
	}
	return svc, eng, u, band.ID, sl.ID
}

// progRec records every publish, partitioned by bake id. Concurrency-safe: the bake
// publishes from its own goroutine (two of them in the concurrent test).
type progRec struct {
	mu  sync.Mutex
	seq map[string][]BakeProgress
}

func newProgRec() *progRec { return &progRec{seq: map[string][]BakeProgress{}} }
func (r *progRec) record(id string, p BakeProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq[id] = append(r.seq[id], p)
}
func (r *progRec) get(id string) []BakeProgress {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]BakeProgress(nil), r.seq[id]...)
}

// progBaker wires a Baker with a progress registry, a recording observer, and a
// deterministic, thread-safe id minter (bake-1, bake-2, …).
func progBaker(t *testing.T, svc *app.Service, eng *engine.Engine, r Rasterizer, rec *progRec) *Baker {
	t.Helper()
	var ctr int64
	return &Baker{
		svc: svc, eng: eng, raster: r, overlays: fakeOverlays{png: tinyPNG(t, 40, 56)},
		bakesDir: t.TempDir(), now: func() int64 { return 1700000000 },
		progress:   newProgressRegistry(nil),
		newBakeID:  func() string { return fmt.Sprintf("bake-%d", atomic.AddInt64(&ctr, 1)) },
		onProgress: func(id string, p BakeProgress) { rec.record(id, p) },
	}
}

// runningWithSong returns the running snapshots that NAME a song — the N per-song
// updates, excluding the initial running-0/N and the song-less "finishing" tail update.
func runningWithSong(seq []BakeProgress) []BakeProgress {
	var out []BakeProgress
	for _, p := range seq {
		if p.State == BakeRunning && p.Song != "" {
			out = append(out, p)
		}
	}
	return out
}

// T96 §6: N updates, done advancing 1..N, each naming the song about to be baked; then a
// terminal succeeded. Assert the SEQUENCE, not just the final value.
func TestBake_Progress_SequenceAndTerminal(t *testing.T) {
	const n = 4
	svc, eng, u, band, sl := seedN(t, n)
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)

	_, id, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	detail, err := svc.Setlist(u, band, sl)
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	seq := rec.get(id)
	running := runningWithSong(seq)
	if len(running) != n {
		t.Fatalf("got %d per-song running updates, want %d — sequence %+v", len(running), n, seq)
	}
	for i, p := range running {
		if p.Done != i+1 || p.Total != n {
			t.Errorf("update %d: done=%d total=%d, want done=%d total=%d", i, p.Done, p.Total, i+1, n)
		}
		if p.Song != detail.Items[i].SongTitle {
			t.Errorf("update %d names %q, want the song about to bake %q", i, p.Song, detail.Items[i].SongTitle)
		}
	}
	// T96/T98 tail: after all N songs are staged there must be a running update with done==total
	// and NO song — the "finishing" signal for the post-loop overlay+assembly work. done==total must
	// not read as finished; only the terminal state does. (Fable: the A39 shape if they disagree.)
	var sawFinishing bool
	for _, p := range seq {
		if p.State == BakeRunning && p.Song == "" && p.Done == n && p.Total == n {
			sawFinishing = true
		}
	}
	if !sawFinishing {
		t.Errorf("no running/done==total/song-less finishing update — done==total would imply finished")
	}
	last := seq[len(seq)-1]
	if last.State != BakeSucceeded || last.Done != n || last.Total != n {
		t.Errorf("terminal = %+v, want succeeded %d/%d", last, n, n)
	}
	// The registry read the endpoint would serve.
	if p, ok := b.Progress(band, sl, id); !ok || p.State != BakeSucceeded {
		t.Errorf("registry final = %+v ok=%v, want succeeded", p, ok)
	}
}

// T96 §6: empty setlist ⇒ succeeded, done == total == 0, and NO per-song update ("song 1 of 0").
func TestBake_Progress_EmptySetlist(t *testing.T) {
	svc, eng, u, band, sl := seedN(t, 0)
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)

	// T124: an empty setlist now FAILS the bake (a bake of nothing is not a concert) — this test
	// previously asserted it SUCCEEDED, which was the defect. The progress mechanics it guards still
	// hold: no per-song updates are emitted, and the terminal record is reached.
	_, id, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err == nil {
		t.Fatal("baking an empty setlist should fail (T124), not succeed")
	}
	if r := runningWithSong(rec.get(id)); len(r) != 0 {
		t.Errorf("empty setlist emitted %d per-song updates, want 0: %+v", len(r), r)
	}
	p, ok := b.Progress(band, sl, id)
	if !ok || p.State != BakeFailed {
		t.Errorf("empty setlist terminal = %+v ok=%v, want a FAILED record (T124)", p, ok)
	}
}

// errRaster fails every rasterize — the substrate for the failure + cancellation tests.
type errRaster struct{ err error }

func (r errRaster) Rasterize(ctx context.Context, _ []byte) ([][]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return nil, r.err
}

// T96 §4/§6: a failed bake's terminal state NAMES the song that failed — the whole value
// of the readout when a bake dies before a gig.
func TestBake_Progress_FailureNamesSong(t *testing.T) {
	svc, eng, u, band, sl := seedN(t, 3)
	rec := newProgRec()
	b := progBaker(t, svc, eng, errRaster{err: fmt.Errorf("boom")}, rec)

	_, id, err := b.Bake(context.Background(), band, sl, u, nil, "")
	if err == nil {
		t.Fatal("bake should have failed (raster errors)")
	}
	detail, _ := svc.Setlist(u, band, sl)
	p, ok := b.Progress(band, sl, id)
	if !ok {
		t.Fatal("failed bake left no progress")
	}
	if p.State != BakeFailed {
		t.Errorf("state = %q, want failed", p.State)
	}
	if p.Song != detail.Items[0].SongTitle {
		t.Errorf("failed state names %q, want the failing song %q", p.Song, detail.Items[0].SongTitle)
	}
	if p.Error == "" {
		t.Error("failed state carries no error detail")
	}
}

// T96 §4: a client disconnect cancels ctx; the terminal state must RESOLVE, never sit on
// "running" forever (the A39 stall in a different costume).
func TestBake_Progress_CancelledResolvesTerminal(t *testing.T) {
	svc, eng, u, band, sl := seedN(t, 3)
	rec := newProgRec()
	b := progBaker(t, svc, eng, errRaster{err: fmt.Errorf("unused")}, rec)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // client gone before the first song
	_, id, err := b.Bake(ctx, band, sl, u, nil, "")
	if err == nil {
		t.Fatal("cancelled bake should error")
	}
	p, ok := b.Progress(band, sl, id)
	if !ok {
		t.Fatal("cancelled bake left no progress")
	}
	if p.State == BakeRunning {
		t.Fatalf("cancelled bake left state=running (A39 stall); want a terminal state, got %+v", p)
	}
}

// T96 §6 (the load-bearing one): two concurrent bakes of the SAME setlist keep SEPARATE
// progress — distinct ids, neither observing the other's counter. This is what makes the
// bake-id-not-setlist-id decision matter.
func TestBake_Progress_ConcurrentDistinctIds(t *testing.T) {
	const n = 3
	svc, eng, u, band, sl := seedN(t, n)
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)

	var wg sync.WaitGroup
	ids := make([]string, 2)
	errs := make([]error, 2)
	for k := 0; k < 2; k++ {
		wg.Add(1)
		go func(k int) {
			defer wg.Done()
			_, ids[k], errs[k] = b.Bake(context.Background(), band, sl, u, nil, "")
		}(k)
	}
	wg.Wait()
	for k, err := range errs {
		if err != nil {
			t.Fatalf("bake %d: %v", k, err)
		}
	}
	if ids[0] == ids[1] || ids[0] == "" {
		t.Fatalf("concurrent bakes shared a progress key: %q / %q", ids[0], ids[1])
	}
	// Each id's per-song sequence is its OWN 1..N — no counter bleed from the other bake.
	for _, id := range ids {
		running := runningWithSong(rec.get(id))
		if len(running) != n {
			t.Fatalf("id %s saw %d per-song updates, want %d", id, len(running), n)
		}
		for i, p := range running {
			if p.Done != i+1 || p.Total != n {
				t.Errorf("id %s update %d: done=%d total=%d, want %d/%d", id, i, p.Done, p.Total, i+1, n)
			}
		}
		if p, ok := b.Progress(band, sl, id); !ok || p.State != BakeSucceeded {
			t.Errorf("id %s final = %+v ok=%v, want succeeded", id, p, ok)
		}
	}
}

// The registry's bound + scope, in isolation: cross-band reads are hidden, a terminal
// entry is readable briefly then evicted, an unknown id is not found.
func TestProgressRegistry_ScopeAndExpiry(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := newProgressRegistry(func() time.Time { return now })

	r.set("id1", "bandA", "sl1", BakeProgress{State: BakeRunning, Done: 1, Total: 3})
	if _, ok := r.get("id1", "bandA", "sl1"); !ok {
		t.Fatal("running entry should be readable by its owner")
	}
	if _, ok := r.get("id1", "bandB", "sl1"); ok {
		t.Fatal("cross-band read must be hidden (no existence oracle)")
	}
	if _, ok := r.get("id1", "bandA", "other"); ok {
		t.Fatal("cross-setlist read must be hidden")
	}
	if _, ok := r.get("nope", "bandA", "sl1"); ok {
		t.Fatal("unknown id must be not-found")
	}

	r.set("id1", "bandA", "sl1", BakeProgress{State: BakeSucceeded, Done: 3, Total: 3})
	now = now.Add(4 * time.Minute) // within the 5-minute terminal TTL
	if _, ok := r.get("id1", "bandA", "sl1"); !ok {
		t.Fatal("a just-finished bake must stay readable so a poller sees the ending")
	}
	now = now.Add(2 * time.Minute) // now 6 min > terminal TTL
	if _, ok := r.get("id1", "bandA", "sl1"); ok {
		t.Fatal("a terminal entry past its TTL must be evicted (no unbounded growth)")
	}
}

// ---- T99/B: client-supplied bake id ----

// A well-formed, free supplied id is honoured, so the client can poll progress under the id it chose.
func TestBake_SuppliedID_HonouredAndReadable(t *testing.T) {
	svc, eng, u, band, sl := seedN(t, 2)
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)
	supplied := "11111111-2222-4333-8444-555555555555"

	_, id, err := b.Bake(context.Background(), band, sl, u, nil, supplied)
	if err != nil {
		t.Fatalf("bake: %v", err)
	}
	if id != supplied {
		t.Fatalf("effective id = %q, want the supplied %q", id, supplied)
	}
	if p, ok := b.Progress(band, sl, supplied); !ok || p.State != BakeSucceeded {
		t.Errorf("progress under supplied id = %+v ok=%v, want succeeded", p, ok)
	}
}

// A malformed supplied id is ignored — the server mints its own, and the junk never becomes a key.
func TestBake_SuppliedID_MalformedIgnored(t *testing.T) {
	svc, eng, u, band, sl := seedN(t, 1)
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)

	_, id, err := b.Bake(context.Background(), band, sl, u, nil, "not-a-uuid; DROP TABLE")
	if err != nil {
		t.Fatalf("bake: %v (a bad progress hint must never fail a bake)", err)
	}
	if id == "not-a-uuid; DROP TABLE" {
		t.Fatal("malformed supplied id was honoured")
	}
	if _, ok := b.Progress(band, sl, "not-a-uuid; DROP TABLE"); ok {
		t.Error("malformed id became a registry key")
	}
	if _, ok := b.Progress(band, sl, id); !ok {
		t.Error("server-minted fallback id is not readable")
	}
}

// Fable's condition 2: two bands, the SAME supplied id — neither clobbers the other's readout.
func TestBake_SuppliedID_NoClobberAcrossBands(t *testing.T) {
	svc := app.NewService(memrepo.New())
	svc.WithBlobStore(blob.NewMem())
	eng := engine.New(memstore.New().(store.HistoryAware))
	u, _ := svc.Register("admin", "Admin", "password123", "")
	mk := func(name string) (string, string) {
		band, _ := svc.CreateBand(u, name)
		sl, _ := svc.CreateSetlist(u, band.ID, "Gig", "", "", "")
		song, _ := svc.CreateSong(u, band.ID, "S", "")
		if _, err := svc.UploadSongFile(u, band.ID, song.ID, "s.pdf", "application/pdf", []byte("%PDF-1.4 fixture")); err != nil {
			t.Fatalf("upload: %v", err)
		}
		if _, err := svc.AddSetlistItem(u, band.ID, sl.ID, song.ID); err != nil {
			t.Fatalf("add: %v", err)
		}
		return band.ID, sl.ID
	}
	bandA, slA := mk("A")
	bandB, slB := mk("B")
	rec := newProgRec()
	b := progBaker(t, svc, eng, fakeRaster{pages: 1, png: tinyPNG(t, 40, 56)}, rec)
	supplied := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"

	_, idA, errA := b.Bake(context.Background(), bandA, slA, u, nil, supplied)
	_, idB, errB := b.Bake(context.Background(), bandB, slB, u, nil, supplied)
	if errA != nil || errB != nil {
		t.Fatalf("bakes: %v / %v", errA, errB)
	}
	if idA != supplied {
		t.Fatalf("band A didn't get the free supplied id: %q", idA)
	}
	if idB == supplied {
		t.Fatal("band B clobbered band A's supplied id (must mint its own on collision)")
	}
	if p, ok := b.Progress(bandA, slA, idA); !ok || p.State != BakeSucceeded {
		t.Errorf("band A progress lost/clobbered: %+v ok=%v", p, ok)
	}
	if p, ok := b.Progress(bandB, slB, idB); !ok || p.State != BakeSucceeded {
		t.Errorf("band B progress lost: %+v ok=%v", p, ok)
	}
	if _, ok := b.Progress(bandB, slB, idA); ok {
		t.Error("band B could read band A's progress by id (cross-band leak)")
	}
}

func TestProgressRegistry_Claim(t *testing.T) {
	now := time.Unix(1700000000, 0)
	r := newProgressRegistry(func() time.Time { return now })
	if !r.claim("id1", "bandA", "sl1") {
		t.Fatal("claim of a free id should succeed")
	}
	if r.claim("id1", "bandA", "sl1") {
		t.Fatal("claim of a taken id must fail")
	}
	if r.claim("id1", "bandB", "sl2") {
		t.Fatal("claim must fail regardless of band — no cross-band clobber")
	}
	now = now.Add(31 * time.Minute) // past the running TTL
	if !r.claim("id1", "bandB", "sl2") {
		t.Fatal("an expired id should be reclaimable")
	}
}

func TestValidBakeID(t *testing.T) {
	good := []string{
		"11111111-2222-4333-8444-555555555555",
		"aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee",
		"AAAAAAAA-BBBB-CCCC-DDDD-EEEEEEEEEEEE",
	}
	bad := []string{
		"", "not-a-uuid",
		"11111111-2222-4333-8444-55555555555",   // too short
		"11111111-2222-4333-8444-5555555555555", // too long
		"11111111_2222_4333_8444_555555555555",  // wrong separators
		"11111111-2222-4333-8444-55555555555g",  // non-hex
		"1111111112222433384445555555555555556", // no dashes
	}
	for _, s := range good {
		if !validBakeID(s) {
			t.Errorf("valid UUID rejected: %q", s)
		}
	}
	for _, s := range bad {
		if validBakeID(s) {
			t.Errorf("invalid id accepted: %q", s)
		}
	}
}

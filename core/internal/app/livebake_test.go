package app_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/memrepo"
)

// fakeBaker counts bakes per setlist so a test can assert "N commits → 1 bake".
type fakeBaker struct {
	mu    sync.Mutex
	count map[string]int
}

func (f *fakeBaker) bake(_ context.Context, _ string, setlistID string, _ app.User) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.count == nil {
		f.count = map[string]int{}
	}
	f.count[setlistID]++
	return nil
}
func (f *fakeBaker) get(setlistID string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.count[setlistID]
}

// liveBakeFixture: a band with an admin, a song in a setlist, live mode ON, and an
// AutoBaker with an injected clock + counting baker. Returns the pieces + song/setlist ids.
func liveBakeFixture(t *testing.T, clock *time.Time) (*app.AutoBaker, *fakeBaker, string, string) {
	t.Helper()
	svc := app.NewService(memrepo.New()).WithClock(func() time.Time { return *clock })
	admin, err := svc.Register("amy", "Amy", "pass1234", "")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	band, err := svc.CreateBand(admin, "The Band")
	if err != nil {
		t.Fatalf("band: %v", err)
	}
	song, err := svc.CreateSong(admin, band.ID, "The Open Road", "Oasis")
	if err != nil {
		t.Fatalf("song: %v", err)
	}
	sl, err := svc.CreateSetlist(admin, band.ID, "Sat", "", "", "")
	if err != nil {
		t.Fatalf("setlist: %v", err)
	}
	if _, err := svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID); err != nil {
		t.Fatalf("add item: %v", err)
	}
	if _, err := svc.SetSetlistLive(admin, band.ID, sl.ID, true); err != nil {
		t.Fatalf("live on: %v", err)
	}
	fb := &fakeBaker{}
	win := 8 * time.Second
	ab := app.NewAutoBaker(svc, fb.bake, func() time.Time { return *clock }, win)
	return ab, fb, song.ID, sl.ID
}

func TestAutoBaker_debounceCoalesces(t *testing.T) {
	base := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	clock := base
	ab, fb, songID, slID := liveBakeFixture(t, &clock)

	// A burst of 5 commits within the quiet window.
	for i := 0; i < 5; i++ {
		clock = base.Add(time.Duration(i) * time.Second)
		ab.Notify(songID)
	}
	// Not yet elapsed → no bake.
	clock = base.Add(6 * time.Second)
	ab.MaybeBakeForTest(clock)
	if got := fb.get(slID); got != 0 {
		t.Fatalf("bake fired early: %d, want 0", got)
	}
	// Quiet period elapses after the LAST commit (t=4s) + 8s = t=12s.
	clock = base.Add(13 * time.Second)
	ab.MaybeBakeForTest(clock)
	waitFor(t, func() bool { return fb.get(slID) == 1 })
	if got := fb.get(slID); got != 1 {
		t.Fatalf("burst of 5 commits baked %d times, want exactly 1", got)
	}

	// A fresh commit after the bake arms a SECOND bake.
	clock = base.Add(20 * time.Second)
	ab.Notify(songID)
	clock = base.Add(30 * time.Second)
	ab.MaybeBakeForTest(clock)
	waitFor(t, func() bool { return fb.get(slID) == 2 })
	if got := fb.get(slID); got != 2 {
		t.Fatalf("second burst baked total %d, want 2", got)
	}
}

func TestAutoBaker_expiredSetlistDoesNotBake(t *testing.T) {
	base := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	clock := base
	ab, fb, songID, slID := liveBakeFixture(t, &clock)

	// Jump PAST the live-mode window → the setlist is no longer live.
	clock = base.Add(app.LiveModeWindow + time.Minute)
	ab.Notify(songID) // resolves to no live setlist → nothing armed
	clock = clock.Add(app.LiveBakeWindow + time.Second)
	ab.MaybeBakeForTest(clock)
	if got := fb.get(slID); got != 0 {
		t.Fatalf("expired live mode baked %d times, want 0", got)
	}
}

func TestAutoBaker_notLiveNeverBakes(t *testing.T) {
	base := time.Date(2026, 7, 12, 20, 0, 0, 0, time.UTC)
	clock := base
	// Build a fixture then turn live mode OFF.
	svc := app.NewService(memrepo.New()).WithClock(func() time.Time { return clock })
	admin, _ := svc.Register("amy", "Amy", "pass1234", "")
	band, _ := svc.CreateBand(admin, "B")
	song, _ := svc.CreateSong(admin, band.ID, "S", "")
	sl, _ := svc.CreateSetlist(admin, band.ID, "SL", "", "", "")
	_, _ = svc.AddSetlistItem(admin, band.ID, sl.ID, song.ID)
	// (never enabled live) — commits must never autobake.
	fb := &fakeBaker{}
	ab := app.NewAutoBaker(svc, fb.bake, func() time.Time { return clock }, 8*time.Second)
	ab.Notify(song.ID)
	clock = clock.Add(time.Minute)
	ab.MaybeBakeForTest(clock)
	if got := fb.get(sl.ID); got != 0 {
		t.Fatalf("non-live setlist baked %d times, want 0", got)
	}
}

// waitFor spins briefly for an async condition (bakes run in a goroutine).
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := makeDeadline()
	for !cond() {
		if atomic.LoadInt64(deadline) == 0 {
			return
		}
		time.Sleep(2 * time.Millisecond)
		atomic.AddInt64(deadline, -1)
	}
}
func makeDeadline() *int64 { d := int64(500); return &d } // ~1s of 2ms spins

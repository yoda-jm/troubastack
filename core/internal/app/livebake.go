package app

import (
	"context"
	"sync"
	"time"
)

// LiveBakeWindow is the debounce quiet period (P201 stage 1b): after the LAST
// annotation commit to a live setlist's songs, wait this long with no further commit
// before auto-baking, so a burst of strokes coalesces into ONE bake.
const LiveBakeWindow = 8 * time.Second

// bakeFunc performs one bake of a setlist as an actor. Injected (a closure over the
// real bake.Baker) so this package does NOT import bake — bake imports app, and the
// closure is wired where both are visible (httpapi), avoiding an import cycle.
type bakeFunc func(ctx context.Context, bandID, setlistID string, actor User) error

// AutoBaker debounces annotation commits into auto-bakes for setlists in rehearsal
// live mode (P201/I11). Policy is separated from the clock: Notify records that a
// live setlist saw a commit; a ticker (or a test) calls maybeBake(now), which bakes
// every setlist whose quiet period has elapsed. This makes the debounce testable with
// a fake clock and no timer flakiness. Concurrent same-setlist bakes are safe by
// design (B08/B09), and a bake in flight for a setlist is never doubled up.
type AutoBaker struct {
	svc  *Service
	bake bakeFunc
	now  func() time.Time
	win  time.Duration

	mu       sync.Mutex
	pending  map[string]pendingBake // setlistID → the debounce state
	inflight map[string]bool        // setlistID → a bake is currently running
}

type pendingBake struct {
	bandID   string
	actorID  string
	lastSeen time.Time
}

// NewAutoBaker builds an AutoBaker. now defaults to time.Now if nil; win to
// LiveBakeWindow if zero (tests inject both).
func NewAutoBaker(svc *Service, bake bakeFunc, now func() time.Time, win time.Duration) *AutoBaker {
	if now == nil {
		now = time.Now
	}
	if win <= 0 {
		win = LiveBakeWindow
	}
	return &AutoBaker{
		svc:      svc,
		bake:     bake,
		now:      now,
		win:      win,
		pending:  map[string]pendingBake{},
		inflight: map[string]bool{},
	}
}

// Notify records an annotation commit for a song. For each live setlist containing
// the song it (re)arms the debounce, so the bake fires only after the quiet period
// following the LAST commit. Cheap and non-blocking on the commit hot path — the
// actual bake happens later on a maybeBake tick. A commit for a song in no live
// setlist is a no-op.
func (a *AutoBaker) Notify(songID string) {
	if a == nil {
		return
	}
	live, err := a.svc.LiveSetlistsForSong(songID)
	if err != nil || len(live) == 0 {
		return
	}
	now := a.now().UTC()
	a.mu.Lock()
	for _, sl := range live {
		a.pending[sl.ID] = pendingBake{bandID: sl.BandID, actorID: sl.LiveBy, lastSeen: now}
	}
	a.mu.Unlock()
}

// maybeBake bakes every pending setlist whose quiet period has elapsed as of now and
// that has no bake already in flight. Called on a ticker in production and directly
// from tests. Bakes run in their own goroutines so one slow bake doesn't stall the
// tick; the inflight guard prevents doubling a setlist up.
func (a *AutoBaker) maybeBake(now time.Time) {
	now = now.UTC()
	a.mu.Lock()
	var ready []struct {
		setlistID string
		p         pendingBake
	}
	for id, p := range a.pending {
		if a.inflight[id] || now.Sub(p.lastSeen) < a.win {
			continue
		}
		ready = append(ready, struct {
			setlistID string
			p         pendingBake
		}{id, p})
		delete(a.pending, id)
		a.inflight[id] = true
	}
	a.mu.Unlock()

	for _, r := range ready {
		go a.runBake(r.setlistID, r.p)
	}
}

func (a *AutoBaker) runBake(setlistID string, p pendingBake) {
	defer func() {
		a.mu.Lock()
		delete(a.inflight, setlistID)
		a.mu.Unlock()
	}()
	actor, err := a.svc.UserByID(p.actorID)
	if err != nil {
		return // the enabling admin vanished; skip (a later commit re-arms)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	_ = a.bake(ctx, p.bandID, setlistID, actor) // best-effort; a failure re-arms on the next commit
}

// Run drives maybeBake on a ticker until ctx is cancelled — the production loop.
// Tests skip this and call maybeBake directly with a fake clock.
func (a *AutoBaker) Run(ctx context.Context, tick time.Duration) {
	if tick <= 0 {
		tick = time.Second
	}
	t := time.NewTicker(tick)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			a.maybeBake(a.now())
		}
	}
}

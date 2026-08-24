package bake

import (
	"sync"
	"time"
)

// Bake progress (T96) is a SIDE CHANNEL: the `POST …/bake` contract is unchanged, but
// while a bake runs it publishes "song N of M" to an in-process registry that a new
// GET endpoint exposes. Keyed by a per-bake id — NEVER the setlist id: B08/B09 proved
// concurrent bakes of the same setlist are legal (they mint distinct revs), so the
// setlist id is precisely the key already known not to be unique.

// BakeState is the terminal-or-not status of a bake.
type BakeState string

const (
	BakeRunning   BakeState = "running"
	BakeSucceeded BakeState = "succeeded"
	BakeFailed    BakeState = "failed"
)

// BakeProgress is a point-in-time snapshot the progress endpoint returns. `Done`
// advances 1..Total as each song is worked; `Song` names the song being baked (or the
// last one, on a terminal state); `Error` is set only on BakeFailed and names the
// offending song — the whole value of the readout when a bake dies before a gig.
type BakeProgress struct {
	State BakeState `json:"state"`
	Done  int       `json:"done"`
	Total int       `json:"total"`
	Song  string    `json:"song,omitempty"`
	Error string    `json:"error,omitempty"`
}

// progressEntry is a registry slot: the snapshot plus the band/setlist it belongs to
// (so a read scoped to a different band can't observe it — a cross-band leak) plus an
// expiry (the bound: see progressRegistry).
type progressEntry struct {
	prog      BakeProgress
	bandID    string
	setlistID string
	expires   time.Time
}

// progressRegistry maps bake id → latest progress. In-memory and BOUNDED: every write
// stamps an expiry (generous while running so an active bake never evicts itself, short
// once terminal so a finished bake lingers only long enough for a poller to read the
// ending) and expired entries are swept lazily on each access. No background goroutine,
// and nothing accumulates on a long-lived server.
type progressRegistry struct {
	mu          sync.Mutex
	entries     map[string]progressEntry
	now         func() time.Time
	runningTTL  time.Duration
	terminalTTL time.Duration
}

func newProgressRegistry(now func() time.Time) *progressRegistry {
	if now == nil {
		now = time.Now
	}
	return &progressRegistry{
		entries:     map[string]progressEntry{},
		now:         now,
		runningTTL:  30 * time.Minute, // an entry idle this long is presumed abandoned (client + bake both gone)
		terminalTTL: 5 * time.Minute,  // a finished bake stays readable this long, then is evicted
	}
}

// set publishes the latest progress for a bake. A terminal state gets the short TTL; a
// running one is kept alive for runningTTL and refreshed on every update.
func (r *progressRegistry) set(id, bandID, setlistID string, p BakeProgress) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	ttl := r.runningTTL
	if p.State != BakeRunning {
		ttl = r.terminalTTL
	}
	r.entries[id] = progressEntry{prog: p, bandID: bandID, setlistID: setlistID, expires: r.now().Add(ttl)}
}

// get returns a bake's progress, but only to a caller scoped to the SAME band+setlist.
// An unknown id, an expired id, or a band/setlist mismatch all report ok=false — the
// handler maps that to 404, so "no such bake" and "not yours" are indistinguishable to
// a cross-band caller (no existence oracle).
func (r *progressRegistry) get(id, bandID, setlistID string) (BakeProgress, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	e, ok := r.entries[id]
	if !ok || e.bandID != bandID || e.setlistID != setlistID {
		return BakeProgress{}, false
	}
	return e.prog, true
}

// sweepLocked drops every expired entry. Caller holds r.mu.
func (r *progressRegistry) sweepLocked() {
	now := r.now()
	for id, e := range r.entries {
		if now.After(e.expires) {
			delete(r.entries, id)
		}
	}
}

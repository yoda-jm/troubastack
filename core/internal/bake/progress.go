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
	// Warnings carries T60's per-song transpose warnings on the TERMINAL succeeded record (T103):
	// the async bake POST returns before the bake finishes, so these no longer ride the POST body —
	// the client reads them here when the poll reaches `succeeded`.
	Warnings []string `json:"warnings,omitempty"`
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

// claim reserves a caller-supplied bake id (T99/B) IF no LIVE entry already holds it,
// seeding an initial running slot scoped to this band+setlist and returning true. A
// supplied id that collides with an in-flight bake — even in another band — is refused,
// so a client cannot blank another bake's readout by replaying its id; the caller mints
// a server-side id instead. (Expired entries are swept first, so a stale id is reusable.)
func (r *progressRegistry) claim(id, bandID, setlistID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	if _, exists := r.entries[id]; exists {
		return false
	}
	r.entries[id] = progressEntry{
		prog:      BakeProgress{State: BakeRunning},
		bandID:    bandID,
		setlistID: setlistID,
		expires:   r.now().Add(r.runningTTL),
	}
	return true
}

// validBakeID accepts only a canonical 8-4-4-4-12 hex UUID (what crypto.randomUUID mints,
// 36 chars). An arbitrary client string must never become a registry map key — that is
// unbounded key growth + log-injection surface for free (T99, Fable's condition 1).
func validBakeID(s string) bool {
	if len(s) != 36 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		isHex := (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isHex {
			return false
		}
	}
	return true
}

// setWarnings attaches T60 warnings to an existing (terminal) record, scoped to band+setlist. A no-op
// if the id is unknown/expired or scoped elsewhere — warnings are decoration, never worth a failure.
func (r *progressRegistry) setWarnings(id, bandID, setlistID string, warnings []string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sweepLocked()
	e, ok := r.entries[id]
	if !ok || e.bandID != bandID || e.setlistID != setlistID {
		return
	}
	e.prog.Warnings = warnings
	r.entries[id] = e
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

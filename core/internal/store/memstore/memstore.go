// Package memstore is an in-memory, history-aware Store — a full Collector kept in
// RAM. Ephemeral (state is lost on restart), so its purpose is TESTS and throwaway
// runs: it exercises the SAME history / revert / pin / GC contract as git/file/pg
// without disk or external services, which makes that logic fast to unit-test. It
// is the reference implementation of the store contract.
//
// Boundary: imports store + domain + stdlib only.
package memstore

import (
	"sort"
	"sync"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// Mem holds per-song history in memory.
type Mem struct {
	mu    sync.Mutex
	songs map[string]*songState
}

type songState struct {
	log       []domain.Mutation // ordered accepted mutations
	revisions []domain.Revision // linear history; revisions[i].Number == i+1
	revLogLen []int             // revLogLen[i] = len(log) when revision i+1 was appended
	pins      map[string]domain.Pin
	seqSeen   map[uint64]bool // idempotency by accepted seq (I2)
}

// New returns an in-memory (history-aware) Store.
func New() store.Store { return &Mem{songs: map[string]*songState{}} }

// Capability assertion: Mem is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*Mem)(nil)

func (m *Mem) state(songID string) *songState {
	s := m.songs[songID]
	if s == nil {
		s = &songState{pins: map[string]domain.Pin{}, seqSeen: map[uint64]bool{}}
		m.songs[songID] = s
	}
	return s
}

// Head returns the materialized current state (revision = number of revisions).
func (m *Mem) Head(songID string) (domain.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.songs[songID]
	if s == nil {
		return domain.Snapshot{}, nil
	}
	return store.Fold(s.log, uint64(len(s.revisions))), nil
}

// Apply records one already-ordered, already-reconciled action (I2 idempotent by seq).
func (m *Mem) Apply(songID string, mut domain.Mutation) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state(songID)
	if mut.Seq != 0 && s.seqSeen[mut.Seq] {
		return nil // idempotent replay
	}
	s.log = append(s.log, mut.Clone())
	if mut.Seq != 0 {
		s.seqSeen[mut.Seq] = true
	}
	return nil
}

// SnapshotAt folds the log prefix captured when revision N was appended (I4).
func (m *Mem) SnapshotAt(songID string, revision uint64) (domain.Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.songs[songID]
	if s == nil || revision == 0 || revision > uint64(len(s.revisions)) {
		return domain.Snapshot{}, store.ErrNotFound
	}
	prefix := s.revLogLen[revision-1]
	return store.Fold(s.log[:prefix], revision), nil
}

// AppendRevision records a new head; returns the assigned revision number (I4).
func (m *Mem) AppendRevision(songID string, r domain.Revision) (uint64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state(songID)
	r.Number = uint64(len(s.revisions)) + 1
	if r.Number > 1 {
		r.Parent = r.Number - 1
	}
	s.revisions = append(s.revisions, r)
	s.revLogLen = append(s.revLogLen, len(s.log))
	return r.Number, nil
}

// Revisions returns the linear history oldest→newest.
func (m *Mem) Revisions(songID string) ([]domain.Revision, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.songs[songID]
	if s == nil {
		return nil, nil
	}
	out := make([]domain.Revision, len(s.revisions))
	copy(out, s.revisions)
	return out, nil
}

// MovePin points a named pin at an existing revision (I4); never deletes its target.
func (m *Mem) MovePin(songID, pinName string, revision uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.state(songID)
	if revision == 0 || revision > uint64(len(s.revisions)) {
		return store.ErrNotFound
	}
	s.pins[pinName] = domain.Pin{SongID: songID, Name: pinName, RevisionNumber: revision}
	return nil
}

// Pins lists the pins for a song.
func (m *Mem) Pins(songID string) ([]domain.Pin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := m.songs[songID]
	if s == nil {
		return nil, nil
	}
	out := make([]domain.Pin, 0, len(s.pins))
	for _, p := range s.pins {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Collect is the in-RAM reachability sweep (I7). For the v1 spike the keep-all tier
// is a no-op; reachability-prune is honored by never dropping pinned/head revisions.
// History is append-only and small (grows with strokes, not files), so this is a
// safe no-op that still keeps every reference reconstructable.
func (m *Mem) Collect(_ store.RootSet, _ store.RetentionPolicy) error {
	// No-op: keep-all is the default and reference-safe. A real prune would drop
	// log entries superseded before the oldest kept revision; omitted for the spike.
	return nil
}

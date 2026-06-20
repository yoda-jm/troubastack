// Package memstore is an in-memory, history-aware Store — a full Collector kept in
// RAM. Ephemeral (state is lost on restart), so its purpose is TESTS and throwaway
// runs: it exercises the SAME history / revert / pin / GC contract as git/file/pg
// without disk or external services, which makes that logic fast to unit-test.
//
// Boundary: imports store + domain + stdlib only.
package memstore

import "troubastack/core/internal/store"

// Mem holds per-song history in memory. TODO: maps keyed by songID → revisions/pins.
type Mem struct{}

// New returns an in-memory (history-aware) Store.
func New() store.Store { return &Mem{} }

// Capability assertion: Mem is a full Collector (⊃ HistoryAware ⊃ Store) — so tests
// can drive the whole history/GC contract against RAM.
var _ store.Collector = (*Mem)(nil)

func (*Mem) Head(string) (any, error)               { return nil, store.ErrTODO }
func (*Mem) Apply(string, any) error                { return store.ErrTODO }
func (*Mem) SnapshotAt(string, string) (any, error) { return nil, store.ErrTODO }
func (*Mem) AppendRevision(string, any) error       { return store.ErrTODO }
func (*Mem) MovePin(string, string, string) error   { return store.ErrTODO }

// Collect prunes unreachable in-memory history per policy (I7) — exercised by tests.
func (*Mem) Collect(store.RootSet, store.RetentionPolicy) error { return store.ErrTODO }

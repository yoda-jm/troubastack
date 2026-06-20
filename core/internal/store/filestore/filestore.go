// Package filestore persists to a plain append-only file tree — the simplest
// zero-infra local-dev backend (ADR 0002). For versioned history with native
// revert + reachability GC, see the gitstore sibling.
//
// Layout (mirrors the append-only domain, I4):
//
//	<dir>/songs/<songID>.log              append-only JSONL; fold to materialize
//	                                      state (I2 idempotent, I5 tombstones,
//	                                      I7 the log IS the history → GC = compaction)
//	<dir>/blobs/<sha256>                  content-addressed PDF/image assets (once)
//	<dir>/bundles/<concertID>/<rev>/...   baked concert outputs (I12)
//
// Human-inspectable and diffable. Implements store.Store.
// Boundary: imports store + domain + stdlib only.
package filestore

import "troubastack/core/internal/store"

// File is a file-tree-backed Store rooted at dir.
type File struct{ dir string }

// New returns a file-backed Store rooted at dir (created on first write).
func New(dir string) store.Store { return &File{dir: dir} }

// Capability assertion: File is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*File)(nil)

// File is Store + HistoryAware + Collector (it retains history).
func (*File) Head(string) (any, error)               { return nil, store.ErrTODO }
func (*File) Apply(string, any) error                { return store.ErrTODO }
func (*File) SnapshotAt(string, string) (any, error) { return nil, store.ErrTODO }
func (*File) AppendRevision(string, any) error       { return store.ErrTODO }
func (*File) MovePin(string, string, string) error   { return store.ErrTODO }

// Collect compacts the append log and drops unreferenced blobs (I7).
func (*File) Collect(store.RootSet, store.RetentionPolicy) error { return store.ErrTODO }

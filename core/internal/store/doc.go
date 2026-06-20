// Package store is persistence plus the retention/GC policy ladder.
//
// Invariants served: I4 (append-only linear writes: append revision, move pin),
// I5 (tombstones persist and win), I7 (GC NEVER breaks a reference — anything
// reachable from a pin, head, or retained anchor must always be reconstructable;
// GC is an optional retention policy layered on top of correctness, never part
// of it; default is keep full history).
//
// Boundary:
//   - MAY import: domain, proto-generated types, stdlib, and (later) a DB driver.
//   - MUST NOT import: sync, session, bake, httpapi. Store knows how to persist
//     truth; it does not know about transport, auth, or baking.
package store

import "errors"

// ErrTODO marks an unimplemented store operation in this scaffold.
var ErrTODO = errors.New("store: not implemented (scaffold)")

// Kind selects a Store backend. Persistence is SWAPPABLE (ADR 0002,
// docs/design/06-tech-stack.md): file/memory for local dev with zero infra,
// Postgres for production. The composition root (cmd/troubacore) picks one; the
// rest of core depends only on the Store interface, never a concrete backend (I14).
type Kind string

const (
	KindMemory   Kind = "mem"  // ephemeral; tests and throwaway runs
	KindFile     Kind = "file" // plain append-only files on disk; DEFAULT for local dev
	KindGit      Kind = "git"  // go-git repo: versioned, content-addressed, native revert + reachability GC
	KindPostgres Kind = "pg"   // production
)

// DefaultKind is the local-dev default: file-backed, no server required.
const DefaultKind = KindFile

// Three layered capabilities (Store ⊂ HistoryAware ⊂ Collector). Concrete backends
// live in subpackages (memstore, filestore, gitstore, pgstore) so this package
// carries no backend dependency. Callers discover the richer capabilities by type
// assertion (the Go optional-interface idiom).

// Store is the MINIMAL contract EVERY backend implements: apply one completed action
// to the materialized HEAD, and read HEAD. No history is implied — a consumer that
// needs only current state depends on this narrow view, and a hypothetical
// snapshot-only backend would implement only this. (Today every backend is a full
// Collector — including the in-memory mem; the narrow interfaces exist for consumers,
// per interface segregation.)
type Store interface {
	// Head returns the current materialized state of a song.
	Head(songID string) (any, error)
	// Apply applies one completed action to HEAD (I2 idempotent by uuid; I5 LWW).
	Apply(songID string, mutation any) error
}

// HistoryAware is the capability of backends that RETAIN revision history (every
// current backend, including the in-memory mem used for tests). It is what makes
// revert, pinning, and audit-from-store possible;
// per-action commits (ADR 0003) land here. (Revert is derived: AppendRevision of
// SnapshotAt(N) — git revert, not reset; I4 — so it is not a separate method.)
type HistoryAware interface {
	Store
	// SnapshotAt materializes a past revision (I4).
	SnapshotAt(songID, revisionID string) (any, error)
	// AppendRevision records a new head — one per completed action (ADR 0003) (I4).
	AppendRevision(songID string, revision any) error
	// MovePin points a named pin at an existing revision (I4); never deletes its target (I7).
	MovePin(songID, pinName, revisionID string) error
}

// Collector is GC, which LIVES ON TOP OF history-aware storage — you can only collect
// history that is retained, so Collector embeds HistoryAware. The retention POLICY and
// ROOT SET are computed ONCE above the backend (uniform); each backend EXECUTES the
// sweep natively:
//
//	git  -> `git gc` over refs (the root set IS the refs)     — near-free
//	file -> compact the append log + drop unreferenced blobs  — implement reachability
//	pg   -> delete/archive rows unreachable from roots         — relational mark-sweep
//	mem  -> in-memory history (ephemeral) — full Collector; the fast substrate for tests
//
// Cross-layer references are honored via the shared RootSet so no backend prunes what
// another still references (I7 is cross-layer). GC is NEVER part of correctness.
type Collector interface {
	HistoryAware
	// Collect prunes unreachable history per policy, never dropping anything reachable
	// from roots (I7). Safe to no-op.
	Collect(roots RootSet, policy RetentionPolicy) error
}

// RootSet is the global set of references GC must preserve, gathered across ALL layers
// (setlist pins, song heads, bake source revisions, retained milestone/audit anchors).
// TODO: concrete shape.
type RootSet struct{}

// RetentionPolicy selects the tier (keep-all | reachability-prune | smart-squash) plus
// any per-layer retention FLOORS (e.g. audit kept longer for compliance). TODO.
type RetentionPolicy struct{}

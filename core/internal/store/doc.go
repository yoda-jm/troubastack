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
//
// The store is a PASSIVE history sink (design/07): the engine above it has already
// serialized, resolved LWW, and assigned seq. A backend NEVER does LWW, never
// merges, never sees a race. It records the already-ordered stream and folds it
// back into materialized state.
package store

import (
	"errors"

	"troubastack/core/internal/domain"
)

// ErrNotFound is returned for an unknown song or revision.
var ErrNotFound = errors.New("store: not found")

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
// needs only current state depends on this narrow view.
//
// Apply records ONE already-ordered, already-reconciled action (the engine has done
// LWW/seq already). It is idempotent by the action's identity (re-applying the same
// accepted seq is a no-op) so a retried WAL drain never double-writes (I2).
type Store interface {
	// Head returns the current materialized state of a song. A song with no history
	// yields an empty Snapshot (revision 0), not an error.
	Head(songID string) (domain.Snapshot, error)
	// Apply folds one completed action into HEAD and appends it to the durable log.
	Apply(songID string, m domain.Mutation) error
}

// HistoryAware is the capability of backends that RETAIN revision history (every
// current backend, including the in-memory mem). It is what makes revert, pinning,
// and audit-from-store possible; per-action commits (ADR 0003) land here.
type HistoryAware interface {
	Store
	// SnapshotAt materializes a past revision (I4). ErrNotFound if unknown.
	SnapshotAt(songID string, revision uint64) (domain.Snapshot, error)
	// AppendRevision records a new head — one per completed action (ADR 0003) (I4).
	// Returns the assigned revision number.
	AppendRevision(songID string, r domain.Revision) (uint64, error)
	// Revisions returns the linear history oldest→newest (audit/browse).
	Revisions(songID string) ([]domain.Revision, error)
	// MovePin points a named pin at an existing revision (I4); never deletes its
	// target (I7).
	MovePin(songID, pinName string, revision uint64) error
	// Pins lists the pins for a song (a GC root contribution).
	Pins(songID string) ([]domain.Pin, error)
}

// Collector is GC, which LIVES ON TOP OF history-aware storage. The retention POLICY
// and ROOT SET are computed ONCE above the backend; each backend EXECUTES the sweep
// natively. GC is NEVER part of correctness — Collect is always safe to no-op.
type Collector interface {
	HistoryAware
	// Collect prunes unreachable history per policy, never dropping anything reachable
	// from roots (I7). Safe to no-op.
	Collect(roots RootSet, policy RetentionPolicy) error
}

// Tier selects a retention level on the GC ladder (design/01, I7).
type Tier int

const (
	// TierKeepAll keeps every revision (the default; full audit timeline).
	TierKeepAll Tier = iota
	// TierReachabilityPrune keeps only {pins} ∪ {head}.
	TierReachabilityPrune
)

// RootSet is the global set of references GC must preserve, gathered across ALL
// layers (setlist pins, song heads, bake source revisions, retained anchors). For
// the v1 spike a backend also folds in its own pins + head, so an empty RootSet is
// safe (never over-prunes).
type RootSet struct {
	// KeepRevisions, keyed by songID, are extra revision numbers to preserve.
	KeepRevisions map[string][]uint64
}

// RetentionPolicy selects the tier plus any per-layer retention floors.
type RetentionPolicy struct {
	Tier Tier
}

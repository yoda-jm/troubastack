// Package domain holds the pure data model and resolution logic: objects with
// client-generated UUIDs, linear append-only revision history, setlist pins,
// per-object last-write-wins, and terminal tombstones.
//
// Invariants served: I2 (idempotent objects by UUID), I4 (linear append-only
// history; revert = new appended head), I5 (LWW + tombstone-wins), I7 (the
// reference set GC must preserve).
//
// Boundary:
//   - MAY import: proto-generated types (troubastack/core/internal/gen/...) and stdlib.
//   - MUST NOT import: store, sync, session, bake, httpapi (no I/O, no transport,
//     no UI). This package is pure types + logic so it stays trivially testable.
//
// Single source of truth (I1): wire/domain types are generated from proto/.
// The structs below are thin DOMAIN WRAPPERS over those generated types — they
// add server-side semantics (resolution, validation), never a parallel schema.
package domain

// Object is an annotation identified by a client-generated UUID (I2).
//
// TODO: wrap the generated proto Object; apply-by-UUID must be idempotent
// (re-receiving the same UUID is a no-op or in-place replace, never a duplicate).
type Object struct {
	// TODO: UUID, page, geometry ([0,1] normalized — I3), style, version, deleted.
}

// Revision is one entry in a song's single linear history (I4).
//
// TODO: wrap the generated proto Revision. The only writes are "append a
// revision" and "move a pin"; there is no branch/merge. "Revert to N" appends a
// new head equal to N.
type Revision struct {
	// TODO: id, parentID (linear), author, createdAt, payload ref.
}

// Pin is a named reference onto the revision line (e.g. a setlist entry, head).
// A pinned revision is part of the live reference set and is immortal to GC (I7).
//
// TODO: wrap the generated proto Pin; moving a pin is one of the two legal writes.
type Pin struct {
	// TODO: name, revisionID.
}

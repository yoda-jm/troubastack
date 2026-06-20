// Package sync is the realtime WebSocket hub AND the apply engine — the live
// authority that sits ABOVE the store.
//
// THE BORDER (docs/design/07): the engine owns the in-memory HEAD, per-song
// serialization (single writer), LWW (I5), seq assignment, optimistic echo, and the
// durable ordered log (WAL). The store is a PASSIVE history sink — the engine hydrates
// HEAD from it at start and persists the already-ordered, already-reconciled stream to
// it (async for slow backends). Concurrency stops here; the store never sees a race.
//
// Invariants served: I6 (server is authoritative; clients reconcile to echoes),
// I2 (apply by UUID is idempotent — your own echo and a peer's object take the
// SAME ingest path, never a separate "remote" path). See
// docs/design/02-sync-protocol.md.
//
// Boundary:
//   - MAY import: domain, store, session, proto-generated wire types, stdlib.
//   - MUST NOT import: httpapi, bake, or any client (web/app). The hub accepts
//     objects and broadcasts echoes; it holds no UI reactivity (I6 — keep
//     authority calm, keep reactive state in Studio).
package sync

// Hub fans realtime traffic across clients and drives one apply engine (a serial
// owning goroutine/actor) PER SONG that holds HEAD and the WAL.
//
// TODO: rooms keyed by songID; register/unregister. On an inbound mutation the song's
// actor → apply to HEAD + LWW (I5) + assign seq → WAL append (durable) → echo to the
// room (I6) → RELEASE; then persist to the store ASYNC, draining the WAL (one action
// per revision). Apply is idempotent by UUID (I2). A mutation for a tombstoned UUID is
// rejected ("deleted-remotely") so the client rolls back (I5).
type Hub struct {
	// TODO: per-song actors (HEAD + WAL), rooms map, store handle, session checker.
}

// New returns a placeholder Hub. TODO: wire store + session dependencies.
func New() *Hub { return &Hub{} }

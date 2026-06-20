// Package pgstore is the production Postgres-backed Store (ADR 0002), for scale
// and relational queries. Implements store.Store.
//
// Boundary: imports store + domain + a pgx/pq driver (later) + stdlib.
package pgstore

import "troubastack/core/internal/store"

// PG is a Postgres-backed Store. TODO: hold the db handle (pgx pool).
type PG struct{}

// New connects using databaseURL. TODO: wire pgx; stub keeps core building offline.
func New(databaseURL string) (store.Store, error) { _ = databaseURL; return &PG{}, nil }

// Capability assertion: PG is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*PG)(nil)

// PG is Store + HistoryAware + Collector.
func (*PG) Head(string) (any, error)               { return nil, store.ErrTODO }
func (*PG) Apply(string, any) error                { return store.ErrTODO }
func (*PG) SnapshotAt(string, string) (any, error) { return nil, store.ErrTODO }
func (*PG) AppendRevision(string, any) error       { return store.ErrTODO }
func (*PG) MovePin(string, string, string) error   { return store.ErrTODO }

// Collect deletes/archives rows unreachable from roots (I7).
func (*PG) Collect(store.RootSet, store.RetentionPolicy) error { return store.ErrTODO }

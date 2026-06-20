// Package pgstore is the production Postgres-backed Store (ADR 0002), for scale
// and relational queries. Implements the store contract.
//
// NOT YET IMPLEMENTED: this remains a compile-time stub (no pgx wiring) so core
// builds offline. Every method returns ErrUnimplemented; the mem/file/git backends
// are the working ones for the v1 spike. The signatures track the store contract so
// the composition root keeps building.
//
// Boundary: imports store + domain + a pgx/pq driver (later) + stdlib.
package pgstore

import (
	"errors"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/store"
)

// ErrUnimplemented marks the not-yet-built Postgres backend.
var ErrUnimplemented = errors.New("pgstore: not implemented")

// PG is a Postgres-backed Store. TODO: hold the db handle (pgx pool).
type PG struct{}

// New connects using databaseURL. TODO: wire pgx; stub keeps core building offline.
func New(databaseURL string) (store.Store, error) { _ = databaseURL; return &PG{}, nil }

// Capability assertion: PG is a full Collector (⊃ HistoryAware ⊃ Store).
var _ store.Collector = (*PG)(nil)

func (*PG) Head(string) (domain.Snapshot, error) { return domain.Snapshot{}, ErrUnimplemented }
func (*PG) Apply(string, domain.Mutation) error  { return ErrUnimplemented }
func (*PG) SnapshotAt(string, uint64) (domain.Snapshot, error) {
	return domain.Snapshot{}, ErrUnimplemented
}
func (*PG) AppendRevision(string, domain.Revision) (uint64, error) { return 0, ErrUnimplemented }
func (*PG) Revisions(string) ([]domain.Revision, error)            { return nil, ErrUnimplemented }
func (*PG) MovePin(string, string, uint64) error                   { return ErrUnimplemented }
func (*PG) Pins(string) ([]domain.Pin, error)                      { return nil, ErrUnimplemented }

// Collect deletes/archives rows unreachable from roots (I7).
func (*PG) Collect(store.RootSet, store.RetentionPolicy) error { return ErrUnimplemented }

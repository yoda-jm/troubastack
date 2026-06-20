package engine

import "errors"

// ErrDeletedRemotely is returned when a mutation targets a tombstoned UUID. Delete is
// terminal (I5): only an explicit Restore revives a UUID. The client rolls back.
var ErrDeletedRemotely = errors.New("engine: deleted-remotely (object is tombstoned)")

// ErrStaleVersion is returned when a mutation's BaseVersion is behind the current
// object version — a stale optimistic edit lost the LWW race (I5).
var ErrStaleVersion = errors.New("engine: stale-version (lost the LWW race)")

// ErrUnknownObject is returned when an edit/move targets a UUID that was never created.
var ErrUnknownObject = errors.New("engine: unknown object")

// ErrInvalidMutation is returned for a malformed mutation (e.g. Create without an
// Object, or a Layer op without a Layer).
var ErrInvalidMutation = errors.New("engine: invalid mutation")

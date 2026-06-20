// Package session is authentication, group membership, and role resolution.
//
// Roles: ADMIN, PERFORMER, CONDUCTOR. Roles gate what the rest of core will do
// — e.g. only an ADMIN may trigger a bake (I11). It supports I6 by establishing
// the identity behind every authoritative write.
//
// Boundary:
//   - MAY import: domain, store, proto-generated types, stdlib.
//   - MUST NOT import: sync, bake, httpapi, or any client. Session answers "who
//     are you and what may you do"; it does not transport or render anything.
package session

// Role enumerates the access tiers.
type Role int

const (
	// RoleUnknown is the zero value; treat as unauthenticated.
	RoleUnknown Role = iota
	RoleAdmin
	RolePerformer
	RoleConductor
)

// Manager resolves auth tokens to a member + role and checks group membership.
//
// TODO: token verification, group lookup, role assignment.
type Manager struct {
	// TODO: store handle, token/keys.
}

// New returns a placeholder Manager. TODO: wire auth backend.
func New() *Manager { return &Manager{} }

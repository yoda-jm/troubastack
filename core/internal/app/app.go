// Package app is the "normal web" relational domain: users, sessions, bands,
// memberships, invites, and song metadata. It is DELIBERATELY separate from the
// per-song annotation engine/store (internal/{engine,store,domain}) — those hold
// the append-only annotation history; this package holds the relational entities
// that frame it (who, which band, which songs exist).
//
// Membership policy (docs/risks.md R8): anyone may create a band; you add users
// by an EXPLICIT identifier (username | email | uuid); users are NOT discoverable
// (you cannot browse others); an invite is a CONSENT request — the invitee must
// accept before becoming a member.
//
// Boundary (I14):
//   - MAY import: stdlib + golang.org/x/crypto/bcrypt.
//   - MUST NOT import: sync, bake, httpapi, the annotation store, or any client.
//     This package answers "who are you / which band / which songs"; transport
//     (httpapi) and the annotation engine sit above it.
package app

import (
	"errors"
	"time"
)

// Sentinel errors returned by the Repo and the service layer. httpapi maps these
// to status codes.
var (
	ErrNotFound       = errors.New("app: not found")
	ErrConflict       = errors.New("app: conflict")      // unique constraint (username/email)
	ErrForbidden      = errors.New("app: forbidden")     // authenticated but not allowed
	ErrUnauthorized   = errors.New("app: unauthorized")  // no/invalid session
	ErrInvalidInput   = errors.New("app: invalid input") // 400-class
	ErrInviteResolved = errors.New("app: invite already resolved")
)

// Role is a member's role within a band. The band owner is always admin.
type Role string

const (
	RoleAdmin     Role = "admin"
	RoleConductor Role = "conductor"
	RoleMember    Role = "member"
)

// ValidRole reports whether r is a known role.
func ValidRole(r Role) bool {
	switch r {
	case RoleAdmin, RoleConductor, RoleMember:
		return true
	}
	return false
}

// IdentifierKind is how an invite names its invitee. Users are not discoverable,
// so an invite always carries an EXACT identifier of one of these kinds.
type IdentifierKind string

const (
	KindUsername IdentifierKind = "username"
	KindEmail    IdentifierKind = "email"
	KindUUID     IdentifierKind = "uuid"
)

// ValidIdentifierKind reports whether k is a known identifier kind.
func ValidIdentifierKind(k IdentifierKind) bool {
	switch k {
	case KindUsername, KindEmail, KindUUID:
		return true
	}
	return false
}

// InviteStatus is the lifecycle of an invite.
type InviteStatus string

const (
	InvitePending  InviteStatus = "pending"
	InviteAccepted InviteStatus = "accepted"
	InviteDeclined InviteStatus = "declined"
)

// User is a registered account. PasswordHash is a bcrypt hash; it is NEVER
// serialized to clients (see PublicUser).
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	DisplayName  string    `json:"displayName"`
	Email        string    `json:"email,omitempty"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

// PublicUser is the client-safe projection of a User (no password hash, no email
// of others is leaked beyond what the API intends).
type PublicUser struct {
	ID          string    `json:"id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"displayName"`
	Email       string    `json:"email,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Public returns the client-safe projection.
func (u User) Public() PublicUser {
	return PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		CreatedAt:   u.CreatedAt,
	}
}

// Session is an opaque bearer token bound to a user. The token is stored as the
// map key in the repo; this record carries who it belongs to.
type Session struct {
	Token     string    `json:"-"`
	UserID    string    `json:"-"`
	CreatedAt time.Time `json:"-"`
}

// Band is a group. The owner is always an admin member.
type Band struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	OwnerID   string    `json:"ownerId"`
	CreatedAt time.Time `json:"createdAt"`
}

// Membership ties a user to a band with a role.
type Membership struct {
	BandID    string    `json:"bandId"`
	UserID    string    `json:"userId"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// Invite is a consent request to join a band, addressed to an exact identifier.
// The invitee is not necessarily a known user at invite time (not discoverable),
// so the invite stores the raw identifier + kind and is matched to a user only
// when that user lists/accepts it.
type Invite struct {
	ID             string         `json:"id"`
	BandID         string         `json:"bandId"`
	Identifier     string         `json:"identifier"`
	IdentifierKind IdentifierKind `json:"kind"`
	InvitedBy      string         `json:"invitedBy"`
	Status         InviteStatus   `json:"status"`
	CreatedAt      time.Time      `json:"createdAt"`
}

// Song is band-scoped metadata only. The annotation history lives in the separate
// engine/store keyed by this SongID; here we just record that the song exists.
type Song struct {
	ID        string    `json:"id"`
	BandID    string    `json:"bandId"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// Repo is the swappable persistence contract for the relational domain. It mirrors
// the annotation store's "interface + multiple backends" pattern but is a plain
// CRUD store — no history, no LWW. Implementations must be safe for concurrent use.
type Repo interface {
	// Users.
	CreateUser(u User) error
	GetUser(id string) (User, error)
	GetUserByUsername(username string) (User, error)
	GetUserByEmail(email string) (User, error)

	// Sessions.
	CreateSession(s Session) error
	GetSession(token string) (Session, error)
	DeleteSession(token string) error

	// Bands + memberships.
	CreateBand(b Band) error
	GetBand(id string) (Band, error)
	BandsForUser(userID string) ([]Band, error)
	AddMembership(m Membership) error
	GetMembership(bandID, userID string) (Membership, error)
	MembersOfBand(bandID string) ([]Membership, error)

	// Invites.
	CreateInvite(i Invite) error
	GetInvite(id string) (Invite, error)
	UpdateInvite(i Invite) error
	InvitesForBand(bandID string) ([]Invite, error)
	// PendingInvitesForIdentifiers returns pending invites whose (kind,identifier)
	// matches any of the supplied pairs (the invitee's own username/email/uuid).
	PendingInvitesForIdentifiers(pairs []IdentifierMatch) ([]Invite, error)

	// Songs.
	CreateSong(s Song) error
	GetSong(id string) (Song, error)
	SongsOfBand(bandID string) ([]Song, error)
}

// IdentifierMatch is one (kind, identifier) pair used to resolve an invite to a
// user without making users discoverable.
type IdentifierMatch struct {
	Kind       IdentifierKind
	Identifier string
}

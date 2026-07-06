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

// AvatarKind selects which silhouette avatar a user displays. "" is treated as
// neutral when rendering.
type AvatarKind string

const (
	AvatarMan     AvatarKind = "man"
	AvatarWoman   AvatarKind = "woman"
	AvatarNeutral AvatarKind = "neutral"
)

// ValidAvatarKind reports whether k is an allowed avatar kind ("" allowed).
func ValidAvatarKind(k AvatarKind) bool {
	switch k {
	case "", AvatarMan, AvatarWoman, AvatarNeutral:
		return true
	}
	return false
}

// User is a registered account. PasswordHash is a bcrypt hash; it is NEVER
// serialized to clients (see PublicUser).
type User struct {
	ID           string     `json:"id"`
	Username     string     `json:"username"`
	DisplayName  string     `json:"displayName"`
	Email        string     `json:"email,omitempty"`
	AvatarKind   AvatarKind `json:"avatarKind,omitempty"`
	PasswordHash string     `json:"-"`
	CreatedAt    time.Time  `json:"createdAt"`
}

// PublicUser is the client-safe projection of a User (no password hash, no email
// of others is leaked beyond what the API intends).
type PublicUser struct {
	ID          string     `json:"id"`
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	Email       string     `json:"email,omitempty"`
	AvatarKind  AvatarKind `json:"avatarKind,omitempty"`
	CreatedAt   time.Time  `json:"createdAt"`
}

// Public returns the client-safe projection.
func (u User) Public() PublicUser {
	return PublicUser{
		ID:          u.ID,
		Username:    u.Username,
		DisplayName: u.DisplayName,
		Email:       u.Email,
		AvatarKind:  u.AvatarKind,
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

// PasswordReset is a one-time, admin-issued credential-recovery grant (T21). The
// plaintext token travels out-of-band — a link the admin hands the user in
// person or in the band chat, the same trust model as invite links — and only
// its SHA-256 hash is stored, so a leaked dataset yields no usable tokens.
// Consuming it sets a new password AND invalidates every existing session for
// the user. Single-use (deleted on consume) and expires after PasswordResetTTL.
type PasswordReset struct {
	TokenHash string    `json:"tokenHash"`
	UserID    string    `json:"userId"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
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

// InviteLink is a tokenized, shareable join link for a band. Unlike an Invite
// (addressed to a specific identifier), an InviteLink is an open door: anyone who
// holds the unguessable Token may join the band as Role. Clicking accept IS the
// consent. Links can carry an expiry, a max-use cap, and may be revoked.
type InviteLink struct {
	ID        string     `json:"id"`
	BandID    string     `json:"bandId"`
	Token     string     `json:"token"`
	Role      Role       `json:"role"`
	ExpiresAt *time.Time `json:"expiresAt,omitempty"`
	MaxUses   int        `json:"maxUses"` // 0 = unlimited
	Uses      int        `json:"uses"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	RevokedAt *time.Time `json:"revokedAt,omitempty"`
}

// Song is band-scoped metadata only. The annotation history lives in the separate
// engine/store keyed by this SongID; here we just record that the song exists.
type Song struct {
	ID        string    `json:"id"`
	BandID    string    `json:"bandId"`
	Title     string    `json:"title"`
	Artist    string    `json:"artist,omitempty"`
	Key       string    `json:"key,omitempty"`
	Tempo     int       `json:"tempo,omitempty"` // BPM; 0 = unset
	Tags      []string  `json:"tags,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// SongFile is metadata for a binary file (sheet music PDF / image) attached to a
// song. The bytes themselves live in a content-addressed blob.Store keyed by
// BlobHash; this record is the relational pointer to them. BandID is denormalized
// from the song so download authorization (members-only) needs no song lookup.
type SongFile struct {
	ID           string    `json:"id"`
	SongID       string    `json:"songId"`
	BandID       string    `json:"bandId"`
	Filename     string    `json:"filename"`
	ContentType  string    `json:"contentType"`
	Size         int64     `json:"size"`
	BlobHash     string    `json:"blobHash"`
	DisplayOrder int       `json:"displayOrder"`
	UploadedBy   string    `json:"uploadedBy"`
	CreatedAt    time.Time `json:"createdAt"`
}

// FileSelection is a member's PERSONAL, ordered choice of which of a song's pool
// files to display, in their chosen order. It is keyed by (UserID, SongID) and is
// private to that user — it never affects what another member sees. FileIDs is a
// subset of the song's shared file pool; entries whose file has since left the pool
// are skipped on read (deleted files drop out gracefully).
type FileSelection struct {
	UserID  string   `json:"userId"`
	SongID  string   `json:"songId"`
	FileIDs []string `json:"fileIds"`
}

// Setlist is a band-scoped, ordered program of songs for an event. Items hold the
// ordering and per-performance overrides; the songs themselves live independently.
type Setlist struct {
	ID        string    `json:"id"`
	BandID    string    `json:"bandId"`
	Name      string    `json:"name"`
	EventDate string    `json:"eventDate,omitempty"` // optional ISO yyyy-mm-dd
	Venue     string    `json:"venue,omitempty"`
	Notes     string    `json:"notes,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

// SetlistItem places a song at a position within a setlist, with optional per-item
// overrides (a different key/tempo for this performance) and notes.
type SetlistItem struct {
	ID            string `json:"id"`
	SetlistID     string `json:"setlistId"`
	SongID        string `json:"songId"`
	Position      int    `json:"position"`
	KeyOverride   string `json:"keyOverride,omitempty"`
	TempoOverride int    `json:"tempoOverride,omitempty"`
	Notes         string `json:"notes,omitempty"`
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
	UpdateUser(u User) error

	// Sessions.
	CreateSession(s Session) error
	GetSession(token string) (Session, error)
	DeleteSession(token string) error
	// DeleteSessionsForUser invalidates every session belonging to userID (used
	// when a password reset is consumed). Deleting zero sessions is not an error.
	DeleteSessionsForUser(userID string) error

	// Password resets (T21) — keyed by the token's SHA-256 hash, never the token.
	CreatePasswordReset(pr PasswordReset) error
	GetPasswordReset(tokenHash string) (PasswordReset, error)
	DeletePasswordReset(tokenHash string) error

	// Bands + memberships.
	CreateBand(b Band) error
	GetBand(id string) (Band, error)
	UpdateBand(b Band) error
	DeleteBand(id string) error
	BandsForUser(userID string) ([]Band, error)
	AddMembership(m Membership) error
	GetMembership(bandID, userID string) (Membership, error)
	UpdateMembership(m Membership) error
	DeleteMembership(bandID, userID string) error
	MembersOfBand(bandID string) ([]Membership, error)

	// Invites.
	CreateInvite(i Invite) error
	GetInvite(id string) (Invite, error)
	UpdateInvite(i Invite) error
	DeleteInvite(id string) error
	InvitesForBand(bandID string) ([]Invite, error)
	// PendingInvitesForIdentifiers returns pending invites whose (kind,identifier)
	// matches any of the supplied pairs (the invitee's own username/email/uuid).
	PendingInvitesForIdentifiers(pairs []IdentifierMatch) ([]Invite, error)

	// Invite links (tokenized join links).
	CreateInviteLink(l InviteLink) error
	GetInviteLink(id string) (InviteLink, error)
	GetInviteLinkByToken(token string) (InviteLink, error)
	UpdateInviteLink(l InviteLink) error
	InviteLinksForBand(bandID string) ([]InviteLink, error)

	// Songs.
	CreateSong(s Song) error
	GetSong(id string) (Song, error)
	UpdateSong(s Song) error
	DeleteSong(id string) error
	SongsOfBand(bandID string) ([]Song, error)

	// Song files (metadata only; bytes live in a blob.Store).
	CreateSongFile(f SongFile) error
	GetSongFile(id string) (SongFile, error)
	UpdateSongFile(f SongFile) error
	DeleteSongFile(id string) error
	FilesOfSong(songID string) ([]SongFile, error)
	// FilesWithBlob returns every SongFile pointing at blobHash (used to decide
	// whether a blob is still referenced before dereferencing it on delete).
	FilesWithBlob(blobHash string) ([]SongFile, error)

	// Per-member, per-song file selections (personal, not shared).
	// GetFileSelection returns the caller's saved selection; ErrNotFound if the
	// member never customized this song (the service then falls back to default).
	GetFileSelection(userID, songID string) (FileSelection, error)
	// SetFileSelection stores (creates or replaces) a member's selection.
	SetFileSelection(sel FileSelection) error
	// DeleteFileSelection clears a member's customization (reverts to default).
	// Idempotent: clearing an unset selection is not an error.
	DeleteFileSelection(userID, songID string) error

	// Setlists + items.
	CreateSetlist(sl Setlist) error
	GetSetlist(id string) (Setlist, error)
	UpdateSetlist(sl Setlist) error
	DeleteSetlist(id string) error
	SetlistsOfBand(bandID string) ([]Setlist, error)
	CreateSetlistItem(it SetlistItem) error
	GetSetlistItem(id string) (SetlistItem, error)
	UpdateSetlistItem(it SetlistItem) error
	DeleteSetlistItem(id string) error
	ItemsOfSetlist(setlistID string) ([]SetlistItem, error)
}

// IdentifierMatch is one (kind, identifier) pair used to resolve an invite to a
// user without making users discoverable.
type IdentifierMatch struct {
	Kind       IdentifierKind
	Identifier string
}

package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/chartpdf"
)

// Service is the business layer: it enforces the membership/consent/RBAC policy
// (R8) on top of a Repo. httpapi calls Service; Service calls Repo. Keeping the
// policy here (not in handlers) means every backend and transport shares one
// enforcement path.
type Service struct {
	repo  Repo
	blobs blob.Store       // content-addressed bytes for song files
	now   func() time.Time // injectable clock (tests)
	newID func() string    // injectable id generator (tests)
	// bakeTranspose is the chord-transpose step used by the bake-warning check (D3),
	// injectable so a test can force a runtime failure. Production = chartpdf.Transpose.
	bakeTranspose func(source string, from, to chartpdf.Key) (string, error)
}

// NewService wires a Service over a Repo with production defaults. The blob store
// defaults to in-memory; call WithBlobStore to use a persistent backend.
func NewService(repo Repo) *Service {
	return &Service{repo: repo, blobs: blob.NewMem(), now: time.Now, newID: newUUID, bakeTranspose: chartpdf.Transpose}
}

// WithBlobStore swaps the blob backend (file-backed for persistent deployments)
// and returns the Service for chaining.
func (s *Service) WithBlobStore(b blob.Store) *Service {
	s.blobs = b
	return s
}

// WithClock swaps the injectable clock (tests drive live-mode expiry etc.) and
// returns the Service for chaining. Production uses time.Now via NewService.
func (s *Service) WithClock(now func() time.Time) *Service {
	if now != nil {
		s.now = now
	}
	return s
}

// WithBakeTransposeFunc overrides the bake-warning transpose step (D3) — a test seam to
// force a runtime transform failure. Production uses chartpdf.Transpose via NewService.
func (s *Service) WithBakeTransposeFunc(fn func(source string, from, to chartpdf.Key) (string, error)) *Service {
	if fn != nil {
		s.bakeTranspose = fn
	}
	return s
}

// ---- identity ----

// Register creates a new user. username and password are required; email is
// optional but unique if set. Returns ErrConflict on a taken username/email.
func (s *Service) Register(username, displayName, password, email string) (User, error) {
	username = strings.TrimSpace(username)
	displayName = strings.TrimSpace(displayName)
	email = strings.TrimSpace(strings.ToLower(email))
	if username == "" || password == "" {
		return User{}, fmt.Errorf("%w: username and password are required", ErrInvalidInput)
	}
	if displayName == "" {
		displayName = username
	}
	// Uniqueness pre-checks (the repo is also authoritative on conflict).
	if _, err := s.repo.GetUserByUsername(username); err == nil {
		return User{}, fmt.Errorf("%w: username taken", ErrConflict)
	}
	if email != "" {
		if _, err := s.repo.GetUserByEmail(email); err == nil {
			return User{}, fmt.Errorf("%w: email taken", ErrConflict)
		}
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, err
	}
	u := User{
		ID:           s.newID(),
		Username:     username,
		DisplayName:  displayName,
		Email:        email,
		PasswordHash: string(hash),
		CreatedAt:    s.now().UTC(),
	}
	if err := s.repo.CreateUser(u); err != nil {
		return User{}, err
	}
	return u, nil
}

// Login verifies credentials and, on success, mints a session token. Returns
// ErrUnauthorized for an unknown user OR a wrong password (no oracle on which).
func (s *Service) Login(username, password string) (User, string, error) {
	u, err := s.repo.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		// Run a dummy compare to keep timing roughly uniform.
		_ = bcrypt.CompareHashAndPassword([]byte("$2a$10$invalidinvalidinvalidinvalidinvalidinvalidinvalidinv"), []byte(password))
		return User{}, "", ErrUnauthorized
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password)); err != nil {
		return User{}, "", ErrUnauthorized
	}
	token := newToken()
	if err := s.repo.CreateSession(Session{Token: token, UserID: u.ID, CreatedAt: s.now().UTC()}); err != nil {
		return User{}, "", err
	}
	return u, token, nil
}

// Logout deletes a session token (idempotent — unknown tokens are not an error).
func (s *Service) Logout(token string) error {
	if token == "" {
		return nil
	}
	err := s.repo.DeleteSession(token)
	if err == ErrNotFound {
		return nil
	}
	return err
}

// UserForToken resolves a session cookie to its user. ErrUnauthorized if the
// token is empty/unknown or the user vanished.
func (s *Service) UserForToken(token string) (User, error) {
	if token == "" {
		return User{}, ErrUnauthorized
	}
	sess, err := s.repo.GetSession(token)
	if err != nil {
		return User{}, ErrUnauthorized
	}
	u, err := s.repo.GetUser(sess.UserID)
	if err != nil {
		return User{}, ErrUnauthorized
	}
	return u, nil
}

// ---- profile ----

// minPasswordLen is the minimum password length. Register today enforces only
// non-empty (historical), so this is the shared floor used at registration and
// for ChangePassword's new password.
const minPasswordLen = 1

// ProfilePatch carries optional profile updates. A nil pointer leaves the field
// unchanged.
type ProfilePatch struct {
	DisplayName *string
	Email       *string
	AvatarKind  *AvatarKind
}

// UpdateProfile updates the caller's own displayName, email, and/or avatarKind.
// Email must be unique (keeping your own is allowed); displayName (if supplied)
// must be non-empty; avatarKind (if supplied) must be in the allowed set.
func (s *Service) UpdateProfile(userID string, p ProfilePatch) (User, error) {
	u, err := s.repo.GetUser(userID)
	if err != nil {
		return User{}, ErrNotFound
	}
	if p.DisplayName != nil {
		dn := strings.TrimSpace(*p.DisplayName)
		if dn == "" {
			return User{}, fmt.Errorf("%w: display name cannot be empty", ErrInvalidInput)
		}
		u.DisplayName = dn
	}
	if p.Email != nil {
		email := strings.TrimSpace(strings.ToLower(*p.Email))
		if email != "" {
			if !validEmail(email) {
				return User{}, fmt.Errorf("%w: invalid email", ErrInvalidInput)
			}
			if existing, err := s.repo.GetUserByEmail(email); err == nil && existing.ID != u.ID {
				return User{}, fmt.Errorf("%w: email taken", ErrConflict)
			}
		}
		u.Email = email
	}
	if p.AvatarKind != nil {
		if !ValidAvatarKind(*p.AvatarKind) {
			return User{}, fmt.Errorf("%w: invalid avatar kind", ErrInvalidInput)
		}
		u.AvatarKind = *p.AvatarKind
	}
	if err := s.repo.UpdateUser(u); err != nil {
		return User{}, err
	}
	return u, nil
}

// ChangePassword changes the caller's password after verifying the current one.
// ErrForbidden if currentPassword is wrong; ErrInvalidInput if newPassword is
// shorter than the minimum.
func (s *Service) ChangePassword(userID, currentPassword, newPassword string) error {
	u, err := s.repo.GetUser(userID)
	if err != nil {
		return ErrNotFound
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(currentPassword)); err != nil {
		return ErrForbidden
	}
	if len(newPassword) < minPasswordLen {
		return fmt.Errorf("%w: password too short", ErrInvalidInput)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return s.repo.UpdateUser(u)
}

// passwordResetTTL is how long an admin-issued reset link stays valid (T21).
const passwordResetTTL = 24 * time.Hour

// hashToken returns the hex SHA-256 of a token. Reset tokens are stored hashed
// (like a password) so a leaked dataset can't be used to reset anyone; the
// plaintext lives only in the link the admin hands over.
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IssuePasswordReset mints a one-time reset token for a band member. Admin-only,
// and only for a user who is a member of bandID — an admin cannot reach across
// bands. Returns the PLAINTEXT token; the caller hands it to the user
// out-of-band (only its hash is stored). See PasswordReset.
func (s *Service) IssuePasswordReset(caller User, bandID, targetUserID string) (string, error) {
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return "", err
	}
	// Scope: the target must be a member of THIS band (no cross-band reach).
	if _, err := s.repo.GetMembership(bandID, targetUserID); err != nil {
		return "", ErrNotFound
	}
	return s.mintPasswordReset(targetUserID)
}

// IssuePasswordResetForUser mints a reset token for a user by username with NO
// band scope — the server-operator path (the CLI), covering the "the only admin
// forgot their password" bootstrap. Not reachable over HTTP.
func (s *Service) IssuePasswordResetForUser(username string) (User, string, error) {
	u, err := s.repo.GetUserByUsername(strings.TrimSpace(username))
	if err != nil {
		return User{}, "", ErrNotFound
	}
	tok, err := s.mintPasswordReset(u.ID)
	if err != nil {
		return User{}, "", err
	}
	return u, tok, nil
}

func (s *Service) mintPasswordReset(userID string) (string, error) {
	token := newToken()
	now := s.now().UTC()
	if err := s.repo.CreatePasswordReset(PasswordReset{
		TokenHash: hashToken(token),
		UserID:    userID,
		CreatedAt: now,
		ExpiresAt: now.Add(passwordResetTTL),
	}); err != nil {
		return "", err
	}
	return token, nil
}

// PasswordResetTarget validates a reset token and returns whose reset it is, so
// the "set a new password for <user>" screen can name them. ErrNotFound for an
// unknown token; ErrForbidden if expired. Does NOT consume the token.
func (s *Service) PasswordResetTarget(token string) (User, error) {
	pr, err := s.lookupLiveReset(token)
	if err != nil {
		return User{}, err
	}
	u, err := s.repo.GetUser(pr.UserID)
	if err != nil {
		return User{}, ErrNotFound
	}
	return u, nil
}

// ConsumePasswordReset sets a new password via a valid reset token. It is
// single-use (the token is deleted) and invalidates ALL of the user's existing
// sessions, so a leaked or forgotten session can't outlive the reset — the
// whole point of a recovery. ErrNotFound (unknown token), ErrForbidden
// (expired), or ErrInvalidInput (new password too short).
func (s *Service) ConsumePasswordReset(token, newPassword string) error {
	pr, err := s.lookupLiveReset(token)
	if err != nil {
		return err
	}
	if len(newPassword) < minPasswordLen {
		return fmt.Errorf("%w: password too short", ErrInvalidInput)
	}
	u, err := s.repo.GetUser(pr.UserID)
	if err != nil {
		return ErrNotFound
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	if err := s.repo.UpdateUser(u); err != nil {
		return err
	}
	// Single-use: burn the token so it can't be replayed.
	_ = s.repo.DeletePasswordReset(pr.TokenHash)
	// Invalidate every existing session for this user.
	return s.repo.DeleteSessionsForUser(u.ID)
}

// lookupLiveReset resolves a plaintext token to a non-expired PasswordReset.
// Expired tokens are swept opportunistically. ErrNotFound (unknown) /
// ErrForbidden (expired).
func (s *Service) lookupLiveReset(token string) (PasswordReset, error) {
	pr, err := s.repo.GetPasswordReset(hashToken(token))
	if err != nil {
		return PasswordReset{}, ErrNotFound
	}
	if !s.now().UTC().Before(pr.ExpiresAt) {
		_ = s.repo.DeletePasswordReset(pr.TokenHash)
		return PasswordReset{}, ErrForbidden
	}
	return pr, nil
}

// ---- bands & membership ----

// CreateBand creates a band owned by creator, who becomes an admin member.
func (s *Service) CreateBand(creator User, name string) (Band, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Band{}, fmt.Errorf("%w: band name is required", ErrInvalidInput)
	}
	b := Band{ID: s.newID(), Name: name, OwnerID: creator.ID, CreatedAt: s.now().UTC()}
	if err := s.repo.CreateBand(b); err != nil {
		return Band{}, err
	}
	m := Membership{BandID: b.ID, UserID: creator.ID, Role: RoleAdmin, CreatedAt: s.now().UTC()}
	if err := s.repo.AddMembership(m); err != nil {
		return Band{}, err
	}
	return b, nil
}

// BandsForUser lists bands the user is a member of.
func (s *Service) BandsForUser(u User) ([]Band, error) {
	return s.repo.BandsForUser(u.ID)
}

// GetBand returns the band and the caller's role, enforcing member-only read.
func (s *Service) GetBand(caller User, bandID string) (Band, Role, error) {
	b, err := s.repo.GetBand(bandID)
	if err != nil {
		return Band{}, "", ErrNotFound
	}
	m, err := s.repo.GetMembership(bandID, caller.ID)
	if err != nil {
		return Band{}, "", ErrForbidden
	}
	return b, m.Role, nil
}

// MemberView pairs a member's user projection with their role.
type MemberView struct {
	User PublicUser `json:"user"`
	Role Role       `json:"role"`
	// joinedAt is the membership CreatedAt, used only to sort the list
	// deterministically (oldest first). Unexported so it is not serialized — the
	// API response shape is unchanged.
	joinedAt time.Time
}

// Members lists a band's members (member-only).
func (s *Service) Members(caller User, bandID string) ([]MemberView, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	ms, err := s.repo.MembersOfBand(bandID)
	if err != nil {
		return nil, err
	}
	// Deterministic, role-independent order: oldest join first, then username as a
	// tie-breaker. The repo returns members in map (nondeterministic) order, so a
	// role change (which rewrites a membership) must never reorder this list (#6).
	out := make([]MemberView, 0, len(ms))
	for _, m := range ms {
		u, err := s.repo.GetUser(m.UserID)
		if err != nil {
			continue
		}
		out = append(out, MemberView{User: u.Public(), Role: m.Role, joinedAt: m.CreatedAt})
	}
	sort.Slice(out, func(i, j int) bool {
		if !out[i].joinedAt.Equal(out[j].joinedAt) {
			return out[i].joinedAt.Before(out[j].joinedAt)
		}
		return out[i].User.Username < out[j].User.Username
	})
	return out, nil
}

// requireRole checks the caller holds one of the allowed roles in the band.
func (s *Service) requireRole(bandID, userID string, allowed ...Role) (Membership, error) {
	m, err := s.repo.GetMembership(bandID, userID)
	if err != nil {
		return Membership{}, ErrForbidden
	}
	for _, r := range allowed {
		if m.Role == r {
			return m, nil
		}
	}
	return Membership{}, ErrForbidden
}

// UpdateBand renames a band (admin-only).
func (s *Service) UpdateBand(caller User, bandID, name string) (Band, error) {
	b, err := s.repo.GetBand(bandID)
	if err != nil {
		return Band{}, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Band{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Band{}, fmt.Errorf("%w: band name is required", ErrInvalidInput)
	}
	b.Name = name
	if err := s.repo.UpdateBand(b); err != nil {
		return Band{}, err
	}
	return b, nil
}

// DeleteBand removes a band and cascades its songs (+ their files/blobs) and
// setlists (+ items). Admin-only. Blobs are dereferenced only when no remaining
// SongFile (in any band) points at them.
func (s *Service) DeleteBand(caller User, bandID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	// Songs + their files (with blob dereference).
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return err
	}
	for _, song := range songs {
		if err := s.deleteSongCascade(song); err != nil {
			return err
		}
	}
	// Setlists + their items.
	setlists, err := s.repo.SetlistsOfBand(bandID)
	if err != nil {
		return err
	}
	for _, sl := range setlists {
		if err := s.deleteSetlistCascade(sl.ID); err != nil {
			return err
		}
	}
	// Memberships.
	members, err := s.repo.MembersOfBand(bandID)
	if err != nil {
		return err
	}
	for _, m := range members {
		_ = s.repo.DeleteMembership(bandID, m.UserID)
	}
	// Pending/resolved invites for the band.
	invites, err := s.repo.InvitesForBand(bandID)
	if err != nil {
		return err
	}
	for _, inv := range invites {
		_ = s.repo.DeleteInvite(inv.ID)
	}
	// Invite links for the band: revoke (mark) so dangling tokens stop working.
	links, err := s.repo.InviteLinksForBand(bandID)
	if err != nil {
		return err
	}
	for _, l := range links {
		if l.RevokedAt == nil {
			now := s.now().UTC()
			l.RevokedAt = &now
			_ = s.repo.UpdateInviteLink(l)
		}
	}
	return s.repo.DeleteBand(bandID)
}

// adminCount returns the number of admins in a band.
func (s *Service) adminCount(bandID string) (int, error) {
	members, err := s.repo.MembersOfBand(bandID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, m := range members {
		if m.Role == RoleAdmin {
			n++
		}
	}
	return n, nil
}

// SetMemberRole changes a member's role (admin-only). Refuses to demote the LAST
// admin (would leave the band with no admin) → ErrConflict.
func (s *Service) SetMemberRole(caller User, bandID, targetUserID string, role Role) (Membership, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return Membership{}, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Membership{}, err
	}
	if !ValidRole(role) {
		return Membership{}, fmt.Errorf("%w: invalid role", ErrInvalidInput)
	}
	target, err := s.repo.GetMembership(bandID, targetUserID)
	if err != nil {
		return Membership{}, ErrNotFound
	}
	// Demoting the last admin would orphan the band.
	if target.Role == RoleAdmin && role != RoleAdmin {
		n, err := s.adminCount(bandID)
		if err != nil {
			return Membership{}, err
		}
		if n <= 1 {
			return Membership{}, fmt.Errorf("%w: cannot demote the last admin", ErrConflict)
		}
	}
	target.Role = role
	if err := s.repo.UpdateMembership(target); err != nil {
		return Membership{}, err
	}
	return target, nil
}

// RemoveMember removes another member from a band (admin-only). Refuses to remove
// the LAST admin → ErrConflict.
func (s *Service) RemoveMember(caller User, bandID, targetUserID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	target, err := s.repo.GetMembership(bandID, targetUserID)
	if err != nil {
		return ErrNotFound
	}
	if target.Role == RoleAdmin {
		n, err := s.adminCount(bandID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return fmt.Errorf("%w: cannot remove the last admin", ErrConflict)
		}
	}
	return s.repo.DeleteMembership(bandID, targetUserID)
}

// LeaveBand removes the caller from a band (self-service; any member). Refuses if
// the caller is the LAST admin → ErrConflict (they must promote someone first).
func (s *Service) LeaveBand(caller User, bandID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	m, err := s.repo.GetMembership(bandID, caller.ID)
	if err != nil {
		return ErrForbidden
	}
	if m.Role == RoleAdmin {
		n, err := s.adminCount(bandID)
		if err != nil {
			return err
		}
		if n <= 1 {
			return fmt.Errorf("%w: you are the last admin; promote another admin before leaving", ErrConflict)
		}
	}
	return s.repo.DeleteMembership(bandID, caller.ID)
}

// BandInvites lists a band's PENDING invites (admin-only).
func (s *Service) BandInvites(caller User, bandID string) ([]Invite, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return nil, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return nil, err
	}
	all, err := s.repo.InvitesForBand(bandID)
	if err != nil {
		return nil, err
	}
	out := make([]Invite, 0, len(all))
	for _, inv := range all {
		if inv.Status == InvitePending {
			out = append(out, inv)
		}
	}
	return out, nil
}

// RevokeInvite deletes a band's invite (admin-only).
func (s *Service) RevokeInvite(caller User, bandID, inviteID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	inv, err := s.repo.GetInvite(inviteID)
	if err != nil || inv.BandID != bandID {
		return ErrNotFound
	}
	return s.repo.DeleteInvite(inviteID)
}

// ---- invites (consent-based; users not discoverable) ----

// Invite creates a pending invite to a band (admin-only). The identifier is taken
// at face value — we do NOT look the user up (not discoverable), so an invite to
// an unknown identifier is still a valid pending invite. The invitee accepts only
// if they actually own that identifier.
func (s *Service) Invite(caller User, bandID, identifier string, kind IdentifierKind) (Invite, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return Invite{}, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Invite{}, err
	}
	identifier = normalizeIdentifier(kind, identifier)
	if identifier == "" || !ValidIdentifierKind(kind) {
		return Invite{}, fmt.Errorf("%w: identifier and a valid kind are required", ErrInvalidInput)
	}
	inv := Invite{
		ID:             s.newID(),
		BandID:         bandID,
		Identifier:     identifier,
		IdentifierKind: kind,
		InvitedBy:      caller.ID,
		Status:         InvitePending,
		CreatedAt:      s.now().UTC(),
	}
	if err := s.repo.CreateInvite(inv); err != nil {
		return Invite{}, err
	}
	return inv, nil
}

// PendingInvites returns the caller's pending invites, matched against the caller's
// own username, email, and uuid (never by browsing others).
func (s *Service) PendingInvites(caller User) ([]Invite, error) {
	return s.repo.PendingInvitesForIdentifiers(identifiersOf(caller))
}

// AcceptInvite consents to an invite: the caller must own the invite's identifier,
// the invite must be pending. The caller becomes a member of the band.
func (s *Service) AcceptInvite(caller User, inviteID string) (Membership, error) {
	inv, err := s.repo.GetInvite(inviteID)
	if err != nil {
		return Membership{}, ErrNotFound
	}
	if !inviteMatchesUser(inv, caller) {
		// Don't reveal the invite exists for someone else.
		return Membership{}, ErrNotFound
	}
	if inv.Status != InvitePending {
		return Membership{}, ErrInviteResolved
	}
	// Already a member? Resolve the invite and return the existing membership.
	if existing, err := s.repo.GetMembership(inv.BandID, caller.ID); err == nil {
		inv.Status = InviteAccepted
		_ = s.repo.UpdateInvite(inv)
		return existing, nil
	}
	m := Membership{BandID: inv.BandID, UserID: caller.ID, Role: RoleMember, CreatedAt: s.now().UTC()}
	if err := s.repo.AddMembership(m); err != nil {
		return Membership{}, err
	}
	inv.Status = InviteAccepted
	if err := s.repo.UpdateInvite(inv); err != nil {
		return Membership{}, err
	}
	return m, nil
}

// DeclineInvite refuses an invite (caller must own its identifier; must be pending).
func (s *Service) DeclineInvite(caller User, inviteID string) error {
	inv, err := s.repo.GetInvite(inviteID)
	if err != nil {
		return ErrNotFound
	}
	if !inviteMatchesUser(inv, caller) {
		return ErrNotFound
	}
	if inv.Status != InvitePending {
		return ErrInviteResolved
	}
	inv.Status = InviteDeclined
	return s.repo.UpdateInvite(inv)
}

// ---- invite links (tokenized join links) ----

// CreateInviteLink mints a tokenized join link for a band (admin-only). role is
// restricted to member|conductor (never admin via a link). expiresInHours of 0
// means no expiry; maxUses of 0 means unlimited.
func (s *Service) CreateInviteLink(caller User, bandID string, role Role, expiresInHours, maxUses int) (InviteLink, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return InviteLink{}, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return InviteLink{}, err
	}
	if role == "" {
		role = RoleMember
	}
	if role != RoleMember && role != RoleConductor {
		return InviteLink{}, fmt.Errorf("%w: invite-link role must be member or conductor", ErrInvalidInput)
	}
	if maxUses < 0 {
		return InviteLink{}, fmt.Errorf("%w: maxUses cannot be negative", ErrInvalidInput)
	}
	if expiresInHours < 0 {
		return InviteLink{}, fmt.Errorf("%w: expiresInHours cannot be negative", ErrInvalidInput)
	}
	l := InviteLink{
		ID:        s.newID(),
		BandID:    bandID,
		Token:     newInviteToken(),
		Role:      role,
		MaxUses:   maxUses,
		Uses:      0,
		CreatedBy: caller.ID,
		CreatedAt: s.now().UTC(),
	}
	if expiresInHours > 0 {
		exp := s.now().UTC().Add(time.Duration(expiresInHours) * time.Hour)
		l.ExpiresAt = &exp
	}
	if err := s.repo.CreateInviteLink(l); err != nil {
		return InviteLink{}, err
	}
	return l, nil
}

// InviteLinkValid reports whether a link is currently usable, and if not, a
// machine-readable reason ("expired"|"revoked"|"exhausted").
func (s *Service) InviteLinkValid(l InviteLink) (bool, string) {
	if l.RevokedAt != nil {
		return false, "revoked"
	}
	if l.ExpiresAt != nil && !s.now().UTC().Before(*l.ExpiresAt) {
		return false, "expired"
	}
	if l.MaxUses > 0 && l.Uses >= l.MaxUses {
		return false, "exhausted"
	}
	return true, ""
}

// BandInviteLinks lists a band's invite links (admin-only).
func (s *Service) BandInviteLinks(caller User, bandID string) ([]InviteLink, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return nil, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return nil, err
	}
	return s.repo.InviteLinksForBand(bandID)
}

// RevokeInviteLink revokes a band's invite link (admin-only); idempotent.
func (s *Service) RevokeInviteLink(caller User, bandID, linkID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	l, err := s.repo.GetInviteLink(linkID)
	if err != nil || l.BandID != bandID {
		return ErrNotFound
	}
	if l.RevokedAt == nil {
		now := s.now().UTC()
		l.RevokedAt = &now
		return s.repo.UpdateInviteLink(l)
	}
	return nil
}

// InviteLinkPreview resolves a token to its link + band for the join page. Any
// authenticated user may preview; invalid links still return the band so the page
// can explain why joining is unavailable.
func (s *Service) InviteLinkPreview(token string) (InviteLink, Band, error) {
	l, err := s.repo.GetInviteLinkByToken(token)
	if err != nil {
		return InviteLink{}, Band{}, ErrNotFound
	}
	b, err := s.repo.GetBand(l.BandID)
	if err != nil {
		return InviteLink{}, Band{}, ErrNotFound
	}
	return l, b, nil
}

// AcceptInviteLink joins the caller to the link's band as the link's role. The
// click IS consent. Idempotent: an existing member rejoins without incrementing
// Uses. ErrInviteResolved (410-class) if the link is expired/revoked/exhausted.
func (s *Service) AcceptInviteLink(caller User, token string) (Band, error) {
	l, err := s.repo.GetInviteLinkByToken(token)
	if err != nil {
		return Band{}, ErrNotFound
	}
	b, err := s.repo.GetBand(l.BandID)
	if err != nil {
		return Band{}, ErrNotFound
	}
	// Already a member? Idempotent: do not increment Uses.
	if _, err := s.repo.GetMembership(l.BandID, caller.ID); err == nil {
		return b, nil
	}
	if ok, _ := s.InviteLinkValid(l); !ok {
		return Band{}, ErrInviteResolved
	}
	m := Membership{BandID: l.BandID, UserID: caller.ID, Role: l.Role, CreatedAt: s.now().UTC()}
	if err := s.repo.AddMembership(m); err != nil {
		return Band{}, err
	}
	l.Uses++
	if err := s.repo.UpdateInviteLink(l); err != nil {
		return Band{}, err
	}
	return b, nil
}

// ---- songs ----

// Songs lists a band's songs (member-only).
func (s *Service) Songs(caller User, bandID string) ([]Song, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	return s.repo.SongsOfBand(bandID)
}

// SongForMember resolves a song scoped to a band the caller belongs to. It enforces
// member-only access (ErrForbidden for non-members) and that the song belongs to the
// band (ErrNotFound otherwise). It is the gate the annotation API uses before reading
// or importing a song's annotation layers/objects (which live in the separate engine).
func (s *Service) SongForMember(caller User, bandID, songID string) (Song, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return Song{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return Song{}, ErrNotFound
	}
	return song, nil
}

// CreateSong adds a song to a band (any member may create).
func (s *Service) CreateSong(caller User, bandID, title, artist string) (Song, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return Song{}, err
	}
	title = strings.TrimSpace(title)
	if title == "" {
		return Song{}, fmt.Errorf("%w: song title is required", ErrInvalidInput)
	}
	song := Song{
		ID:        s.newID(),
		BandID:    bandID,
		Title:     title,
		Artist:    strings.TrimSpace(artist),
		CreatedAt: s.now().UTC(),
	}
	if err := s.repo.CreateSong(song); err != nil {
		return Song{}, err
	}
	return song, nil
}

// SongPatch carries optional song-metadata updates. A nil pointer leaves the field
// unchanged; a non-nil pointer sets it (including to an empty/zero value).
type SongPatch struct {
	Title  *string
	Artist *string
	Key    *string
	Tempo  *int
	Meter  *string
	Tags   *[]string
	Notes  *string
}

// UpdateSong patches a song's metadata (any band member). Only supplied fields
// change. Title, if supplied, must be non-empty.
func (s *Service) UpdateSong(caller User, bandID, songID string, p SongPatch) (Song, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return Song{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return Song{}, ErrNotFound
	}
	if p.Title != nil {
		t := strings.TrimSpace(*p.Title)
		if t == "" {
			return Song{}, fmt.Errorf("%w: song title cannot be empty", ErrInvalidInput)
		}
		song.Title = t
	}
	if p.Artist != nil {
		song.Artist = strings.TrimSpace(*p.Artist)
	}
	if p.Key != nil {
		song.Key = strings.TrimSpace(*p.Key)
	}
	if p.Tempo != nil {
		if *p.Tempo < 0 {
			return Song{}, fmt.Errorf("%w: tempo cannot be negative", ErrInvalidInput)
		}
		song.Tempo = *p.Tempo
	}
	if p.Meter != nil {
		// Lenient (T86): store the canonical metre if it parses, else unset — a typo
		// becomes 4/4, it never fails the save.
		song.Meter = NormalizeMeter(*p.Meter)
	}
	if p.Tags != nil {
		tags := make([]string, 0, len(*p.Tags))
		for _, t := range *p.Tags {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
		song.Tags = tags
	}
	if p.Notes != nil {
		song.Notes = *p.Notes
	}
	if err := s.repo.UpdateSong(song); err != nil {
		return Song{}, err
	}
	return song, nil
}

// DeleteSong removes a song and its files (admin-only). Each file's blob is
// dereferenced if unreferenced afterwards.
func (s *Service) DeleteSong(caller User, bandID, songID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return ErrNotFound
	}
	return s.deleteSongCascade(song)
}

// deleteSongCascade deletes a song's files (dereferencing orphaned blobs) and then
// the song record. It does NOT check permissions — callers must.
func (s *Service) deleteSongCascade(song Song) error {
	files, err := s.repo.FilesOfSong(song.ID)
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := s.repo.DeleteSongFile(f.ID); err != nil {
			return err
		}
		s.derefBlob(f.BlobHash)
	}
	return s.repo.DeleteSong(song.ID)
}

// derefBlob deletes a blob's bytes if no SongFile references it any longer.
// Best-effort: a failure to delete the bytes is not fatal to the operation.
func (s *Service) derefBlob(blobHash string) {
	if blobHash == "" {
		return
	}
	refs, err := s.repo.FilesWithBlob(blobHash)
	if err != nil || len(refs) > 0 {
		return
	}
	_ = s.blobs.Delete(blobHash)
}

// ---- song files ----

// allowedFileType reports whether ct is an accepted upload content type: a PDF or
// any image. It tolerates parameters (e.g. "image/png; charset=...").
func allowedFileType(ct string) bool {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if i := strings.IndexByte(ct, ';'); i >= 0 {
		ct = strings.TrimSpace(ct[:i])
	}
	return ct == "application/pdf" || strings.HasPrefix(ct, "image/")
}

// UploadSongFile stores an uploaded file for a song (any band member may upload).
// It validates the content by sniffing the bytes (declaredType is only a fallback),
// rejecting anything that is not a PDF or image with ErrInvalidInput. The bytes go
// to the content-addressed blob store; a SongFile metadata record is returned.
func (s *Service) UploadSongFile(caller User, bandID, songID, filename, declaredType string, data []byte) (SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return SongFile{}, ErrNotFound
	}
	if len(data) == 0 {
		return SongFile{}, fmt.Errorf("%w: empty file", ErrInvalidInput)
	}
	// Sniff the real type from the bytes; fall back to the declared type only when
	// the sniff is inconclusive (http.DetectContentType returns octet-stream).
	ct := http.DetectContentType(data)
	if ct == "application/octet-stream" && allowedFileType(declaredType) {
		ct = strings.ToLower(strings.TrimSpace(declaredType))
	}
	if !allowedFileType(ct) {
		return SongFile{}, fmt.Errorf("%w: only PDF or image files are allowed (got %q)", ErrInvalidInput, ct)
	}
	hash, err := s.blobs.Put(data)
	if err != nil {
		return SongFile{}, err
	}
	filename = strings.TrimSpace(filename)
	if filename == "" {
		filename = "file"
	}
	// T79: a part is a part — strip the upload's extension from the stored pool name (".pdf"/".png"
	// is noise; text charts already dropped it in T72). The extension is re-derived from ContentType
	// at the download boundary, so "Save as" still yields a usable file. New files only — no
	// migration of existing names. Guard the degenerate ".pdf"-only name (would strip to empty).
	if base := strings.TrimSuffix(filename, filepath.Ext(filename)); base != "" {
		filename = base
	}
	// Append at the end of the pool: stable, deterministic order across uploads.
	existing, _ := s.repo.FilesOfSong(songID)
	f := SongFile{
		ID:           s.newID(),
		SongID:       songID,
		BandID:       bandID,
		Filename:     filename,
		ContentType:  ct,
		Size:         int64(len(data)),
		BlobHash:     hash,
		UploadedBy:   caller.ID,
		DisplayOrder: len(existing),
		CreatedAt:    s.now().UTC(),
	}
	if err := s.repo.CreateSongFile(f); err != nil {
		return SongFile{}, err
	}
	return f, nil
}

// SongFiles lists the files attached to a song (member-only).
func (s *Service) SongFiles(caller User, bandID, songID string) ([]SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return nil, ErrNotFound
	}
	return s.repo.FilesOfSong(songID)
}

// SongFilePatch carries optional song-file updates (rename and/or reorder).
type SongFilePatch struct {
	Filename     *string
	DisplayOrder *int
}

// UpdateSongFile renames and/or reorders a song file (any band member).
func (s *Service) UpdateSongFile(caller User, bandID, songID, fileID string, p SongFilePatch) (SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID {
		return SongFile{}, ErrNotFound
	}
	if p.Filename != nil {
		name := strings.TrimSpace(*p.Filename)
		if name == "" {
			return SongFile{}, fmt.Errorf("%w: filename cannot be empty", ErrInvalidInput)
		}
		f.Filename = name
	}
	if p.DisplayOrder != nil {
		f.DisplayOrder = *p.DisplayOrder
	}
	if err := s.repo.UpdateSongFile(f); err != nil {
		return SongFile{}, err
	}
	return f, nil
}

// DeleteSongFile removes a song file (any band member) and dereferences its blob
// if no other SongFile points at the same bytes.
func (s *Service) DeleteSongFile(caller User, bandID, songID, fileID string) error {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID {
		return ErrNotFound
	}
	if err := s.repo.DeleteSongFile(fileID); err != nil {
		return err
	}
	s.derefBlob(f.BlobHash)
	if f.Generated {
		_ = s.repo.DeleteChartSource(fileID) // drop the editable source too
	}
	return nil
}

// ---- text charts (T19): a member writes a chart in the tiny chartpdf dialect;
// the server renders it to a PDF that enters the song's pool like any file, so
// everything downstream (view/annotate/my-files/bake/Stage) works unchanged. ----

// CreateTextChart renders source to a PDF and adds it to the song's pool as a
// GENERATED file (any band member), storing the editable source keyed by the new
// file id. ErrInvalidInput if the source uses characters the cp1252 renderer
// can't represent.
func (s *Service) CreateTextChart(caller User, bandID, songID, source string) (SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return SongFile{}, ErrNotFound
	}
	pdf, err := chartpdf.Render(source)
	if err != nil {
		return SongFile{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	hash, err := s.blobs.Put(pdf)
	if err != nil {
		return SongFile{}, err
	}
	existing, _ := s.repo.FilesOfSong(songID)
	f := SongFile{
		ID:           s.newID(),
		SongID:       songID,
		BandID:       bandID,
		Filename:     chartpdf.Title(source), // create-time default; no ".pdf" (it's source, not an upload) — T72
		ContentType:  "application/pdf",
		Size:         int64(len(pdf)),
		BlobHash:     hash,
		UploadedBy:   caller.ID,
		DisplayOrder: len(existing),
		CreatedAt:    s.now().UTC(),
		Generated:    true,
		Revision:     1,
	}
	if err := s.repo.CreateSongFile(f); err != nil {
		return SongFile{}, err
	}
	if err := s.repo.SetChartSource(f.ID, source); err != nil {
		return SongFile{}, err
	}
	return f, nil
}

// PreviewTextChart renders source to PDF bytes WITHOUT persisting anything (no
// blob, no file record) — the editor's write→see loop (T25). Member-gated to the
// song's band like the create path; ErrInvalidInput for unrenderable source.
func (s *Service) PreviewTextChart(caller User, bandID, songID, source string) ([]byte, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil || song.BandID != bandID {
		return nil, ErrNotFound
	}
	pdf, err := chartpdf.Render(source)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return pdf, nil
}

// ChartSource returns a generated file's editable source (member-only).
// ErrNotFound if the file isn't a generated text chart.
func (s *Service) ChartSource(caller User, bandID, songID, fileID string) (SongFile, string, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, "", err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID || !f.Generated {
		return SongFile{}, "", ErrNotFound
	}
	src, err := s.repo.GetChartSource(fileID)
	if err != nil {
		return SongFile{}, "", ErrNotFound
	}
	return f, src, nil
}

// SaveChartSource re-renders a generated file from new source, in place (same
// file id, Revision bumped). baseRevision is the revision the editor started from;
// a mismatch means someone else saved first → ErrConflict ("reload"). Any member
// may edit (like annotations). ErrInvalidInput for unrenderable source.
func (s *Service) SaveChartSource(caller User, bandID, songID, fileID string, baseRevision int, source string) (SongFile, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID || !f.Generated {
		return SongFile{}, ErrNotFound
	}
	if baseRevision != f.Revision {
		return SongFile{}, fmt.Errorf("%w: chart was changed by someone else — reload", ErrConflict)
	}
	pdf, err := chartpdf.Render(source)
	if err != nil {
		return SongFile{}, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	hash, err := s.blobs.Put(pdf)
	if err != nil {
		return SongFile{}, err
	}
	old := f.BlobHash
	f.BlobHash = hash
	f.Size = int64(len(pdf))
	// T72: do NOT re-derive Filename here — the name belongs to the user from create-time; a
	// source edit updates the blob/size/revision only, so a rename (e.g. "Guitar/Bass") sticks.
	f.Revision++
	if err := s.repo.UpdateSongFile(f); err != nil {
		return SongFile{}, err
	}
	if err := s.repo.SetChartSource(fileID, source); err != nil {
		return SongFile{}, err
	}
	if old != hash {
		s.derefBlob(old) // release the previous render if nothing else points at it
	}
	return f, nil
}

// TransposeEligible reports whether a setlist item's chord-transpose can be applied,
// and if not, names the first failing condition (T60 surface 2). The three conditions
// are the SINGLE source of truth shared by the bake decision, the bake warning, the
// playlist preview, and the Studio greying tooltip (mirrored client-side):
//  1. the song has a generated text chart to transpose,
//  2. the song key parses (the transpose "from"),
//  3. the override key parses (the transpose "to").
//
// The reason strings match the Studio tooltip copy.
func TransposeEligible(songKey, keyOverride string, hasGeneratedChart bool) (bool, string) {
	if !hasGeneratedChart {
		return false, "no text chart on this song"
	}
	if _, ok := chartpdf.ParseKey(songKey); !ok {
		return false, "song key not set or not parseable"
	}
	if _, ok := chartpdf.ParseKey(keyOverride); !ok {
		return false, "override key not parseable"
	}
	return true, ""
}

// BakeTransposeSucceeds reports whether an ELIGIBLE item's generated chart actually
// transposes + renders (D3). The baker falls through to the untransposed chart if the
// transform errors at bake, so eligibility passing does NOT guarantee the gig bundle is
// transposed; the bake-warning path calls this to surface that otherwise-silent fallthrough.
// Callers should only invoke it for items already TransposeEligible; a false return then
// means "eligible, but the transform failed at bake → baked untransposed".
func (s *Service) BakeTransposeSucceeds(caller User, bandID, songID, keyOverride string) bool {
	song, err := s.SongForMember(caller, bandID, songID)
	if err != nil {
		return false
	}
	files, err := s.SongFiles(caller, bandID, songID)
	if err != nil {
		return false
	}
	SortFiles(files)
	var chartID string
	for _, f := range files {
		if f.Generated {
			chartID = f.ID
			break
		}
	}
	if chartID == "" {
		return false
	}
	src, err := s.repo.GetChartSource(chartID)
	if err != nil {
		return false
	}
	from, ok1 := chartpdf.ParseKey(song.Key)
	to, ok2 := chartpdf.ParseKey(keyOverride)
	if !ok1 || !ok2 {
		return false
	}
	t, err := s.bakeTranspose(src, from, to)
	if err != nil {
		return false
	}
	if _, err := chartpdf.Render(t); err != nil {
		return false
	}
	return true
}

// SetlistItemChartPreview renders a setlist item's chart as it will appear on stage
// (T60 surface 2 preview): the song's first generated chart, transposed to the item's
// key override when the item asks for it and it's eligible, else rendered as-is (an
// identity "show me the chart from here"). No persistence. ErrNotFound when the song
// has no generated chart to preview.
func (s *Service) SetlistItemChartPreview(caller User, bandID, setlistID, itemID string) ([]byte, error) {
	if _, err := s.getSetlistForMember(caller, bandID, setlistID); err != nil {
		return nil, err
	}
	it, err := s.repo.GetSetlistItem(itemID)
	if err != nil || it.SetlistID != setlistID {
		return nil, ErrNotFound
	}
	song, err := s.repo.GetSong(it.SongID)
	if err != nil || song.BandID != bandID {
		return nil, ErrNotFound
	}
	files, err := s.repo.FilesOfSong(it.SongID)
	if err != nil {
		return nil, err
	}
	var chart *SongFile
	for i := range files {
		if files[i].Generated {
			chart = &files[i]
			break
		}
	}
	if chart == nil {
		return nil, ErrNotFound // no chart to preview
	}
	src, err := s.repo.GetChartSource(chart.ID)
	if err != nil {
		return nil, ErrNotFound
	}
	// hasGeneratedChart is true here (we found `chart`); transpose only if the item asks.
	if it.TransposeChords {
		if ok, _ := TransposeEligible(song.Key, it.KeyOverride, true); ok {
			from, _ := chartpdf.ParseKey(song.Key)
			to, _ := chartpdf.ParseKey(it.KeyOverride)
			if t, terr := chartpdf.TransposeToKey(src, it.KeyOverride, from, to); terr == nil { // D5
				src = t
			}
		}
	}
	pdf, err := chartpdf.Render(src)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}
	return pdf, nil
}

// TransposeChartSource transposes a generated chart's source (T60 surface 1). One
// atomic server op — the client never composes two writes:
//   - key path: when BOTH the song's key and targetKey parse, transpose by their
//     interval; targetKey is also the new song key when updateSongKey is set.
//   - semitone path: when the song key isn't parseable, transpose by `semitones`
//     (updateSongKey is ignored — there's no target key to write).
//   - dryRun: return the transposed source, persist nothing (the editor feeds it to
//     the existing preview machinery).
//   - persist: SaveChartSource semantics (same fileId, revision bump, 409 on stale
//     baseRevision) + optionally set song.key.
//
// ErrInvalidInput (400) when the file isn't a generated chart, or when neither a
// parseable targetKey (with a parseable song key) nor semitones is supplied.
func (s *Service) TransposeChartSource(caller User, bandID, songID, fileID, targetKey string, semitones *int, updateSongKey bool, baseRevision int, dryRun bool) (SongFile, string, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return SongFile{}, "", err
	}
	f, err := s.repo.GetSongFile(fileID)
	if err != nil || f.BandID != bandID || f.SongID != songID {
		return SongFile{}, "", ErrNotFound
	}
	if !f.Generated {
		return SongFile{}, "", fmt.Errorf("%w: not a text chart — only generated charts can be transposed", ErrInvalidInput)
	}
	src, err := s.repo.GetChartSource(fileID)
	if err != nil {
		return SongFile{}, "", ErrNotFound
	}
	song, err := s.repo.GetSong(songID)
	if err != nil {
		return SongFile{}, "", ErrNotFound
	}

	from, fromOK := chartpdf.ParseKey(song.Key)
	to, toOK := chartpdf.ParseKey(targetKey)
	var transposed string
	canUpdateKey := false
	switch {
	case fromOK && toOK:
		transposed, err = chartpdf.TransposeToKey(src, targetKey, from, to) // D5: respect F#/Gb spelling
		canUpdateKey = true
	case semitones != nil:
		transposed, err = chartpdf.TransposeSemitones(src, *semitones)
	default:
		return SongFile{}, "", fmt.Errorf("%w: need a parseable target key (song key must also parse) or a semitone count", ErrInvalidInput)
	}
	if err != nil {
		return SongFile{}, "", fmt.Errorf("%w: %v", ErrInvalidInput, err)
	}

	if dryRun {
		return f, transposed, nil // preview only — nothing persisted
	}

	saved, err := s.SaveChartSource(caller, bandID, songID, fileID, baseRevision, transposed)
	if err != nil {
		return SongFile{}, "", err // includes 409 on stale baseRevision, invalid render
	}
	if updateSongKey && canUpdateKey {
		if _, err := s.UpdateSong(caller, bandID, songID, SongPatch{Key: &targetKey}); err != nil {
			return SongFile{}, "", err
		}
	}
	return saved, transposed, nil
}

// DownloadSongFile returns a file's metadata and bytes, enforcing that the caller
// is a member of the owning band. Non-members get ErrForbidden (the file exists)
// or ErrNotFound (it does not) — never the bytes.
func (s *Service) DownloadSongFile(caller User, fileID string) (SongFile, []byte, error) {
	f, err := s.repo.GetSongFile(fileID)
	if err != nil {
		return SongFile{}, nil, ErrNotFound
	}
	if _, err := s.repo.GetMembership(f.BandID, caller.ID); err != nil {
		return SongFile{}, nil, ErrForbidden
	}
	data, err := s.blobs.Get(f.BlobHash)
	if err != nil {
		// T69: a generated chart's rendered blob is a CACHE of Render(source) — the source is
		// the source of truth, stored separately. If the blob is missing (orphaned historical
		// data), re-materialize it from source instead of 404ing. Deterministic render +
		// content-addressed Put restores the same bytes/hash, so the ?rev URL stays valid; the
		// revision is NOT bumped (same logical content, just re-cached). An uploaded file whose
		// bytes are gone has no source to heal from → still 404 (genuinely lost).
		if healed, hdata, herr := s.healGeneratedBlob(f); herr == nil {
			return healed, hdata, nil
		}
		return SongFile{}, nil, ErrNotFound
	}
	return f, data, nil
}

// healGeneratedBlob re-renders a generated chart's missing PDF from its stored chart source
// and re-stores it (T69). Returns an error when the file isn't a healable generated chart
// (not generated, or no stored source), or the re-render/store fails. On success the blob
// exists again and the returned file/bytes are current; the record's BlobHash is repointed
// only if the render drifted from the original (revision unchanged either way).
func (s *Service) healGeneratedBlob(f SongFile) (SongFile, []byte, error) {
	if !f.Generated {
		return SongFile{}, nil, fmt.Errorf("%w: not a generated file", ErrNotFound)
	}
	source, err := s.repo.GetChartSource(f.ID)
	if err != nil || source == "" {
		return SongFile{}, nil, fmt.Errorf("%w: no chart source to re-render from", ErrNotFound)
	}
	pdf, err := chartpdf.Render(source)
	if err != nil {
		return SongFile{}, nil, fmt.Errorf("re-render failed: %w", err)
	}
	hash, err := s.blobs.Put(pdf)
	if err != nil {
		return SongFile{}, nil, err
	}
	if hash != f.BlobHash {
		// The render drifted (source or renderer changed since) — repoint the record at the
		// bytes that now exist. Same logical revision (content re-materialized, not edited).
		f.BlobHash = hash
		f.Size = int64(len(pdf))
		if err := s.repo.UpdateSongFile(f); err != nil {
			return SongFile{}, nil, err
		}
	}
	return f, pdf, nil
}

// BlobRepairReport summarizes a repair-blobs pass (T69): how many file records were scanned,
// how many already had their blob, the ids re-rendered from source, and the ids whose blob is
// gone and can't be regenerated (uploaded files — bytes genuinely lost, need re-upload).
type BlobRepairReport struct {
	Scanned   int
	Healthy   int
	Healed    []string
	Unfixable []SongFile
}

// RepairMissingBlobs scans every song-file record and re-materializes any generated chart
// whose rendered blob is missing (from its stored source), reporting uploaded files whose
// bytes are genuinely lost. The operator entry point for `troubacore repair-blobs` — heals a
// box in one pass instead of waiting for each file to be viewed (the download-time auto-heal).
func (s *Service) RepairMissingBlobs() (BlobRepairReport, error) {
	files, err := s.repo.AllSongFiles()
	if err != nil {
		return BlobRepairReport{}, err
	}
	rep := BlobRepairReport{Scanned: len(files)}
	for _, f := range files {
		if _, err := s.blobs.Get(f.BlobHash); err == nil {
			rep.Healthy++
			continue
		}
		if _, _, herr := s.healGeneratedBlob(f); herr == nil {
			rep.Healed = append(rep.Healed, f.ID)
		} else {
			rep.Unfixable = append(rep.Unfixable, f)
		}
	}
	return rep, nil
}

// ---- per-member file selection ----

// MyFileSelection resolves the caller's personal, ordered view of a song's file
// pool (member-only; the selection is always the CALLER's own). Customized reports
// whether the member has a saved selection:
//   - customized=true: the pool files matching the member's saved fileIds, in the
//     saved order, skipping any fileId no longer in the pool (deleted files drop
//     out gracefully).
//   - customized=false (member never set one): ALL pool files in DisplayOrder, so
//     a new member sees everything by default.
func (s *Service) MyFileSelection(caller User, bandID, songID string) ([]SongFile, bool, error) {
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, false, err
	}
	pool, err := s.repo.FilesOfSong(songID)
	if err != nil {
		return nil, false, err
	}
	sel, err := s.repo.GetFileSelection(caller.ID, songID)
	if err != nil {
		// No saved selection → default to all pool files in DisplayOrder.
		sort.Slice(pool, func(i, j int) bool { return pool[i].DisplayOrder < pool[j].DisplayOrder })
		return pool, false, nil
	}
	return resolveSelection(pool, sel.FileIDs), true, nil
}

// SetMyFileSelection replaces the caller's ordered selection for a song. Every
// fileId must belong to this song's pool (ErrInvalidInput otherwise). An empty
// list is allowed (the member chose to show nothing). Returns the resolved+ordered
// files (always customized=true on success).
func (s *Service) SetMyFileSelection(caller User, bandID, songID string, fileIDs []string) ([]SongFile, error) {
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, err
	}
	pool, err := s.repo.FilesOfSong(songID)
	if err != nil {
		return nil, err
	}
	inPool := make(map[string]bool, len(pool))
	for _, f := range pool {
		inPool[f.ID] = true
	}
	seen := make(map[string]bool, len(fileIDs))
	clean := make([]string, 0, len(fileIDs))
	for _, id := range fileIDs {
		if !inPool[id] {
			return nil, fmt.Errorf("%w: file %q does not belong to this song", ErrInvalidInput, id)
		}
		if seen[id] {
			return nil, fmt.Errorf("%w: duplicate file id %q", ErrInvalidInput, id)
		}
		seen[id] = true
		clean = append(clean, id)
	}
	if err := s.repo.SetFileSelection(FileSelection{UserID: caller.ID, SongID: songID, FileIDs: clean}); err != nil {
		return nil, err
	}
	return resolveSelection(pool, clean), nil
}

// ClearMyFileSelection removes the caller's customization, reverting to the
// default (all pool files). Idempotent.
func (s *Service) ClearMyFileSelection(caller User, bandID, songID string) error {
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return err
	}
	return s.repo.DeleteFileSelection(caller.ID, songID)
}

// cueColorRe validates a cue tint: empty (neutral) or a 6-hex "#rrggbb". The model
// deliberately accepts any hex (the UI offers a fixed stage palette; future custom
// colors cost nothing) — it just rejects malformed input.
var cueColorRe = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

// maxSongCues bounds a member's cue list server-side. The UI caps at 4 for
// glanceability (T50); this is a generous abuse guard well above that so a
// legitimate list is never rejected but a runaway payload is.
const maxSongCues = 32

// MyCues returns the caller's PERSONAL, ordered cues for a song (T50). Member-gated
// like other song reads; an empty (never-set) list is not an error — it returns an
// empty slice. Self-only by construction: the caller can only ever read their own.
func (s *Service) MyCues(caller User, bandID, songID string) ([]SongCue, error) {
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, err
	}
	sc, err := s.repo.GetSongCues(caller.ID, songID)
	if err != nil {
		return []SongCue{}, nil // ErrNotFound → the member simply has no cues here
	}
	return sc.Cues, nil
}

// MemberSongCues is one member's personal cues for a song (P205 band-wide bake).
type MemberSongCues struct {
	MemberID string    `json:"memberId"`
	Cues     []SongCue `json:"cues"`
}

// AllMemberCues returns EVERY band member's personal cues for a song, keyed by
// member id, in the deterministic Members order (P205 band-wide bake). ADMIN-only:
// unlike MyCues (self-only), this exposes other members' cues, so it is gated like
// the band-wide bake that consumes it (bake is admin-only, I11). Members with no
// cues for the song are omitted (never a hollow entry).
func (s *Service) AllMemberCues(caller User, bandID, songID string) ([]MemberSongCues, error) {
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return nil, err
	}
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, err
	}
	members, err := s.Members(caller, bandID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberSongCues, 0, len(members))
	for _, m := range members {
		sc, err := s.repo.GetSongCues(m.User.ID, songID)
		if err != nil || len(sc.Cues) == 0 {
			continue // ErrNotFound / no cues → omit this member
		}
		out = append(out, MemberSongCues{MemberID: m.User.ID, Cues: sc.Cues})
	}
	return out, nil
}

// MemberFileSelection is one member's ordered file choice for a song (T137 per-member
// reading sequence in the band-wide bake).
type MemberFileSelection struct {
	MemberID string   `json:"memberId"`
	FileIDs  []string `json:"fileIds"`
}

// AllFileSelections returns EVERY band member's personal, ordered file selection for a
// song, keyed by member id, in the deterministic Members order (T137). ADMIN-only and
// gated like the band-wide bake that consumes it (I11) — it exposes other members'
// selections, exactly as AllMemberCues exposes their cues. A member with no selection is
// omitted (they read the default; the baker resolves that), never a hollow entry.
func (s *Service) AllFileSelections(caller User, bandID, songID string) ([]MemberFileSelection, error) {
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return nil, err
	}
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, err
	}
	members, err := s.Members(caller, bandID)
	if err != nil {
		return nil, err
	}
	out := make([]MemberFileSelection, 0, len(members))
	for _, m := range members {
		sel, err := s.repo.GetFileSelection(m.User.ID, songID)
		if err != nil || len(sel.FileIDs) == 0 {
			continue // ErrNotFound / empty → omit; this member reads the default
		}
		out = append(out, MemberFileSelection{MemberID: m.User.ID, FileIDs: sel.FileIDs})
	}
	return out, nil
}

// SetMyCues replaces the caller's ordered cue list for a song (T50). Self-only by
// construction — cues are always keyed to caller.ID, so a member can never write
// another's. An empty list clears the cues. Each icon id must be non-empty; each
// color must be "" or "#rrggbb". Returns the stored list.
func (s *Service) SetMyCues(caller User, bandID, songID string, cues []SongCue) ([]SongCue, error) {
	if _, err := s.SongForMember(caller, bandID, songID); err != nil {
		return nil, err
	}
	if len(cues) > maxSongCues {
		return nil, fmt.Errorf("%w: too many cues (max %d)", ErrInvalidInput, maxSongCues)
	}
	clean := make([]SongCue, 0, len(cues))
	for _, c := range cues {
		icon := strings.TrimSpace(c.Icon)
		if icon == "" {
			return nil, fmt.Errorf("%w: cue icon must not be empty", ErrInvalidInput)
		}
		color := strings.TrimSpace(c.Color)
		if color != "" && !cueColorRe.MatchString(color) {
			return nil, fmt.Errorf("%w: cue color %q must be empty or #rrggbb", ErrInvalidInput, color)
		}
		clean = append(clean, SongCue{Icon: icon, Color: color})
	}
	if len(clean) == 0 {
		// Empty list clears the customization (idempotent) rather than storing a
		// hollow record; MyCues then reports [] exactly as before.
		if err := s.repo.DeleteSongCues(caller.ID, songID); err != nil {
			return nil, err
		}
		return []SongCue{}, nil
	}
	if err := s.repo.SetSongCues(SongCues{UserID: caller.ID, SongID: songID, Cues: clean}); err != nil {
		return nil, err
	}
	return clean, nil
}

// resolveSelection maps an ordered list of fileIds to their SongFile records from
// the pool, in that order, silently dropping ids no longer present in the pool.
func resolveSelection(pool []SongFile, fileIDs []string) []SongFile {
	byID := make(map[string]SongFile, len(pool))
	for _, f := range pool {
		byID[f.ID] = f
	}
	out := make([]SongFile, 0, len(fileIDs))
	for _, id := range fileIDs {
		if f, ok := byID[id]; ok {
			out = append(out, f)
		}
	}
	return out
}

// ---- setlists ----

// Setlists lists a band's setlists (member-only).
func (s *Service) Setlists(caller User, bandID string) ([]Setlist, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	return s.repo.SetlistsOfBand(bandID)
}

// SetlistView is a Setlist plus the list-only metadata the concert list needs (T131) — today just the
// song count, so the list can reproduce the empty-setlist bake guard WITHOUT a per-row detail fetch
// (that would turn one request into N). SongCount is computed at read time, never persisted.
type SetlistView struct {
	Setlist
	SongCount int `json:"songCount"`
}

// SetlistsWithCounts is Setlists plus each setlist's song count. It reads the items per setlist (a
// cheap id lookup, not the full detail with song/file joins), so a concert list stays one request.
func (s *Service) SetlistsWithCounts(caller User, bandID string) ([]SetlistView, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	sls, err := s.repo.SetlistsOfBand(bandID)
	if err != nil {
		return nil, err
	}
	out := make([]SetlistView, 0, len(sls))
	for _, sl := range sls {
		items, err := s.repo.ItemsOfSetlist(sl.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, SetlistView{Setlist: sl, SongCount: len(items)})
	}
	return out, nil
}

// SetlistInput carries the create/patch fields for a setlist.
type SetlistInput struct {
	Name      *string
	EventDate *string
	Venue     *string
	Notes     *string
}

// validEventDate reports whether d is empty or a well-formed ISO yyyy-mm-dd date.
func validEventDate(d string) bool {
	if d == "" {
		return true
	}
	_, err := time.Parse("2006-01-02", d)
	return err == nil
}

// CreateSetlist adds a setlist to a band (any member). Name is required; eventDate,
// if present, must be ISO yyyy-mm-dd.
func (s *Service) CreateSetlist(caller User, bandID, name, eventDate, venue, notes string) (Setlist, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return Setlist{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return Setlist{}, fmt.Errorf("%w: setlist name is required", ErrInvalidInput)
	}
	eventDate = strings.TrimSpace(eventDate)
	if !validEventDate(eventDate) {
		return Setlist{}, fmt.Errorf("%w: eventDate must be ISO yyyy-mm-dd", ErrInvalidInput)
	}
	sl := Setlist{
		ID:        s.newID(),
		BandID:    bandID,
		Name:      name,
		EventDate: eventDate,
		Venue:     strings.TrimSpace(venue),
		Notes:     notes,
		CreatedAt: s.now().UTC(),
	}
	if err := s.repo.CreateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

// SetlistDetail is a setlist with its ordered items, each enriched with the song's
// title/artist so the client renders without N extra lookups.
type SetlistDetail struct {
	Setlist Setlist           `json:"setlist"`
	Items   []SetlistItemView `json:"items"`
}

// SetlistItemView is a SetlistItem plus the referenced song's display fields.
type SetlistItemView struct {
	SetlistItem
	SongTitle  string `json:"songTitle"`
	SongArtist string `json:"songArtist,omitempty"`
	SongMeter  string `json:"songMeter,omitempty"` // T86: rides into the bake so the Stage beat knows the metre
	SongTempo  int    `json:"songTempo,omitempty"` // T86: the song's BASE tempo, so the bake carries it when no setlist override
	// T60: hints for the Studio transpose checkbox greying — the song's key and whether
	// it has a generated chart. The client parses the (live-edited) keyOverride itself;
	// these two don't change while editing, so they ride the view instead of N fetches.
	SongKey  string `json:"songKey,omitempty"`
	HasChart bool   `json:"hasChart,omitempty"`
}

// getSetlistForMember resolves a setlist scoped to a band the caller belongs to.
func (s *Service) getSetlistForMember(caller User, bandID, setlistID string) (Setlist, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return Setlist{}, err
	}
	sl, err := s.repo.GetSetlist(setlistID)
	if err != nil || sl.BandID != bandID {
		return Setlist{}, ErrNotFound
	}
	return sl, nil
}

// Setlist returns a setlist with its items sorted by Position (member-only).
func (s *Service) Setlist(caller User, bandID, setlistID string) (SetlistDetail, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return SetlistDetail{}, err
	}
	items, err := s.repo.ItemsOfSetlist(setlistID)
	if err != nil {
		return SetlistDetail{}, err
	}
	// Main order first, then the bench (on-call) items — the order the baker emits
	// and Studio sections on (T23). Within each group, by Position then ID.
	sort.Slice(items, func(i, j int) bool {
		if items[i].OnCall != items[j].OnCall {
			return !items[i].OnCall // main (false) before bench (true)
		}
		if items[i].Position != items[j].Position {
			return items[i].Position < items[j].Position
		}
		return items[i].ID < items[j].ID
	})
	views := make([]SetlistItemView, 0, len(items))
	for _, it := range items {
		v := SetlistItemView{SetlistItem: it}
		if song, err := s.repo.GetSong(it.SongID); err == nil {
			v.SongTitle = song.Title
			v.SongArtist = song.Artist
			v.SongKey = song.Key
			v.SongMeter = song.Meter
			v.SongTempo = song.Tempo
		}
		if files, err := s.repo.FilesOfSong(it.SongID); err == nil {
			for _, f := range files {
				if f.Generated {
					v.HasChart = true
					break
				}
			}
		}
		views = append(views, v)
	}
	return SetlistDetail{Setlist: sl, Items: views}, nil
}

// UpdateSetlist patches a setlist's metadata (any member). Only supplied fields
// change; Name (if supplied) must be non-empty; eventDate (if supplied) must be ISO.
func (s *Service) UpdateSetlist(caller User, bandID, setlistID string, in SetlistInput) (Setlist, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return Setlist{}, err
	}
	if in.Name != nil {
		name := strings.TrimSpace(*in.Name)
		if name == "" {
			return Setlist{}, fmt.Errorf("%w: setlist name cannot be empty", ErrInvalidInput)
		}
		sl.Name = name
	}
	if in.EventDate != nil {
		d := strings.TrimSpace(*in.EventDate)
		if !validEventDate(d) {
			return Setlist{}, fmt.Errorf("%w: eventDate must be ISO yyyy-mm-dd", ErrInvalidInput)
		}
		sl.EventDate = d
	}
	if in.Venue != nil {
		sl.Venue = strings.TrimSpace(*in.Venue)
	}
	if in.Notes != nil {
		sl.Notes = *in.Notes
	}
	if err := s.repo.UpdateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

// DeleteSetlist removes a setlist and its items (admin-only).
func (s *Service) DeleteSetlist(caller User, bandID, setlistID string) error {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return err
	}
	sl, err := s.repo.GetSetlist(setlistID)
	if err != nil || sl.BandID != bandID {
		return ErrNotFound
	}
	return s.deleteSetlistCascade(setlistID)
}

// SetSetlistLive turns rehearsal live mode (P201/I11) on or off for a setlist,
// admin-only. ON sets LiveUntil to now+LiveModeWindow (a bounded, self-expiring
// window — a forgotten live mode auto-clears before the gig); OFF zeroes it.
// Returns the updated setlist so the caller can report the (computed) live state.
func (s *Service) SetSetlistLive(caller User, bandID, setlistID string, live bool) (Setlist, error) {
	if _, err := s.repo.GetBand(bandID); err != nil {
		return Setlist{}, ErrNotFound
	}
	if _, err := s.requireRole(bandID, caller.ID, RoleAdmin); err != nil {
		return Setlist{}, err
	}
	sl, err := s.repo.GetSetlist(setlistID)
	if err != nil || sl.BandID != bandID {
		return Setlist{}, ErrNotFound
	}
	if live {
		sl.LiveUntil = s.now().UTC().Add(LiveModeWindow)
		sl.LiveBy = caller.ID
	} else {
		sl.LiveUntil = time.Time{}
		sl.LiveBy = ""
	}
	if err := s.repo.UpdateSetlist(sl); err != nil {
		return Setlist{}, err
	}
	return sl, nil
}

// SetlistLiveNow reports whether a setlist is in live mode as of the service clock —
// the app-layer read used by the autobaker (stage 1b) and the API response.
func (s *Service) SetlistLiveNow(sl Setlist) bool { return SetlistLive(sl, s.now().UTC()) }

// LiveSetlistsForSong returns the band's setlists that are in live mode AS OF NOW and
// contain the given song — the autobaker's (stage 1b) reverse lookup on each committed
// annotation. Caller-less (system-triggered); resolves song→band, filters live, checks
// membership. A song not found, or in no live setlist, yields an empty slice + nil err.
func (s *Service) LiveSetlistsForSong(songID string) ([]Setlist, error) {
	song, err := s.repo.GetSong(songID)
	if err != nil {
		return nil, nil // unknown song → nothing to bake (not an error on the commit path)
	}
	sls, err := s.repo.SetlistsOfBand(song.BandID)
	if err != nil {
		return nil, err
	}
	now := s.now().UTC()
	var live []Setlist
	for _, sl := range sls {
		if !SetlistLive(sl, now) {
			continue
		}
		items, err := s.repo.ItemsOfSetlist(sl.ID)
		if err != nil {
			return nil, err
		}
		for _, it := range items {
			if it.SongID == songID {
				live = append(live, sl)
				break
			}
		}
	}
	return live, nil
}

// UserByID resolves a user for the autobaker's bake actor (the LiveBy admin).
func (s *Service) UserByID(id string) (User, error) { return s.repo.GetUser(id) }

// deleteSetlistCascade deletes a setlist's items then the setlist record. No
// permission check — callers must enforce.
func (s *Service) deleteSetlistCascade(setlistID string) error {
	items, err := s.repo.ItemsOfSetlist(setlistID)
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := s.repo.DeleteSetlistItem(it.ID); err != nil {
			return err
		}
	}
	return s.repo.DeleteSetlist(setlistID)
}

// AddSetlistItem appends a song to a setlist (any member). The song must belong to
// the same band as the setlist (ErrInvalidInput otherwise).
func (s *Service) AddSetlistItem(caller User, bandID, setlistID, songID string) (SetlistItem, error) {
	sl, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return SetlistItem{}, err
	}
	song, err := s.repo.GetSong(songID)
	if err != nil {
		return SetlistItem{}, fmt.Errorf("%w: song not found", ErrInvalidInput)
	}
	if song.BandID != bandID {
		return SetlistItem{}, fmt.Errorf("%w: song does not belong to this band", ErrInvalidInput)
	}
	// Append at the end: next position is max(existing)+1.
	items, err := s.repo.ItemsOfSetlist(setlistID)
	if err != nil {
		return SetlistItem{}, err
	}
	pos := 0
	for _, it := range items {
		if it.Position >= pos {
			pos = it.Position + 1
		}
	}
	item := SetlistItem{
		ID:        s.newID(),
		SetlistID: sl.ID,
		SongID:    songID,
		Position:  pos,
	}
	if err := s.repo.CreateSetlistItem(item); err != nil {
		return SetlistItem{}, err
	}
	return item, nil
}

// SetlistItemPatch carries optional per-item overrides.
type SetlistItemPatch struct {
	KeyOverride     *string
	TempoOverride   *int
	Notes           *string
	OnCall          *bool // move to/from the bench (T23)
	TransposeChords *bool // burn the chart transposed to keyOverride at bake (T60)
}

// UpdateSetlistItem patches an item's overrides/notes (any member).
func (s *Service) UpdateSetlistItem(caller User, bandID, setlistID, itemID string, p SetlistItemPatch) (SetlistItem, error) {
	if _, err := s.getSetlistForMember(caller, bandID, setlistID); err != nil {
		return SetlistItem{}, err
	}
	it, err := s.repo.GetSetlistItem(itemID)
	if err != nil || it.SetlistID != setlistID {
		return SetlistItem{}, ErrNotFound
	}
	if p.KeyOverride != nil {
		it.KeyOverride = strings.TrimSpace(*p.KeyOverride)
	}
	if p.TempoOverride != nil {
		if *p.TempoOverride < 0 {
			return SetlistItem{}, fmt.Errorf("%w: tempoOverride cannot be negative", ErrInvalidInput)
		}
		it.TempoOverride = *p.TempoOverride
	}
	if p.Notes != nil {
		it.Notes = *p.Notes
	}
	if p.OnCall != nil {
		it.OnCall = *p.OnCall
	}
	if p.TransposeChords != nil {
		it.TransposeChords = *p.TransposeChords
	}
	if err := s.repo.UpdateSetlistItem(it); err != nil {
		return SetlistItem{}, err
	}
	return it, nil
}

// RemoveSetlistItem deletes an item from a setlist (any member).
func (s *Service) RemoveSetlistItem(caller User, bandID, setlistID, itemID string) error {
	if _, err := s.getSetlistForMember(caller, bandID, setlistID); err != nil {
		return err
	}
	it, err := s.repo.GetSetlistItem(itemID)
	if err != nil || it.SetlistID != setlistID {
		return ErrNotFound
	}
	return s.repo.DeleteSetlistItem(itemID)
}

// ReorderSetlist reassigns item positions by the order of orderedItemIDs (any
// member). The set of ids must exactly match the setlist's current items, else
// ErrInvalidInput. Returns the reordered items sorted by their new Position.
func (s *Service) ReorderSetlist(caller User, bandID, setlistID string, orderedItemIDs []string) ([]SetlistItem, error) {
	if _, err := s.getSetlistForMember(caller, bandID, setlistID); err != nil {
		return nil, err
	}
	items, err := s.repo.ItemsOfSetlist(setlistID)
	if err != nil {
		return nil, err
	}
	if len(orderedItemIDs) != len(items) {
		return nil, fmt.Errorf("%w: orderedItemIds must list every item exactly once", ErrInvalidInput)
	}
	byID := make(map[string]SetlistItem, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	seen := make(map[string]bool, len(orderedItemIDs))
	for _, id := range orderedItemIDs {
		if seen[id] {
			return nil, fmt.Errorf("%w: duplicate item id %q", ErrInvalidInput, id)
		}
		if _, ok := byID[id]; !ok {
			return nil, fmt.Errorf("%w: unknown item id %q", ErrInvalidInput, id)
		}
		seen[id] = true
	}
	out := make([]SetlistItem, 0, len(orderedItemIDs))
	for pos, id := range orderedItemIDs {
		it := byID[id]
		it.Position = pos
		if err := s.repo.UpdateSetlistItem(it); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, nil
}

// DuplicateSetlist deep-copies a setlist (any member — creating setlists is
// member-level): a new setlist named "<original> (copy)" with the same metadata and
// every item's song/position/overrides. The copy is independent — a fresh id, no
// shared items, and no bake history (its concertId = the new id), so baking it mints
// rev 1 by construction.
func (s *Service) DuplicateSetlist(caller User, bandID, setlistID string) (Setlist, error) {
	src, err := s.getSetlistForMember(caller, bandID, setlistID)
	if err != nil {
		return Setlist{}, err
	}
	items, err := s.repo.ItemsOfSetlist(src.ID)
	if err != nil {
		return Setlist{}, err
	}
	dup, err := s.CreateSetlist(caller, bandID, src.Name+" (copy)", src.EventDate, src.Venue, src.Notes)
	if err != nil {
		return Setlist{}, err
	}
	for _, it := range items {
		if err := s.repo.CreateSetlistItem(SetlistItem{
			ID:            s.newID(),
			SetlistID:     dup.ID,
			SongID:        it.SongID,
			Position:      it.Position,
			KeyOverride:   it.KeyOverride,
			TempoOverride: it.TempoOverride,
			Notes:         it.Notes,
			OnCall:        it.OnCall, // bench membership copies too (T23)
		}); err != nil {
			return Setlist{}, err
		}
	}
	return dup, nil
}

// ---- helpers ----

func identifiersOf(u User) []IdentifierMatch {
	out := []IdentifierMatch{
		{Kind: KindUsername, Identifier: strings.ToLower(u.Username)},
		{Kind: KindUUID, Identifier: u.ID},
	}
	if u.Email != "" {
		out = append(out, IdentifierMatch{Kind: KindEmail, Identifier: strings.ToLower(u.Email)})
	}
	return out
}

func inviteMatchesUser(inv Invite, u User) bool {
	for _, m := range identifiersOf(u) {
		if m.Kind == inv.IdentifierKind && m.Identifier == inv.Identifier {
			return true
		}
	}
	return false
}

func normalizeIdentifier(kind IdentifierKind, id string) string {
	id = strings.TrimSpace(id)
	switch kind {
	case KindUsername, KindEmail:
		return strings.ToLower(id)
	default:
		return id
	}
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func newToken() string {
	var b [32]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// newInviteToken returns an unguessable URL-safe token (~24 random bytes,
// base64url, no padding) for tokenized invite links.
func newInviteToken() string {
	var b [24]byte
	_, _ = rand.Read(b[:])
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// emailRe is a deliberately lenient "looks like an email" check (the app does not
// verify deliverability): something@something.tld with no whitespace.
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func validEmail(email string) bool {
	return emailRe.MatchString(email)
}

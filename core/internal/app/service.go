package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"troubastack/core/internal/app/blob"
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
}

// NewService wires a Service over a Repo with production defaults. The blob store
// defaults to in-memory; call WithBlobStore to use a persistent backend.
func NewService(repo Repo) *Service {
	return &Service{repo: repo, blobs: blob.NewMem(), now: time.Now, newID: newUUID}
}

// WithBlobStore swaps the blob backend (file-backed for persistent deployments)
// and returns the Service for chaining.
func (s *Service) WithBlobStore(b blob.Store) *Service {
	s.blobs = b
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
	out := make([]MemberView, 0, len(ms))
	for _, m := range ms {
		u, err := s.repo.GetUser(m.UserID)
		if err != nil {
			continue
		}
		out = append(out, MemberView{User: u.Public(), Role: m.Role})
	}
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

// ---- songs ----

// Songs lists a band's songs (member-only).
func (s *Service) Songs(caller User, bandID string) ([]Song, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	return s.repo.SongsOfBand(bandID)
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
	f := SongFile{
		ID:          s.newID(),
		SongID:      songID,
		BandID:      bandID,
		Filename:    filename,
		ContentType: ct,
		Size:        int64(len(data)),
		BlobHash:    hash,
		UploadedBy:  caller.ID,
		CreatedAt:   s.now().UTC(),
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
		return SongFile{}, nil, ErrNotFound
	}
	return f, data, nil
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

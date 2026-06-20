package app

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
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

// SongPatch carries optional song-metadata updates. A nil pointer leaves the field
// unchanged; a non-nil pointer sets it (including to an empty/zero value).
type SongPatch struct {
	Title  *string
	Artist *string
	Key    *string
	Tempo  *int
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
	return nil
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

// ---- setlists ----

// Setlists lists a band's setlists (member-only).
func (s *Service) Setlists(caller User, bandID string) ([]Setlist, error) {
	if _, _, err := s.GetBand(caller, bandID); err != nil {
		return nil, err
	}
	return s.repo.SetlistsOfBand(bandID)
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
	sort.Slice(items, func(i, j int) bool { return items[i].Position < items[j].Position })
	views := make([]SetlistItemView, 0, len(items))
	for _, it := range items {
		v := SetlistItemView{SetlistItem: it}
		if song, err := s.repo.GetSong(it.SongID); err == nil {
			v.SongTitle = song.Title
			v.SongArtist = song.Artist
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
	KeyOverride   *string
	TempoOverride *int
	Notes         *string
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

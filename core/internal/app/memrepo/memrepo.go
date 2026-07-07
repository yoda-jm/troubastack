// Package memrepo is an in-memory app.Repo: the reference backend for tests and
// throwaway dev runs. State is lost on restart. It enforces the same uniqueness
// and not-found contract as the file backend, just without disk.
//
// Boundary: imports app + stdlib only.
package memrepo

import (
	"strings"
	"sync"

	"troubastack/core/internal/app"
)

// Repo is the in-memory store. All access is mutex-guarded so it is safe for
// concurrent HTTP handlers.
type Repo struct {
	mu sync.RWMutex

	users          map[string]app.User          // id -> user
	sessions       map[string]app.Session       // token -> session
	passwordResets map[string]app.PasswordReset // token hash -> reset grant
	bands          map[string]app.Band          // id -> band
	members        map[string]app.Membership    // bandID|userID -> membership
	invites        map[string]app.Invite        // id -> invite
	inviteLinks    map[string]app.InviteLink    // id -> invite link
	songs          map[string]app.Song          // id -> song
	files          map[string]app.SongFile      // id -> song file
	selections     map[string]app.FileSelection // userID|songID -> personal selection
	setlists       map[string]app.Setlist       // id -> setlist
	setlistItems   map[string]app.SetlistItem   // id -> setlist item
}

// New returns an empty in-memory Repo.
func New() *Repo {
	return &Repo{
		users:          map[string]app.User{},
		sessions:       map[string]app.Session{},
		passwordResets: map[string]app.PasswordReset{},
		bands:          map[string]app.Band{},
		members:        map[string]app.Membership{},
		invites:        map[string]app.Invite{},
		inviteLinks:    map[string]app.InviteLink{},
		songs:          map[string]app.Song{},
		files:          map[string]app.SongFile{},
		selections:     map[string]app.FileSelection{},
		setlists:       map[string]app.Setlist{},
		setlistItems:   map[string]app.SetlistItem{},
	}
}

var _ app.Repo = (*Repo)(nil)

func memberKey(bandID, userID string) string { return bandID + "|" + userID }

func selectionKey(userID, songID string) string { return userID + "|" + songID }

// ---- users ----

func (r *Repo) CreateUser(u app.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.users {
		if strings.EqualFold(e.Username, u.Username) {
			return app.ErrConflict
		}
		if u.Email != "" && strings.EqualFold(e.Email, u.Email) {
			return app.ErrConflict
		}
	}
	r.users[u.ID] = u
	return nil
}

func (r *Repo) GetUser(id string) (app.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	u, ok := r.users[id]
	if !ok {
		return app.User{}, app.ErrNotFound
	}
	return u, nil
}

func (r *Repo) GetUserByUsername(username string) (app.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, u := range r.users {
		if strings.EqualFold(u.Username, username) {
			return u, nil
		}
	}
	return app.User{}, app.ErrNotFound
}

func (r *Repo) GetUserByEmail(email string) (app.User, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if email == "" {
		return app.User{}, app.ErrNotFound
	}
	for _, u := range r.users {
		if u.Email != "" && strings.EqualFold(u.Email, email) {
			return u, nil
		}
	}
	return app.User{}, app.ErrNotFound
}

func (r *Repo) UpdateUser(u app.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.users[u.ID]; !ok {
		return app.ErrNotFound
	}
	// Enforce uniqueness against OTHER users.
	for id, e := range r.users {
		if id == u.ID {
			continue
		}
		if strings.EqualFold(e.Username, u.Username) {
			return app.ErrConflict
		}
		if u.Email != "" && strings.EqualFold(e.Email, u.Email) {
			return app.ErrConflict
		}
	}
	r.users[u.ID] = u
	return nil
}

// ---- sessions ----

func (r *Repo) CreateSession(s app.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[s.Token] = s
	return nil
}

func (r *Repo) GetSession(token string) (app.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[token]
	if !ok {
		return app.Session{}, app.ErrNotFound
	}
	return s, nil
}

func (r *Repo) DeleteSession(token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.sessions[token]; !ok {
		return app.ErrNotFound
	}
	delete(r.sessions, token)
	return nil
}

func (r *Repo) DeleteSessionsForUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for token, s := range r.sessions {
		if s.UserID == userID {
			delete(r.sessions, token)
		}
	}
	return nil
}

// ---- password resets ----

func (r *Repo) CreatePasswordReset(pr app.PasswordReset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.passwordResets[pr.TokenHash] = pr
	return nil
}

func (r *Repo) GetPasswordReset(tokenHash string) (app.PasswordReset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pr, ok := r.passwordResets[tokenHash]
	if !ok {
		return app.PasswordReset{}, app.ErrNotFound
	}
	return pr, nil
}

func (r *Repo) DeletePasswordReset(tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.passwordResets[tokenHash]; !ok {
		return app.ErrNotFound
	}
	delete(r.passwordResets, tokenHash)
	return nil
}

// ---- bands & memberships ----

func (r *Repo) CreateBand(b app.Band) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.bands[b.ID] = b
	return nil
}

func (r *Repo) GetBand(id string) (app.Band, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	b, ok := r.bands[id]
	if !ok {
		return app.Band{}, app.ErrNotFound
	}
	return b, nil
}

func (r *Repo) UpdateBand(b app.Band) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.bands[b.ID]; !ok {
		return app.ErrNotFound
	}
	r.bands[b.ID] = b
	return nil
}

func (r *Repo) DeleteBand(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.bands[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.bands, id)
	return nil
}

func (r *Repo) BandsForUser(userID string) ([]app.Band, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Band
	for _, m := range r.members {
		if m.UserID == userID {
			if b, ok := r.bands[m.BandID]; ok {
				out = append(out, b)
			}
		}
	}
	app.SortBands(out)
	return out, nil
}

func (r *Repo) AddMembership(m app.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := memberKey(m.BandID, m.UserID)
	if _, ok := r.members[k]; ok {
		return app.ErrConflict
	}
	r.members[k] = m
	return nil
}

func (r *Repo) GetMembership(bandID, userID string) (app.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.members[memberKey(bandID, userID)]
	if !ok {
		return app.Membership{}, app.ErrNotFound
	}
	return m, nil
}

func (r *Repo) UpdateMembership(m app.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := memberKey(m.BandID, m.UserID)
	if _, ok := r.members[k]; !ok {
		return app.ErrNotFound
	}
	r.members[k] = m
	return nil
}

func (r *Repo) DeleteMembership(bandID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := memberKey(bandID, userID)
	if _, ok := r.members[k]; !ok {
		return app.ErrNotFound
	}
	delete(r.members, k)
	return nil
}

func (r *Repo) MembersOfBand(bandID string) ([]app.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Membership
	for _, m := range r.members {
		if m.BandID == bandID {
			out = append(out, m)
		}
	}
	app.SortMembers(out)
	return out, nil
}

// ---- invites ----

func (r *Repo) CreateInvite(i app.Invite) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.invites[i.ID] = i
	return nil
}

func (r *Repo) GetInvite(id string) (app.Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	i, ok := r.invites[id]
	if !ok {
		return app.Invite{}, app.ErrNotFound
	}
	return i, nil
}

func (r *Repo) UpdateInvite(i app.Invite) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.invites[i.ID]; !ok {
		return app.ErrNotFound
	}
	r.invites[i.ID] = i
	return nil
}

func (r *Repo) DeleteInvite(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.invites[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.invites, id)
	return nil
}

func (r *Repo) InvitesForBand(bandID string) ([]app.Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Invite
	for _, i := range r.invites {
		if i.BandID == bandID {
			out = append(out, i)
		}
	}
	return out, nil
}

func (r *Repo) PendingInvitesForIdentifiers(pairs []app.IdentifierMatch) ([]app.Invite, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Invite
	for _, i := range r.invites {
		if i.Status != app.InvitePending {
			continue
		}
		for _, p := range pairs {
			if i.IdentifierKind == p.Kind && i.Identifier == p.Identifier {
				out = append(out, i)
				break
			}
		}
	}
	return out, nil
}

// ---- invite links ----

func (r *Repo) CreateInviteLink(l app.InviteLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.inviteLinks[l.ID] = l
	return nil
}

func (r *Repo) GetInviteLink(id string) (app.InviteLink, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	l, ok := r.inviteLinks[id]
	if !ok {
		return app.InviteLink{}, app.ErrNotFound
	}
	return l, nil
}

func (r *Repo) GetInviteLinkByToken(token string) (app.InviteLink, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if token == "" {
		return app.InviteLink{}, app.ErrNotFound
	}
	for _, l := range r.inviteLinks {
		if l.Token == token {
			return l, nil
		}
	}
	return app.InviteLink{}, app.ErrNotFound
}

func (r *Repo) UpdateInviteLink(l app.InviteLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.inviteLinks[l.ID]; !ok {
		return app.ErrNotFound
	}
	r.inviteLinks[l.ID] = l
	return nil
}

func (r *Repo) InviteLinksForBand(bandID string) ([]app.InviteLink, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.InviteLink
	for _, l := range r.inviteLinks {
		if l.BandID == bandID {
			out = append(out, l)
		}
	}
	return out, nil
}

// ---- songs ----

func (r *Repo) CreateSong(s app.Song) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.songs[s.ID] = s
	return nil
}

func (r *Repo) GetSong(id string) (app.Song, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.songs[id]
	if !ok {
		return app.Song{}, app.ErrNotFound
	}
	return s, nil
}

func (r *Repo) UpdateSong(s app.Song) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.songs[s.ID]; !ok {
		return app.ErrNotFound
	}
	r.songs[s.ID] = s
	return nil
}

func (r *Repo) DeleteSong(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.songs[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.songs, id)
	return nil
}

func (r *Repo) SongsOfBand(bandID string) ([]app.Song, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Song
	for _, s := range r.songs {
		if s.BandID == bandID {
			out = append(out, s)
		}
	}
	app.SortSongs(out)
	return out, nil
}

// ---- song files ----

func (r *Repo) CreateSongFile(f app.SongFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.files[f.ID] = f
	return nil
}

func (r *Repo) GetSongFile(id string) (app.SongFile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.files[id]
	if !ok {
		return app.SongFile{}, app.ErrNotFound
	}
	return f, nil
}

func (r *Repo) UpdateSongFile(f app.SongFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.files[f.ID]; !ok {
		return app.ErrNotFound
	}
	r.files[f.ID] = f
	return nil
}

func (r *Repo) DeleteSongFile(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.files[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.files, id)
	return nil
}

func (r *Repo) FilesOfSong(songID string) ([]app.SongFile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.SongFile
	for _, f := range r.files {
		if f.SongID == songID {
			out = append(out, f)
		}
	}
	app.SortFiles(out)
	return out, nil
}

func (r *Repo) FilesWithBlob(blobHash string) ([]app.SongFile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.SongFile
	for _, f := range r.files {
		if f.BlobHash == blobHash {
			out = append(out, f)
		}
	}
	app.SortFiles(out)
	return out, nil
}

// ---- file selections (per-member, per-song) ----

func (r *Repo) GetFileSelection(userID, songID string) (app.FileSelection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sel, ok := r.selections[selectionKey(userID, songID)]
	if !ok {
		return app.FileSelection{}, app.ErrNotFound
	}
	return sel, nil
}

func (r *Repo) SetFileSelection(sel app.FileSelection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.selections[selectionKey(sel.UserID, sel.SongID)] = sel
	return nil
}

func (r *Repo) DeleteFileSelection(userID, songID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.selections, selectionKey(userID, songID))
	return nil
}

// ---- setlists ----

func (r *Repo) CreateSetlist(sl app.Setlist) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setlists[sl.ID] = sl
	return nil
}

func (r *Repo) GetSetlist(id string) (app.Setlist, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sl, ok := r.setlists[id]
	if !ok {
		return app.Setlist{}, app.ErrNotFound
	}
	return sl, nil
}

func (r *Repo) UpdateSetlist(sl app.Setlist) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.setlists[sl.ID]; !ok {
		return app.ErrNotFound
	}
	r.setlists[sl.ID] = sl
	return nil
}

func (r *Repo) DeleteSetlist(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.setlists[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.setlists, id)
	return nil
}

func (r *Repo) SetlistsOfBand(bandID string) ([]app.Setlist, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Setlist
	for _, sl := range r.setlists {
		if sl.BandID == bandID {
			out = append(out, sl)
		}
	}
	app.SortSetlists(out)
	return out, nil
}

func (r *Repo) CreateSetlistItem(it app.SetlistItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setlistItems[it.ID] = it
	return nil
}

func (r *Repo) GetSetlistItem(id string) (app.SetlistItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	it, ok := r.setlistItems[id]
	if !ok {
		return app.SetlistItem{}, app.ErrNotFound
	}
	return it, nil
}

func (r *Repo) UpdateSetlistItem(it app.SetlistItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.setlistItems[it.ID]; !ok {
		return app.ErrNotFound
	}
	r.setlistItems[it.ID] = it
	return nil
}

func (r *Repo) DeleteSetlistItem(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.setlistItems[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.setlistItems, id)
	return nil
}

func (r *Repo) ItemsOfSetlist(setlistID string) ([]app.SetlistItem, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.SetlistItem
	for _, it := range r.setlistItems {
		if it.SetlistID == setlistID {
			out = append(out, it)
		}
	}
	app.SortSetlistItems(out)
	return out, nil
}

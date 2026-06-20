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

	users    map[string]app.User       // id -> user
	sessions map[string]app.Session    // token -> session
	bands    map[string]app.Band       // id -> band
	members  map[string]app.Membership // bandID|userID -> membership
	invites  map[string]app.Invite     // id -> invite
	songs    map[string]app.Song       // id -> song
	files    map[string]app.SongFile   // id -> song file
}

// New returns an empty in-memory Repo.
func New() *Repo {
	return &Repo{
		users:    map[string]app.User{},
		sessions: map[string]app.Session{},
		bands:    map[string]app.Band{},
		members:  map[string]app.Membership{},
		invites:  map[string]app.Invite{},
		songs:    map[string]app.Song{},
		files:    map[string]app.SongFile{},
	}
}

var _ app.Repo = (*Repo)(nil)

func memberKey(bandID, userID string) string { return bandID + "|" + userID }

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

func (r *Repo) MembersOfBand(bandID string) ([]app.Membership, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Membership
	for _, m := range r.members {
		if m.BandID == bandID {
			out = append(out, m)
		}
	}
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

func (r *Repo) SongsOfBand(bandID string) ([]app.Song, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.Song
	for _, s := range r.songs {
		if s.BandID == bandID {
			out = append(out, s)
		}
	}
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

func (r *Repo) FilesOfSong(songID string) ([]app.SongFile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []app.SongFile
	for _, f := range r.files {
		if f.SongID == songID {
			out = append(out, f)
		}
	}
	return out, nil
}

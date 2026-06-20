// Package filerepo is a JSON-file-backed app.Repo: zero-infra persistence for
// local dev. The entire relational dataset is held in memory and flushed to a
// single JSON file under the data dir on every mutating call (atomic rename).
// This is intentionally simple — it is NOT the annotation store; the dataset is
// small (users/bands/songs metadata), so whole-file rewrites are fine.
//
// Boundary: imports app + stdlib only.
package filerepo

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"troubastack/core/internal/app"
)

// dataset is the on-disk shape. Maps mirror memrepo's keys.
type dataset struct {
	Users        map[string]app.User          `json:"users"`
	Sessions     map[string]app.Session       `json:"sessions"`
	Bands        map[string]app.Band          `json:"bands"`
	Members      map[string]app.Membership    `json:"members"`
	Invites      map[string]app.Invite        `json:"invites"`
	Songs        map[string]app.Song          `json:"songs"`
	Files        map[string]app.SongFile      `json:"files"`
	Selections   map[string]app.FileSelection `json:"selections"`
	Setlists     map[string]app.Setlist       `json:"setlists"`
	SetlistItems map[string]app.SetlistItem   `json:"setlistItems"`
}

// Repo persists the dataset to <dir>/app.json on every write.
type Repo struct {
	mu   sync.Mutex
	path string
	d    dataset
}

// New opens (or initializes) a file-backed Repo at dir/app.json. The dir is
// created if missing; an existing file is loaded.
func New(dir string) (*Repo, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("filerepo: mkdir: %w", err)
	}
	r := &Repo{
		path: filepath.Join(dir, "app.json"),
		d:    emptyDataset(),
	}
	if err := r.load(); err != nil {
		return nil, err
	}
	return r, nil
}

var _ app.Repo = (*Repo)(nil)

func emptyDataset() dataset {
	return dataset{
		Users:        map[string]app.User{},
		Sessions:     map[string]app.Session{},
		Bands:        map[string]app.Band{},
		Members:      map[string]app.Membership{},
		Invites:      map[string]app.Invite{},
		Songs:        map[string]app.Song{},
		Files:        map[string]app.SongFile{},
		Selections:   map[string]app.FileSelection{},
		Setlists:     map[string]app.Setlist{},
		SetlistItems: map[string]app.SetlistItem{},
	}
}

func (r *Repo) load() error {
	b, err := os.ReadFile(r.path)
	if os.IsNotExist(err) {
		return nil // fresh
	}
	if err != nil {
		return fmt.Errorf("filerepo: read: %w", err)
	}
	if len(b) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, &r.d); err != nil {
		return fmt.Errorf("filerepo: parse: %w", err)
	}
	// Guard against nil maps from a partial file.
	if r.d.Users == nil {
		r.d = emptyDataset()
	}
	// Files was added after the first releases; an older file lacks the key.
	if r.d.Files == nil {
		r.d.Files = map[string]app.SongFile{}
	}
	// Setlists + items were added later still; nil-guard for backward compatibility
	// with older app.json files that predate them.
	if r.d.Setlists == nil {
		r.d.Setlists = map[string]app.Setlist{}
	}
	if r.d.SetlistItems == nil {
		r.d.SetlistItems = map[string]app.SetlistItem{}
	}
	// Per-member file selections were added later still; nil-guard for older files.
	if r.d.Selections == nil {
		r.d.Selections = map[string]app.FileSelection{}
	}
	return nil
}

// flush writes the dataset atomically. Caller must hold r.mu.
func (r *Repo) flush() error {
	b, err := json.MarshalIndent(r.d, "", "  ")
	if err != nil {
		return fmt.Errorf("filerepo: marshal: %w", err)
	}
	tmp := r.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return fmt.Errorf("filerepo: write tmp: %w", err)
	}
	if err := os.Rename(tmp, r.path); err != nil {
		return fmt.Errorf("filerepo: rename: %w", err)
	}
	return nil
}

func memberKey(bandID, userID string) string { return bandID + "|" + userID }

func selectionKey(userID, songID string) string { return userID + "|" + songID }

// ---- users ----

func (r *Repo) CreateUser(u app.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, e := range r.d.Users {
		if strings.EqualFold(e.Username, u.Username) {
			return app.ErrConflict
		}
		if u.Email != "" && strings.EqualFold(e.Email, u.Email) {
			return app.ErrConflict
		}
	}
	r.d.Users[u.ID] = u
	return r.flush()
}

func (r *Repo) GetUser(id string) (app.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	u, ok := r.d.Users[id]
	if !ok {
		return app.User{}, app.ErrNotFound
	}
	return u, nil
}

func (r *Repo) GetUserByUsername(username string) (app.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, u := range r.d.Users {
		if strings.EqualFold(u.Username, username) {
			return u, nil
		}
	}
	return app.User{}, app.ErrNotFound
}

func (r *Repo) GetUserByEmail(email string) (app.User, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if email == "" {
		return app.User{}, app.ErrNotFound
	}
	for _, u := range r.d.Users {
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
	r.d.Sessions[s.Token] = s
	return r.flush()
}

func (r *Repo) GetSession(token string) (app.Session, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.d.Sessions[token]
	if !ok {
		return app.Session{}, app.ErrNotFound
	}
	return s, nil
}

func (r *Repo) DeleteSession(token string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Sessions[token]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Sessions, token)
	return r.flush()
}

// ---- bands & memberships ----

func (r *Repo) CreateBand(b app.Band) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.Bands[b.ID] = b
	return r.flush()
}

func (r *Repo) GetBand(id string) (app.Band, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	b, ok := r.d.Bands[id]
	if !ok {
		return app.Band{}, app.ErrNotFound
	}
	return b, nil
}

func (r *Repo) UpdateBand(b app.Band) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Bands[b.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.Bands[b.ID] = b
	return r.flush()
}

func (r *Repo) DeleteBand(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Bands[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Bands, id)
	return r.flush()
}

func (r *Repo) BandsForUser(userID string) ([]app.Band, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Band
	for _, m := range r.d.Members {
		if m.UserID == userID {
			if b, ok := r.d.Bands[m.BandID]; ok {
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
	if _, ok := r.d.Members[k]; ok {
		return app.ErrConflict
	}
	r.d.Members[k] = m
	return r.flush()
}

func (r *Repo) GetMembership(bandID, userID string) (app.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	m, ok := r.d.Members[memberKey(bandID, userID)]
	if !ok {
		return app.Membership{}, app.ErrNotFound
	}
	return m, nil
}

func (r *Repo) UpdateMembership(m app.Membership) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := memberKey(m.BandID, m.UserID)
	if _, ok := r.d.Members[k]; !ok {
		return app.ErrNotFound
	}
	r.d.Members[k] = m
	return r.flush()
}

func (r *Repo) DeleteMembership(bandID, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := memberKey(bandID, userID)
	if _, ok := r.d.Members[k]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Members, k)
	return r.flush()
}

func (r *Repo) MembersOfBand(bandID string) ([]app.Membership, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Membership
	for _, m := range r.d.Members {
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
	r.d.Invites[i.ID] = i
	return r.flush()
}

func (r *Repo) GetInvite(id string) (app.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	i, ok := r.d.Invites[id]
	if !ok {
		return app.Invite{}, app.ErrNotFound
	}
	return i, nil
}

func (r *Repo) UpdateInvite(i app.Invite) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Invites[i.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.Invites[i.ID] = i
	return r.flush()
}

func (r *Repo) DeleteInvite(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Invites[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Invites, id)
	return r.flush()
}

func (r *Repo) InvitesForBand(bandID string) ([]app.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Invite
	for _, i := range r.d.Invites {
		if i.BandID == bandID {
			out = append(out, i)
		}
	}
	return out, nil
}

func (r *Repo) PendingInvitesForIdentifiers(pairs []app.IdentifierMatch) ([]app.Invite, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Invite
	for _, i := range r.d.Invites {
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
	r.d.Songs[s.ID] = s
	return r.flush()
}

func (r *Repo) GetSong(id string) (app.Song, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.d.Songs[id]
	if !ok {
		return app.Song{}, app.ErrNotFound
	}
	return s, nil
}

func (r *Repo) UpdateSong(s app.Song) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Songs[s.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.Songs[s.ID] = s
	return r.flush()
}

func (r *Repo) DeleteSong(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Songs[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Songs, id)
	return r.flush()
}

func (r *Repo) SongsOfBand(bandID string) ([]app.Song, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Song
	for _, s := range r.d.Songs {
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
	r.d.Files[f.ID] = f
	return r.flush()
}

func (r *Repo) GetSongFile(id string) (app.SongFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.d.Files[id]
	if !ok {
		return app.SongFile{}, app.ErrNotFound
	}
	return f, nil
}

func (r *Repo) UpdateSongFile(f app.SongFile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Files[f.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.Files[f.ID] = f
	return r.flush()
}

func (r *Repo) DeleteSongFile(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Files[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Files, id)
	return r.flush()
}

func (r *Repo) FilesOfSong(songID string) ([]app.SongFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.SongFile
	for _, f := range r.d.Files {
		if f.SongID == songID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *Repo) FilesWithBlob(blobHash string) ([]app.SongFile, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.SongFile
	for _, f := range r.d.Files {
		if f.BlobHash == blobHash {
			out = append(out, f)
		}
	}
	return out, nil
}

// ---- file selections (per-member, per-song) ----

func (r *Repo) GetFileSelection(userID, songID string) (app.FileSelection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sel, ok := r.d.Selections[selectionKey(userID, songID)]
	if !ok {
		return app.FileSelection{}, app.ErrNotFound
	}
	return sel, nil
}

func (r *Repo) SetFileSelection(sel app.FileSelection) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.Selections[selectionKey(sel.UserID, sel.SongID)] = sel
	return r.flush()
}

func (r *Repo) DeleteFileSelection(userID, songID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := selectionKey(userID, songID)
	if _, ok := r.d.Selections[k]; !ok {
		return nil // idempotent: clearing an unset selection is a no-op
	}
	delete(r.d.Selections, k)
	return r.flush()
}

// ---- setlists ----

func (r *Repo) CreateSetlist(sl app.Setlist) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.Setlists[sl.ID] = sl
	return r.flush()
}

func (r *Repo) GetSetlist(id string) (app.Setlist, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sl, ok := r.d.Setlists[id]
	if !ok {
		return app.Setlist{}, app.ErrNotFound
	}
	return sl, nil
}

func (r *Repo) UpdateSetlist(sl app.Setlist) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Setlists[sl.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.Setlists[sl.ID] = sl
	return r.flush()
}

func (r *Repo) DeleteSetlist(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Setlists[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.Setlists, id)
	return r.flush()
}

func (r *Repo) SetlistsOfBand(bandID string) ([]app.Setlist, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.Setlist
	for _, sl := range r.d.Setlists {
		if sl.BandID == bandID {
			out = append(out, sl)
		}
	}
	return out, nil
}

func (r *Repo) CreateSetlistItem(it app.SetlistItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.SetlistItems[it.ID] = it
	return r.flush()
}

func (r *Repo) GetSetlistItem(id string) (app.SetlistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	it, ok := r.d.SetlistItems[id]
	if !ok {
		return app.SetlistItem{}, app.ErrNotFound
	}
	return it, nil
}

func (r *Repo) UpdateSetlistItem(it app.SetlistItem) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.SetlistItems[it.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.SetlistItems[it.ID] = it
	return r.flush()
}

func (r *Repo) DeleteSetlistItem(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.SetlistItems[id]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.SetlistItems, id)
	return r.flush()
}

func (r *Repo) ItemsOfSetlist(setlistID string) ([]app.SetlistItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.SetlistItem
	for _, it := range r.d.SetlistItems {
		if it.SetlistID == setlistID {
			out = append(out, it)
		}
	}
	return out, nil
}

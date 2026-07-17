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
	Users          map[string]app.User          `json:"users"`
	Sessions       map[string]app.Session       `json:"sessions"`
	PasswordResets map[string]app.PasswordReset `json:"passwordResets"`
	ChartSources   map[string]string            `json:"chartSources"`
	Bands          map[string]app.Band          `json:"bands"`
	Members        map[string]app.Membership    `json:"members"`
	Invites        map[string]app.Invite        `json:"invites"`
	InviteLinks    map[string]app.InviteLink    `json:"inviteLinks"`
	Songs          map[string]app.Song          `json:"songs"`
	Files          map[string]app.SongFile      `json:"files"`
	Selections     map[string]app.FileSelection `json:"selections"`
	SongCues       map[string]app.SongCues      `json:"songCues"`
	Setlists       map[string]app.Setlist       `json:"setlists"`
	SetlistItems   map[string]app.SetlistItem   `json:"setlistItems"`
}

// storedUser is the on-disk user record: app.User's API-visible fields PLUS the
// password hash. app.User tags PasswordHash json:"-" to keep it out of API
// responses — which ALSO kept it out of persistence, so a file-backed restart
// silently dropped every password and all logins then 401'd. We persist it
// explicitly. Embedding app.User means future user fields persist automatically.
type storedUser struct {
	app.User
	Hash string `json:"passwordHash"`
}

func toStored(u app.User) storedUser  { return storedUser{User: u, Hash: u.PasswordHash} }
func (s storedUser) toUser() app.User { u := s.User; u.PasswordHash = s.Hash; return u }

// MarshalJSON / UnmarshalJSON persist Users via storedUser so the password hash
// survives a flush/load round-trip; every other map serializes as-is. Without
// these, app.User.PasswordHash (json:"-") is dropped on write.
func (d dataset) MarshalJSON() ([]byte, error) {
	type plain dataset // a methodless alias → no recursion into MarshalJSON
	users := make(map[string]storedUser, len(d.Users))
	for id, u := range d.Users {
		users[id] = toStored(u)
	}
	return json.Marshal(struct {
		Users map[string]storedUser `json:"users"`
		plain
	}{Users: users, plain: plain(d)})
}

func (d *dataset) UnmarshalJSON(b []byte) error {
	*d = emptyDataset() // all maps non-nil, even for a partial/old file
	type plain dataset
	aux := struct {
		Users map[string]storedUser `json:"users"`
		*plain
	}{plain: (*plain)(d)}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	d.Users = make(map[string]app.User, len(aux.Users))
	for id, su := range aux.Users {
		d.Users[id] = su.toUser()
	}
	return nil
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
		Users:          map[string]app.User{},
		Sessions:       map[string]app.Session{},
		PasswordResets: map[string]app.PasswordReset{},
		ChartSources:   map[string]string{},
		Bands:          map[string]app.Band{},
		Members:        map[string]app.Membership{},
		Invites:        map[string]app.Invite{},
		InviteLinks:    map[string]app.InviteLink{},
		Songs:          map[string]app.Song{},
		Files:          map[string]app.SongFile{},
		Selections:     map[string]app.FileSelection{},
		SongCues:       map[string]app.SongCues{},
		Setlists:       map[string]app.Setlist{},
		SetlistItems:   map[string]app.SetlistItem{},
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
	// Per-member song cues (T50) were added later still; nil-guard for older files.
	if r.d.SongCues == nil {
		r.d.SongCues = map[string]app.SongCues{}
	}
	// Invite links were added later still; nil-guard for older files.
	if r.d.InviteLinks == nil {
		r.d.InviteLinks = map[string]app.InviteLink{}
	}
	// Password resets (T21) were added later still; nil-guard for older files.
	if r.d.PasswordResets == nil {
		r.d.PasswordResets = map[string]app.PasswordReset{}
	}
	// Chart sources (T19) were added later still; nil-guard for older files.
	if r.d.ChartSources == nil {
		r.d.ChartSources = map[string]string{}
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

func (r *Repo) UpdateUser(u app.User) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.Users[u.ID]; !ok {
		return app.ErrNotFound
	}
	for id, e := range r.d.Users {
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
	r.d.Users[u.ID] = u
	return r.flush()
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

func (r *Repo) DeleteSessionsForUser(userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for token, s := range r.d.Sessions {
		if s.UserID == userID {
			delete(r.d.Sessions, token)
		}
	}
	return r.flush()
}

// ---- password resets ----

func (r *Repo) CreatePasswordReset(pr app.PasswordReset) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.PasswordResets[pr.TokenHash] = pr
	return r.flush()
}

func (r *Repo) GetPasswordReset(tokenHash string) (app.PasswordReset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	pr, ok := r.d.PasswordResets[tokenHash]
	if !ok {
		return app.PasswordReset{}, app.ErrNotFound
	}
	return pr, nil
}

func (r *Repo) DeletePasswordReset(tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.PasswordResets[tokenHash]; !ok {
		return app.ErrNotFound
	}
	delete(r.d.PasswordResets, tokenHash)
	return r.flush()
}

// ---- chart sources ----

func (r *Repo) SetChartSource(fileID, source string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.ChartSources[fileID] = source
	return r.flush()
}

func (r *Repo) GetChartSource(fileID string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	src, ok := r.d.ChartSources[fileID]
	if !ok {
		return "", app.ErrNotFound
	}
	return src, nil
}

func (r *Repo) DeleteChartSource(fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.ChartSources[fileID]; !ok {
		return nil // idempotent
	}
	delete(r.d.ChartSources, fileID)
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
	app.SortBands(out)
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
	app.SortMembers(out)
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
	app.SortInvites(out) // user-visible band panel — stable order (T22)
	return out, nil
}

// PendingInvitesForIdentifiers returns matches in unspecified order — it feeds an
// internal existence/dedup check at invite time, never a user-facing list, so
// order is irrelevant (T22).
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

// ---- invite links ----

func (r *Repo) CreateInviteLink(l app.InviteLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.InviteLinks[l.ID] = l
	return r.flush()
}

func (r *Repo) GetInviteLink(id string) (app.InviteLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	l, ok := r.d.InviteLinks[id]
	if !ok {
		return app.InviteLink{}, app.ErrNotFound
	}
	return l, nil
}

func (r *Repo) GetInviteLinkByToken(token string) (app.InviteLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if token == "" {
		return app.InviteLink{}, app.ErrNotFound
	}
	for _, l := range r.d.InviteLinks {
		if l.Token == token {
			return l, nil
		}
	}
	return app.InviteLink{}, app.ErrNotFound
}

func (r *Repo) UpdateInviteLink(l app.InviteLink) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.d.InviteLinks[l.ID]; !ok {
		return app.ErrNotFound
	}
	r.d.InviteLinks[l.ID] = l
	return r.flush()
}

func (r *Repo) InviteLinksForBand(bandID string) ([]app.InviteLink, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []app.InviteLink
	for _, l := range r.d.InviteLinks {
		if l.BandID == bandID {
			out = append(out, l)
		}
	}
	app.SortInviteLinks(out) // user-visible band panel — stable order (T22)
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
	app.SortSongs(out)
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
	app.SortFiles(out)
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
	app.SortFiles(out)
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

// ---- song cues (per-member, per-song) ----

func (r *Repo) GetSongCues(userID, songID string) (app.SongCues, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	sc, ok := r.d.SongCues[selectionKey(userID, songID)]
	if !ok {
		return app.SongCues{}, app.ErrNotFound
	}
	return sc, nil
}

func (r *Repo) SetSongCues(sc app.SongCues) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.d.SongCues[selectionKey(sc.UserID, sc.SongID)] = sc
	return r.flush()
}

func (r *Repo) DeleteSongCues(userID, songID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := selectionKey(userID, songID)
	if _, ok := r.d.SongCues[k]; !ok {
		return nil // idempotent: clearing unset cues is a no-op
	}
	delete(r.d.SongCues, k)
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
	app.SortSetlists(out)
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
	app.SortSetlistItems(out)
	return out, nil
}

package app

// Band export / import (T62): one zip carries a COMPLETE band out of a server and back
// into one (backup, migration, band-moves-servers). No versioning machinery beyond a
// single formatVersion integer — incompatibilities are a future human's problem, per
// VLL's ruling. Baked concerts are NOT included (rebake on the target).
//
// Layout:
//
//	band.json          the whole relational graph (one manifest)
//	blobs/<sha256>     unique file bytes, content-addressed (dedup by BlobHash)
//
// Identity: members match by username on the target, else are created with no usable
// password (the admin hands out credentials via the T21 reset flow). Relational IDs are
// re-minted on import; annotation layer ids + object uuids are KEPT (each song gets a
// fresh engine keyed by its new id), with Layer.FileID rewritten through the file map
// and Layer/Object OwnerID through the member map (the SharedOwner sentinel passes
// through unchanged). Import is ALL-OR-NOTHING: the whole manifest is validated before a
// single row is written.

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"troubastack/core/internal/domain"
	"troubastack/core/internal/engine"

	"troubastack/core/internal/app/blob"
)

// BandExportFormatVersion is the ENTIRE versioning story: the importer rejects any other
// value with a 400 (no migration code — a future human resolves incompatibilities).
const BandExportFormatVersion = 1

// MaxImportBytes caps an uploaded band zip (the per-song-file maxUploadBytes is far too
// small for a whole band).
const MaxImportBytes = 512 << 20

// --- manifest shapes (band.json) -----------------------------------------------------

type bandManifest struct {
	FormatVersion  int                       `json:"formatVersion"`
	ExportedAt     string                    `json:"exportedAt"`
	Band           manifestBand              `json:"band"`
	Members        []manifestMember          `json:"members"`
	Songs          []manifestSong            `json:"songs"`
	Setlists       []manifestSetlist         `json:"setlists"`
	FileSelections []manifestSelection       `json:"fileSelections,omitempty"`
	SongCues       []manifestCue             `json:"songCues,omitempty"`
	Annotations    map[string]manifestAnnots `json:"annotations,omitempty"` // songRef → head
}

type manifestBand struct {
	Name string `json:"name"`
}

// manifestMember carries the orig member id (the key for the OwnerID rewrite) plus the
// projection needed to match-or-create the account. NEVER the password hash.
type manifestMember struct {
	ID          string     `json:"id"` // original server UUID — the OwnerID-rewrite key
	Username    string     `json:"username"`
	DisplayName string     `json:"displayName"`
	Email       string     `json:"email,omitempty"`
	AvatarKind  AvatarKind `json:"avatarKind,omitempty"`
	Role        Role       `json:"role"`
}

type manifestSong struct {
	ID     string         `json:"id"` // original songID — the manifest-internal ref
	Title  string         `json:"title"`
	Artist string         `json:"artist,omitempty"`
	Key    string         `json:"key,omitempty"`
	Tempo  int            `json:"tempo,omitempty"`
	Tags   []string       `json:"tags,omitempty"`
	Notes  string         `json:"notes,omitempty"`
	Files  []manifestFile `json:"files"`
}

type manifestFile struct {
	ID           string `json:"id"` // original fileID — the manifest-internal ref
	Filename     string `json:"filename"`
	ContentType  string `json:"contentType"`
	Size         int64  `json:"size"`
	BlobHash     string `json:"blobHash"`
	DisplayOrder int    `json:"displayOrder"`
	Generated    bool   `json:"generated,omitempty"`
	Revision     int    `json:"revision,omitempty"`
	ChartSource  string `json:"chartSource,omitempty"`
}

type manifestSetlist struct {
	Name      string         `json:"name"`
	EventDate string         `json:"eventDate,omitempty"`
	Venue     string         `json:"venue,omitempty"`
	Notes     string         `json:"notes,omitempty"`
	Items     []manifestItem `json:"items"`
}

type manifestItem struct {
	SongRef         string `json:"songRef"` // original songID
	Position        int    `json:"position"`
	KeyOverride     string `json:"keyOverride,omitempty"`
	TempoOverride   int    `json:"tempoOverride,omitempty"`
	Notes           string `json:"notes,omitempty"`
	OnCall          bool   `json:"onCall,omitempty"`
	TransposeChords bool   `json:"transposeChords,omitempty"`
}

type manifestSelection struct {
	MemberUsername string   `json:"memberUsername"`
	SongRef        string   `json:"songRef"`
	FileRefs       []string `json:"fileRefs"` // original fileIDs
}

type manifestCue struct {
	MemberUsername string    `json:"memberUsername"`
	SongRef        string    `json:"songRef"`
	Cues           []SongCue `json:"cues"`
}

// manifestAnnots is the engine HEAD for one song — the same domain types both ends, so
// annotations round-trip by construction. Tombstones are dropped (head-only, no history).
type manifestAnnots struct {
	Layers  []domain.Layer  `json:"layers"`
	Objects []domain.Object `json:"objects"`
}

// ImportReport is returned to the importer: the new band + which member accounts were
// reused vs freshly created, plus counts.
type ImportReport struct {
	Band     Band     `json:"band"`
	Matched  []string `json:"matched"` // usernames of pre-existing accounts attached
	Created  []string `json:"created"` // usernames of accounts created (need a reset)
	Songs    int      `json:"songs"`
	Files    int      `json:"files"`
	Setlists int      `json:"setlists"`
}

// --- export --------------------------------------------------------------------------

// ExportBand serializes a whole band to a .tband zip. Admin-only (the zip carries every
// member's email + personal cues/selections).
func (s *Service) ExportBand(caller User, eng *engine.Engine, bandID string) ([]byte, string, error) {
	band, role, err := s.GetBand(caller, bandID)
	if err != nil {
		return nil, "", err
	}
	if role != RoleAdmin {
		return nil, "", ErrForbidden
	}

	man := bandManifest{
		FormatVersion: BandExportFormatVersion,
		ExportedAt:    s.now().UTC().Format(time.RFC3339),
		Band:          manifestBand{Name: band.Name},
		Annotations:   map[string]manifestAnnots{},
	}
	blobs := map[string][]byte{} // hash → bytes (dedup)

	// Members.
	members, err := s.Members(caller, bandID)
	if err != nil {
		return nil, "", err
	}
	for _, m := range members {
		man.Members = append(man.Members, manifestMember{
			ID: m.User.ID, Username: m.User.Username, DisplayName: m.User.DisplayName,
			Email: m.User.Email, AvatarKind: m.User.AvatarKind, Role: m.Role,
		})
	}

	// Songs + files (+ blobs, chart source) + per-member selections/cues + annotations.
	songs, err := s.repo.SongsOfBand(bandID)
	if err != nil {
		return nil, "", err
	}
	for _, song := range songs {
		ms := manifestSong{
			ID: song.ID, Title: song.Title, Artist: song.Artist, Key: song.Key,
			Tempo: song.Tempo, Tags: song.Tags, Notes: song.Notes,
		}
		files, err := s.repo.FilesOfSong(song.ID)
		if err != nil {
			return nil, "", err
		}
		for _, f := range files {
			mf := manifestFile{
				ID: f.ID, Filename: f.Filename, ContentType: f.ContentType, Size: f.Size,
				BlobHash: f.BlobHash, DisplayOrder: f.DisplayOrder, Generated: f.Generated, Revision: f.Revision,
			}
			if f.BlobHash != "" {
				if _, ok := blobs[f.BlobHash]; !ok {
					data, berr := s.blobs.Get(f.BlobHash)
					if berr != nil {
						return nil, "", fmt.Errorf("export: blob %s: %w", f.BlobHash, berr)
					}
					blobs[f.BlobHash] = data
				}
			}
			if f.Generated {
				if src, serr := s.repo.GetChartSource(f.ID); serr == nil {
					mf.ChartSource = src
				}
			}
			ms.Files = append(ms.Files, mf)
		}
		man.Songs = append(man.Songs, ms)

		// Annotations (head only; drop tombstones).
		snap, err := eng.Head(song.ID)
		if err != nil {
			return nil, "", err
		}
		if len(snap.Layers) > 0 || len(snap.Objects) > 0 {
			ann := manifestAnnots{Layers: snap.Layers}
			for _, o := range snap.Objects {
				if !o.Deleted {
					ann.Objects = append(ann.Objects, o)
				}
			}
			man.Annotations[song.ID] = ann
		}

		// Per-member personal layers: file selections + song cues.
		for _, m := range members {
			if sel, err := s.repo.GetFileSelection(m.User.ID, song.ID); err == nil {
				man.FileSelections = append(man.FileSelections, manifestSelection{
					MemberUsername: m.User.Username, SongRef: song.ID, FileRefs: sel.FileIDs,
				})
			}
			if sc, err := s.repo.GetSongCues(m.User.ID, song.ID); err == nil && len(sc.Cues) > 0 {
				man.SongCues = append(man.SongCues, manifestCue{
					MemberUsername: m.User.Username, SongRef: song.ID, Cues: sc.Cues,
				})
			}
		}
	}

	// Setlists + items.
	setlists, err := s.repo.SetlistsOfBand(bandID)
	if err != nil {
		return nil, "", err
	}
	for _, sl := range setlists {
		msl := manifestSetlist{Name: sl.Name, EventDate: sl.EventDate, Venue: sl.Venue, Notes: sl.Notes}
		items, err := s.repo.ItemsOfSetlist(sl.ID)
		if err != nil {
			return nil, "", err
		}
		for _, it := range items {
			msl.Items = append(msl.Items, manifestItem{
				SongRef: it.SongID, Position: it.Position, KeyOverride: it.KeyOverride,
				TempoOverride: it.TempoOverride, Notes: it.Notes, OnCall: it.OnCall, TransposeChords: it.TransposeChords,
			})
		}
		man.Setlists = append(man.Setlists, msl)
	}

	// Zip it: band.json + blobs/<hash>.
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	manBytes, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if w, err := zw.Create("band.json"); err != nil {
		return nil, "", err
	} else if _, err := w.Write(manBytes); err != nil {
		return nil, "", err
	}
	// Deterministic blob order.
	hashes := make([]string, 0, len(blobs))
	for h := range blobs {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	for _, h := range hashes {
		w, err := zw.Create("blobs/" + h)
		if err != nil {
			return nil, "", err
		}
		if _, err := w.Write(blobs[h]); err != nil {
			return nil, "", err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, "", err
	}
	filename := sanitizeFilename(band.Name) + "-" + s.now().UTC().Format("2006-01-02") + ".tband.zip"
	return buf.Bytes(), filename, nil
}

func sanitizeFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "band"
	}
	repl := func(r rune) rune {
		switch r {
		case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
			return '-'
		}
		return r
	}
	return strings.Map(repl, name)
}

// --- import --------------------------------------------------------------------------

// ImportBand creates a NEW band from a .tband zip, owned by (and admin for) the caller.
// All-or-nothing: the whole manifest is validated before anything is written.
func (s *Service) ImportBand(caller User, eng *engine.Engine, zipBytes []byte) (ImportReport, error) {
	man, blobs, err := parseBandZip(zipBytes)
	if err != nil {
		return ImportReport{}, err
	}
	if man.FormatVersion != BandExportFormatVersion {
		return ImportReport{}, fmt.Errorf("%w: unsupported band export formatVersion %d (this server reads %d)", ErrInvalidInput, man.FormatVersion, BandExportFormatVersion)
	}

	// ---- validate EVERYTHING first (all-or-nothing) ----
	songIDs := map[string]bool{}    // orig songID
	fileToSong := map[string]bool{} // orig fileID present
	fileIDs := map[string]bool{}
	for _, sg := range man.Songs {
		songIDs[sg.ID] = true
		for _, f := range sg.Files {
			fileIDs[f.ID] = true
			fileToSong[f.ID] = true
			if f.BlobHash != "" {
				data, ok := blobs[f.BlobHash]
				if !ok {
					return ImportReport{}, fmt.Errorf("%w: file %q references blob %s not present in the archive", ErrInvalidInput, f.Filename, f.BlobHash)
				}
				if blob.HashOf(data) != f.BlobHash {
					return ImportReport{}, fmt.Errorf("%w: blob %s content does not match its hash", ErrInvalidInput, f.BlobHash)
				}
			}
		}
	}
	memberByUsername := map[string]manifestMember{}
	memberByOrigID := map[string]manifestMember{}
	for _, m := range man.Members {
		if m.Username == "" {
			return ImportReport{}, fmt.Errorf("%w: a member has no username", ErrInvalidInput)
		}
		// A would-create member (username absent here) whose email belongs to a
		// DIFFERENT existing account would fail at CreateUser mid-import, after the
		// band + earlier rows are written — breaking all-or-nothing. Pre-validate it
		// against target-server state so nothing is written on conflict.
		if _, err := s.repo.GetUserByUsername(m.Username); err != nil && m.Email != "" {
			if existing, err := s.repo.GetUserByEmail(m.Email); err == nil {
				return ImportReport{}, fmt.Errorf("%w: member %q would be created, but its email %q already belongs to a different account (%q) on this server", ErrInvalidInput, m.Username, m.Email, existing.Username)
			}
		}
		memberByUsername[m.Username] = m
		memberByOrigID[m.ID] = m
	}
	for _, sl := range man.Setlists {
		for _, it := range sl.Items {
			if !songIDs[it.SongRef] {
				return ImportReport{}, fmt.Errorf("%w: setlist %q item references unknown song %s", ErrInvalidInput, sl.Name, it.SongRef)
			}
		}
	}
	for _, sel := range man.FileSelections {
		if _, ok := memberByUsername[sel.MemberUsername]; !ok {
			return ImportReport{}, fmt.Errorf("%w: file selection references unknown member %q", ErrInvalidInput, sel.MemberUsername)
		}
		if !songIDs[sel.SongRef] {
			return ImportReport{}, fmt.Errorf("%w: file selection references unknown song %s", ErrInvalidInput, sel.SongRef)
		}
		for _, fr := range sel.FileRefs {
			if !fileIDs[fr] {
				return ImportReport{}, fmt.Errorf("%w: file selection references unknown file %s", ErrInvalidInput, fr)
			}
		}
	}
	for _, c := range man.SongCues {
		if _, ok := memberByUsername[c.MemberUsername]; !ok {
			return ImportReport{}, fmt.Errorf("%w: song cues reference unknown member %q", ErrInvalidInput, c.MemberUsername)
		}
		if !songIDs[c.SongRef] {
			return ImportReport{}, fmt.Errorf("%w: song cues reference unknown song %s", ErrInvalidInput, c.SongRef)
		}
	}
	for songRef, ann := range man.Annotations {
		if !songIDs[songRef] {
			return ImportReport{}, fmt.Errorf("%w: annotations reference unknown song %s", ErrInvalidInput, songRef)
		}
		for _, l := range ann.Layers {
			if l.FileID != "" && !fileIDs[l.FileID] {
				return ImportReport{}, fmt.Errorf("%w: annotation layer references unknown file %s", ErrInvalidInput, l.FileID)
			}
			if l.OwnerID != "" && l.OwnerID != domain.SharedOwner {
				if _, ok := memberByOrigID[l.OwnerID]; !ok {
					return ImportReport{}, fmt.Errorf("%w: annotation layer owner %s is not a member in the manifest", ErrInvalidInput, l.OwnerID)
				}
			}
		}
	}

	// ---- create (validation passed) ----
	now := s.now().UTC()
	band := Band{ID: s.newID(), Name: strings.TrimSpace(man.Band.Name), OwnerID: caller.ID, CreatedAt: now}
	if band.Name == "" {
		band.Name = "Imported band"
	}
	if err := s.repo.CreateBand(band); err != nil {
		return ImportReport{}, err
	}

	// Non-nil slices so the JSON always has arrays (never null) — the UI reads .length.
	report := ImportReport{Band: band, Matched: []string{}, Created: []string{}}

	// Members: match by username else create; build orig-id → new-user-id map.
	memberMap := map[string]string{} // orig member UUID → new user id
	usernameToNewID := map[string]string{}
	for _, m := range man.Members {
		var uid string
		if existing, err := s.repo.GetUserByUsername(m.Username); err == nil {
			uid = existing.ID
			report.Matched = append(report.Matched, m.Username)
		} else {
			u := User{
				ID: s.newID(), Username: m.Username, DisplayName: m.DisplayName,
				Email: m.Email, AvatarKind: m.AvatarKind, PasswordHash: "", CreatedAt: now,
			}
			if err := s.repo.CreateUser(u); err != nil {
				return ImportReport{}, err
			}
			uid = u.ID
			report.Created = append(report.Created, m.Username)
		}
		memberMap[m.ID] = uid
		usernameToNewID[m.Username] = uid
		// Add membership with the manifest role — unless this is the importer (handled below).
		if uid != caller.ID {
			if err := s.repo.AddMembership(Membership{BandID: band.ID, UserID: uid, Role: m.Role, CreatedAt: now}); err != nil {
				return ImportReport{}, err
			}
		}
	}
	// The importer is always an admin member + the owner, whether or not in the manifest.
	if err := s.repo.AddMembership(Membership{BandID: band.ID, UserID: caller.ID, Role: RoleAdmin, CreatedAt: now}); err != nil {
		return ImportReport{}, err
	}

	// Songs + files + chart source + annotations.
	songMap := map[string]string{} // orig songID → new songID
	fileMap := map[string]string{} // orig fileID → new fileID
	for _, sg := range man.Songs {
		ns := Song{
			ID: s.newID(), BandID: band.ID, Title: sg.Title, Artist: sg.Artist, Key: sg.Key,
			Tempo: sg.Tempo, Tags: sg.Tags, Notes: sg.Notes, CreatedAt: now,
		}
		if err := s.repo.CreateSong(ns); err != nil {
			return ImportReport{}, err
		}
		songMap[sg.ID] = ns.ID
		report.Songs++
		for _, f := range sg.Files {
			hash := f.BlobHash
			if data, ok := blobs[f.BlobHash]; ok {
				h, err := s.blobs.Put(data) // content-addressed → same hash
				if err != nil {
					return ImportReport{}, err
				}
				hash = h
			}
			nf := SongFile{
				ID: s.newID(), SongID: ns.ID, BandID: band.ID, Filename: f.Filename, ContentType: f.ContentType,
				Size: f.Size, BlobHash: hash, DisplayOrder: f.DisplayOrder, UploadedBy: caller.ID, CreatedAt: now,
				Generated: f.Generated, Revision: f.Revision,
			}
			if err := s.repo.CreateSongFile(nf); err != nil {
				return ImportReport{}, err
			}
			fileMap[f.ID] = nf.ID
			report.Files++
			if f.Generated && f.ChartSource != "" {
				if err := s.repo.SetChartSource(nf.ID, f.ChartSource); err != nil {
					return ImportReport{}, err
				}
			}
		}
	}

	// Annotations: fresh engine per new songID; rewrite Layer.FileID + Layer/Object OwnerID.
	remapOwner := func(orig string) string {
		if orig == "" || orig == domain.SharedOwner {
			return orig
		}
		if nid, ok := memberMap[orig]; ok {
			return nid
		}
		return orig // validated to exist; defensive
	}
	for songRef, ann := range man.Annotations {
		newSongID := songMap[songRef]
		for _, l := range ann.Layers {
			nl := l
			nl.FileID = fileMap[l.FileID]
			nl.OwnerID = remapOwner(l.OwnerID)
			if _, err := eng.Apply(newSongID, domain.Mutation{Kind: domain.KindLayerCreate, Layer: &nl, AuthorID: caller.ID, Summary: "import layer " + nl.ID}); err != nil {
				return ImportReport{}, fmt.Errorf("import annotations: %v", err)
			}
		}
		for _, o := range ann.Objects {
			no := o
			no.OwnerID = remapOwner(o.OwnerID)
			if no.Version == 0 {
				no.Version = 1
			}
			if _, err := eng.Apply(newSongID, domain.Mutation{Kind: domain.KindCreate, UUID: no.UUID, Object: &no, AuthorID: caller.ID, Summary: "import object " + no.UUID}); err != nil {
				return ImportReport{}, fmt.Errorf("import annotations: %v", err)
			}
		}
	}

	// Per-member selections + cues (remap song + file refs, keyed by username).
	for _, sel := range man.FileSelections {
		newFiles := make([]string, 0, len(sel.FileRefs))
		for _, fr := range sel.FileRefs {
			newFiles = append(newFiles, fileMap[fr])
		}
		if err := s.repo.SetFileSelection(FileSelection{UserID: usernameToNewID[sel.MemberUsername], SongID: songMap[sel.SongRef], FileIDs: newFiles}); err != nil {
			return ImportReport{}, err
		}
	}
	for _, c := range man.SongCues {
		if err := s.repo.SetSongCues(SongCues{UserID: usernameToNewID[c.MemberUsername], SongID: songMap[c.SongRef], Cues: c.Cues}); err != nil {
			return ImportReport{}, err
		}
	}

	// Setlists + items.
	for _, sl := range man.Setlists {
		nsl := Setlist{ID: s.newID(), BandID: band.ID, Name: sl.Name, EventDate: sl.EventDate, Venue: sl.Venue, Notes: sl.Notes, CreatedAt: now}
		if err := s.repo.CreateSetlist(nsl); err != nil {
			return ImportReport{}, err
		}
		report.Setlists++
		for _, it := range sl.Items {
			ni := SetlistItem{
				ID: s.newID(), SetlistID: nsl.ID, SongID: songMap[it.SongRef], Position: it.Position,
				KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride, Notes: it.Notes,
				OnCall: it.OnCall, TransposeChords: it.TransposeChords,
			}
			if err := s.repo.CreateSetlistItem(ni); err != nil {
				return ImportReport{}, err
			}
		}
	}

	return report, nil
}

// parseBandZip reads band.json + the blobs/ entries from an uploaded zip.
func parseBandZip(zipBytes []byte) (bandManifest, map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(zipBytes), int64(len(zipBytes)))
	if err != nil {
		return bandManifest{}, nil, fmt.Errorf("%w: not a valid zip archive", ErrInvalidInput)
	}
	var man bandManifest
	haveManifest := false
	blobs := map[string][]byte{}
	for _, f := range zr.File {
		if f.Name == "band.json" {
			rc, err := f.Open()
			if err != nil {
				return bandManifest{}, nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return bandManifest{}, nil, err
			}
			if err := json.Unmarshal(data, &man); err != nil {
				return bandManifest{}, nil, fmt.Errorf("%w: band.json is not valid JSON: %v", ErrInvalidInput, err)
			}
			haveManifest = true
		} else if strings.HasPrefix(f.Name, "blobs/") && !strings.HasSuffix(f.Name, "/") {
			rc, err := f.Open()
			if err != nil {
				return bandManifest{}, nil, err
			}
			data, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return bandManifest{}, nil, err
			}
			blobs[strings.TrimPrefix(f.Name, "blobs/")] = data
		}
	}
	if !haveManifest {
		return bandManifest{}, nil, fmt.Errorf("%w: archive has no band.json", ErrInvalidInput)
	}
	return man, blobs, nil
}

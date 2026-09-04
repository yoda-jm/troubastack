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
// small for a whole band). This bounds the COMPRESSED upload; the decompressed total is
// bounded separately (maxDecompressedBytes) so a small zip can't inflate to exhaust memory.
const MaxImportBytes = 512 << 20

// Zip-bomb bounds (T63). vars, not consts, only so tests can dial them low; production
// never mutates them.
var (
	// maxImportEntries caps how many entries a .tband may contain — a bomb of millions of
	// tiny entries is as dangerous as one huge entry.
	maxImportEntries = 100_000
	// maxDecompressedBytes caps the RUNNING TOTAL decompressed across all entries. Enforced
	// DURING extraction (zip headers can under-report), so a high-ratio DEFLATE bomb is
	// refused before it fills memory.
	maxDecompressedBytes = int64(MaxImportBytes)
)

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
	Meter  string         `json:"meter,omitempty"` // T86 — must ride here or a band export silently drops it
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
	Invited  []string `json:"invited"` // usernames given a pending invite (T63)
	Skipped  []string `json:"skipped"` // usernames neither created nor invited (T63)
	Songs    int      `json:"songs"`
	Files    int      `json:"files"`
	Setlists int      `json:"setlists"`
	// Personal content dropped because its owner was invited/skipped (T63). Shared and
	// conductor content is never dropped.
	DroppedLayers     int `json:"droppedLayers,omitempty"`
	DroppedObjects    int `json:"droppedObjects,omitempty"`
	DroppedCues       int `json:"droppedCues,omitempty"`
	DroppedSelections int `json:"droppedSelections,omitempty"`
}

// ImportDisposition is what import does (T63) with a manifest member the importer must
// decide about. SECURITY: a member whose username already belongs to a DIFFERENT account
// on this server is CONSENT-REQUIRED — only invite or skip; it is NEVER silently attached
// (the T62 account-takeover fix). A new username may be created, invited, or skipped.
type ImportDisposition string

const (
	DispositionCreate ImportDisposition = "create" // mint a passwordless account (new usernames only)
	DispositionInvite ImportDisposition = "invite" // create a pending invite; drop their personal content
	DispositionSkip   ImportDisposition = "skip"   // neither create nor invite; drop their personal content
)

func validDisposition(d ImportDisposition) bool {
	switch d {
	case DispositionCreate, DispositionInvite, DispositionSkip:
		return true
	}
	return false
}

// PreviewMember is one manifest member classified against the target server (T63). The
// importer (IsCaller) is always attached as the band admin. An Existing account (already
// on this server, not the importer) is consent-required — invite or skip only. A new
// username may be created (default), invited, or skipped.
type PreviewMember struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        Role   `json:"role"`
	Existing    bool   `json:"existing"`
	IsCaller    bool   `json:"isCaller"`
}

// ImportPreview classifies a .tband's members against the target server WITHOUT writing
// anything (T63), plus the band name + content counts. The importer dispositions each
// non-caller member (allowed choices depend on Existing).
type ImportPreview struct {
	BandName string          `json:"bandName"`
	Members  []PreviewMember `json:"members"`
	Songs    int             `json:"songs"`
	Files    int             `json:"files"`
	Setlists int             `json:"setlists"`
}

// classifyMembers projects each manifest member against the target server + caller: is
// the username already an account here, and is it the importer? Shared by preview + import
// so the classification (and thus the security rules) can never drift between them.
func (s *Service) classifyMembers(caller User, man bandManifest) []PreviewMember {
	out := make([]PreviewMember, 0, len(man.Members))
	for _, m := range man.Members {
		pm := PreviewMember{Username: m.Username, DisplayName: m.DisplayName, Role: m.Role}
		if existing, err := s.repo.GetUserByUsername(m.Username); err == nil {
			pm.Existing = true
			pm.IsCaller = existing.ID == caller.ID
		}
		out = append(out, pm)
	}
	return out
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
			Tempo: song.Tempo, Meter: song.Meter, Tags: song.Tags, Notes: song.Notes,
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
			// The file bytes are fetched by marshalV2 via s.blobs.Get and written under `<slug>/<filename>`
			// (amendment 4 — no blobs/); here we only carry the metadata + integrity hash.
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

	// Serialize as the v2 folder format (T134): band.json + repertoire.json + setlists.json +
	// annotations/<slug>.json + cues.json + the file bytes under `<slug>/<filename>` (amendment 4 — no
	// blobs/). We WRITE v2 always; the reader still accepts v1 (parseBandZip). The in-memory manifest
	// gathered above is the shared shape both the v1 and v2 writers/readers translate through.
	v2files, err := marshalV2(man, s.blobs.Get)
	if err != nil {
		return nil, "", err
	}
	zipBytes, err := writeV2Zip(v2files)
	if err != nil {
		return nil, "", err
	}
	filename := sanitizeFilename(band.Name) + "-" + s.now().UTC().Format("2006-01-02") + ".tband.zip"
	return zipBytes, filename, nil
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

// validateImport runs the full all-or-nothing manifest validation (T62): formatVersion,
// blob presence + hash, every ref resolvable, annotation-layer owners present in the
// manifest, and — against target-server state — that a would-create member's email does
// not collide with a different existing account. Writes nothing; shared by import +
// import-preview (T63) so the rules can never drift between them.
func (s *Service) validateImport(man bandManifest, blobs map[string][]byte) error {
	// The formatVersion gate lives in parseBandZip (which dispatches v1/v2 and rejects the
	// rest with a 400 before this runs). This belt-and-suspenders check keeps validateImport
	// self-contained — it is shared by import + preview, both of which parse first — and
	// accepts exactly the two versions the reader produces.
	if man.FormatVersion != BandExportFormatVersion && man.FormatVersion != BandExportFormatVersionV2 {
		return fmt.Errorf("%w: unsupported band export formatVersion %d (this server reads %d and %d)", ErrInvalidInput, man.FormatVersion, BandExportFormatVersion, BandExportFormatVersionV2)
	}
	songIDs := map[string]bool{} // orig songID
	fileIDs := map[string]bool{}
	for _, sg := range man.Songs {
		songIDs[sg.ID] = true
		for _, f := range sg.Files {
			fileIDs[f.ID] = true
			if f.BlobHash != "" {
				data, ok := blobs[f.BlobHash]
				if !ok {
					return fmt.Errorf("%w: file %q references blob %s not present in the archive", ErrInvalidInput, f.Filename, f.BlobHash)
				}
				if blob.HashOf(data) != f.BlobHash {
					return fmt.Errorf("%w: blob %s content does not match its hash", ErrInvalidInput, f.BlobHash)
				}
			}
		}
	}
	memberByUsername := map[string]manifestMember{}
	memberByOrigID := map[string]manifestMember{}
	// Manifest-INTERNAL uniqueness (case-insensitive, matching the repo's EqualFold on
	// username + email). Without this, two would-create members sharing a username or email
	// both reach CreateUser/AddMembership and the second fails mid-write, orphaning a band
	// (all-or-nothing hole — T63). Our own exporter can't emit dups; a crafted zip can.
	seenUsername := map[string]bool{}
	seenEmail := map[string]bool{}
	for _, m := range man.Members {
		if m.Username == "" {
			return fmt.Errorf("%w: a member has no username", ErrInvalidInput)
		}
		if lu := strings.ToLower(m.Username); seenUsername[lu] {
			return fmt.Errorf("%w: duplicate member username %q in the manifest", ErrInvalidInput, m.Username)
		} else {
			seenUsername[lu] = true
		}
		if m.Email != "" {
			if le := strings.ToLower(m.Email); seenEmail[le] {
				return fmt.Errorf("%w: duplicate member email %q in the manifest", ErrInvalidInput, m.Email)
			} else {
				seenEmail[le] = true
			}
		}
		// A would-create member (username absent here) whose email belongs to a
		// DIFFERENT existing account would fail at CreateUser mid-import, after the
		// band + earlier rows are written — breaking all-or-nothing. Pre-validate it
		// against target-server state so nothing is written on conflict.
		if _, err := s.repo.GetUserByUsername(m.Username); err != nil && m.Email != "" {
			if existing, err := s.repo.GetUserByEmail(m.Email); err == nil {
				return fmt.Errorf("%w: member %q would be created, but its email %q already belongs to a different account (%q) on this server", ErrInvalidInput, m.Username, m.Email, existing.Username)
			}
		}
		memberByUsername[m.Username] = m
		memberByOrigID[m.ID] = m
	}
	for _, sl := range man.Setlists {
		for _, it := range sl.Items {
			if !songIDs[it.SongRef] {
				return fmt.Errorf("%w: setlist %q item references unknown song %s", ErrInvalidInput, sl.Name, it.SongRef)
			}
		}
	}
	for _, sel := range man.FileSelections {
		if _, ok := memberByUsername[sel.MemberUsername]; !ok {
			return fmt.Errorf("%w: file selection references unknown member %q", ErrInvalidInput, sel.MemberUsername)
		}
		if !songIDs[sel.SongRef] {
			return fmt.Errorf("%w: file selection references unknown song %s", ErrInvalidInput, sel.SongRef)
		}
		for _, fr := range sel.FileRefs {
			if !fileIDs[fr] {
				return fmt.Errorf("%w: file selection references unknown file %s", ErrInvalidInput, fr)
			}
		}
	}
	for _, c := range man.SongCues {
		if _, ok := memberByUsername[c.MemberUsername]; !ok {
			return fmt.Errorf("%w: song cues reference unknown member %q", ErrInvalidInput, c.MemberUsername)
		}
		if !songIDs[c.SongRef] {
			return fmt.Errorf("%w: song cues reference unknown song %s", ErrInvalidInput, c.SongRef)
		}
	}
	for songRef, ann := range man.Annotations {
		if !songIDs[songRef] {
			return fmt.Errorf("%w: annotations reference unknown song %s", ErrInvalidInput, songRef)
		}
		for _, l := range ann.Layers {
			if l.FileID != "" && !fileIDs[l.FileID] {
				return fmt.Errorf("%w: annotation layer references unknown file %s", ErrInvalidInput, l.FileID)
			}
			if l.OwnerID != "" && l.OwnerID != domain.SharedOwner {
				if _, ok := memberByOrigID[l.OwnerID]; !ok {
					return fmt.Errorf("%w: annotation layer owner %s is not a member in the manifest", ErrInvalidInput, l.OwnerID)
				}
			}
		}
		// Object UUIDs must be present + unique per song. A crafted empty UUID
		// (ErrInvalidMutation) or a duplicate involving a tombstone (ErrDeletedRemotely)
		// would otherwise error mid-`engine.Apply`, after the band/songs/files are written
		// — an all-or-nothing hole (T63). A real HEAD is keyed by UUID, so it can't repeat.
		seenUUID := map[string]bool{}
		for _, o := range ann.Objects {
			if o.UUID == "" {
				return fmt.Errorf("%w: annotation object with no uuid in song %s", ErrInvalidInput, songRef)
			}
			if seenUUID[o.UUID] {
				return fmt.Errorf("%w: duplicate annotation object uuid %s in song %s", ErrInvalidInput, o.UUID, songRef)
			}
			seenUUID[o.UUID] = true
		}
	}
	return nil
}

// PreviewImport parses + fully validates a .tband (T62 rules) and classifies its members
// against the target server WITHOUT writing anything (T63). Any authenticated user may preview.
func (s *Service) PreviewImport(caller User, zipBytes []byte) (ImportPreview, error) {
	man, blobs, err := parseBandZip(zipBytes)
	if err != nil {
		return ImportPreview{}, err
	}
	if err := s.validateImport(man, blobs); err != nil {
		return ImportPreview{}, err
	}
	pv := ImportPreview{
		BandName: strings.TrimSpace(man.Band.Name),
		Members:  s.classifyMembers(caller, man),
		Songs:    len(man.Songs),
		Setlists: len(man.Setlists),
	}
	for _, sg := range man.Songs {
		pv.Files += len(sg.Files)
	}
	return pv, nil
}

// ImportBand creates a NEW band from a .tband zip, owned by (and admin for) the caller.
// All-or-nothing: the whole manifest is validated before anything is written.
//
// SECURITY (T63): membership is CONSENT-REQUIRED. The importer is the band admin; every
// other manifest member is dispositioned — a pre-existing account (a username already on
// this server) may only be invited (default) or skipped, NEVER silently attached (that was
// the T62 account-takeover vector); a new username may be created (default), invited, or
// skipped. invite/skip drop that member's personal content (annotations/cues/selections);
// shared + conductor content is never dropped.
func (s *Service) ImportBand(caller User, eng *engine.Engine, zipBytes []byte, dispositions map[string]ImportDisposition) (ImportReport, error) {
	man, blobs, err := parseBandZip(zipBytes)
	if err != nil {
		return ImportReport{}, err
	}
	if err := s.validateImport(man, blobs); err != nil {
		return ImportReport{}, err
	}

	// Classify every member against the target + caller, then validate the dispositions up
	// front (all-or-nothing). SECURITY (T63): a pre-existing account (other than the importer)
	// is CONSENT-REQUIRED — it may only be invited or skipped, never created or silently
	// attached. A new username may be created (default), invited, or skipped.
	classified := s.classifyMembers(caller, man)
	byUsername := map[string]PreviewMember{}
	for _, pm := range classified {
		byUsername[pm.Username] = pm
	}
	for uname, disp := range dispositions {
		pm, ok := byUsername[uname]
		if !ok || pm.IsCaller {
			return ImportReport{}, fmt.Errorf("%w: disposition names %q, which is not a member you choose for", ErrInvalidInput, uname)
		}
		if !validDisposition(disp) {
			return ImportReport{}, fmt.Errorf("%w: unknown disposition %q for member %q", ErrInvalidInput, disp, uname)
		}
		if pm.Existing && disp == DispositionCreate {
			return ImportReport{}, fmt.Errorf("%w: cannot create an account for %q — it already exists here; invite or skip", ErrInvalidInput, uname)
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
	report := ImportReport{Band: band, Matched: []string{}, Created: []string{}, Invited: []string{}, Skipped: []string{}}

	// Build the orig-id → new-user-id map. A DROPPED member (invited/skipped) has no user
	// to own content in this band, so we track it and skip its personal annotations/cues/
	// selections below.
	memberMap := map[string]string{}       // orig member UUID → new user id (attached members)
	usernameToNewID := map[string]string{} // username → new user id (attached members)
	droppedOrigID := map[string]bool{}     // orig member UUID → its content is dropped
	droppedUsername := map[string]bool{}   // username → its content is dropped

	// invite/skip a member: no membership, drop their personal content. Returns any error.
	inviteOrSkip := func(m manifestMember, disp ImportDisposition) error {
		droppedOrigID[m.ID] = true
		droppedUsername[m.Username] = true
		if disp == DispositionSkip {
			report.Skipped = append(report.Skipped, m.Username)
			return nil
		}
		// Pull-based, email-free invite keyed by username (this app has no SMTP); the invitee
		// sees + accepts it on their next login. Consent lands the membership, not the import.
		// NOTE: invites carry no role, so an accepted invitee joins as a plain member.
		inv := Invite{
			ID: s.newID(), BandID: band.ID, Identifier: m.Username, IdentifierKind: KindUsername,
			InvitedBy: caller.ID, Status: InvitePending, CreatedAt: now,
		}
		if err := s.repo.CreateInvite(inv); err != nil {
			return err
		}
		report.Invited = append(report.Invited, m.Username)
		return nil
	}

	for _, m := range man.Members {
		pm := byUsername[m.Username]
		switch {
		case pm.IsCaller:
			// The importer restoring their own membership — attached as admin (below); their
			// content lands under them. (They consented by importing.)
			memberMap[m.ID] = caller.ID
			usernameToNewID[m.Username] = caller.ID
			report.Matched = append(report.Matched, m.Username)
		case pm.Existing:
			// A DIFFERENT pre-existing account — CONSENT REQUIRED. Never attach/create. The
			// default is invite; the importer may downgrade to skip.
			disp := DispositionInvite
			if d, ok := dispositions[m.Username]; ok {
				disp = d
			}
			if err := inviteOrSkip(m, disp); err != nil {
				return ImportReport{}, err
			}
		default:
			// A NEW username — create (default) | invite | skip.
			disp := DispositionCreate
			if d, ok := dispositions[m.Username]; ok {
				disp = d
			}
			if disp != DispositionCreate {
				if err := inviteOrSkip(m, disp); err != nil {
					return ImportReport{}, err
				}
				continue
			}
			u := User{
				ID: s.newID(), Username: m.Username, DisplayName: m.DisplayName,
				Email: m.Email, AvatarKind: m.AvatarKind, PasswordHash: "", CreatedAt: now,
			}
			if err := s.repo.CreateUser(u); err != nil {
				return ImportReport{}, err
			}
			memberMap[m.ID] = u.ID
			usernameToNewID[m.Username] = u.ID
			report.Created = append(report.Created, m.Username)
			if err := s.repo.AddMembership(Membership{BandID: band.ID, UserID: u.ID, Role: m.Role, CreatedAt: now}); err != nil {
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
			Tempo: sg.Tempo, Meter: sg.Meter, Tags: sg.Tags, Notes: sg.Notes, CreatedAt: now,
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
	// A dropped member (invited/skipped) has no target user to own content, so its layers,
	// objects, cues, and selections are dropped (T63). Shared/conductor layers (SharedOwner)
	// and content owned by matched/created members land as in T62. Objects in a dropped
	// layer go with the layer.
	droppedLayerIDs := map[string]bool{}
	for songRef, ann := range man.Annotations {
		newSongID := songMap[songRef]
		for _, l := range ann.Layers {
			if l.OwnerID != "" && l.OwnerID != domain.SharedOwner && droppedOrigID[l.OwnerID] {
				droppedLayerIDs[l.ID] = true
				report.DroppedLayers++
				continue
			}
			nl := l
			nl.FileID = fileMap[l.FileID]
			nl.OwnerID = remapOwner(l.OwnerID)
			if _, err := eng.Apply(newSongID, domain.Mutation{Kind: domain.KindLayerCreate, Layer: &nl, AuthorID: caller.ID, Summary: "import layer " + nl.ID}); err != nil {
				return ImportReport{}, fmt.Errorf("import annotations: %v", err)
			}
		}
		for _, o := range ann.Objects {
			if droppedLayerIDs[o.LayerID] || (o.OwnerID != "" && o.OwnerID != domain.SharedOwner && droppedOrigID[o.OwnerID]) {
				report.DroppedObjects++
				continue
			}
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

	// Per-member selections + cues (remap song + file refs, keyed by username). A dropped
	// member's selections/cues have no owner on this server, so they're dropped too.
	for _, sel := range man.FileSelections {
		if droppedUsername[sel.MemberUsername] {
			report.DroppedSelections++
			continue
		}
		newFiles := make([]string, 0, len(sel.FileRefs))
		for _, fr := range sel.FileRefs {
			newFiles = append(newFiles, fileMap[fr])
		}
		if err := s.repo.SetFileSelection(FileSelection{UserID: usernameToNewID[sel.MemberUsername], SongID: songMap[sel.SongRef], FileIDs: newFiles}); err != nil {
			return ImportReport{}, err
		}
	}
	for _, c := range man.SongCues {
		if droppedUsername[c.MemberUsername] {
			report.DroppedCues++
			continue
		}
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
	if len(zr.File) > maxImportEntries {
		return bandManifest{}, nil, fmt.Errorf("%w: archive has too many entries (%d > %d)", ErrInvalidInput, len(zr.File), maxImportEntries)
	}

	// Decompress with a running budget so a high-ratio DEFLATE bomb is refused mid-stream
	// (the compressed upload is already capped at MaxImportBytes; this bounds the inflated
	// total). Enforced during the read — a lying header can't get past io.LimitReader.
	remaining := maxDecompressedBytes
	readEntry := func(f *zip.File) ([]byte, error) {
		if f.UncompressedSize64 > uint64(remaining) { // cheap early reject on the honest case
			return nil, fmt.Errorf("%w: decompressed band archive exceeds %d bytes (possible zip bomb)", ErrInvalidInput, maxDecompressedBytes)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		data, err := io.ReadAll(io.LimitReader(rc, remaining+1))
		if err != nil {
			return nil, err
		}
		if int64(len(data)) > remaining {
			return nil, fmt.Errorf("%w: decompressed band archive exceeds %d bytes (possible zip bomb)", ErrInvalidInput, maxDecompressedBytes)
		}
		remaining -= int64(len(data))
		return data, nil
	}

	// Read every entry through the decompression budget, partitioning blobs/ from the JSON
	// files. Reading the JSON entries here (not just band.json) is what lets the v2 reader
	// see repertoire.json / setlists.json / annotations/* — all still under the T63 bound.
	entries := map[string][]byte{}
	blobs := map[string][]byte{}
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "/") {
			continue // directory entry
		}
		data, err := readEntry(f)
		if err != nil {
			return bandManifest{}, nil, err
		}
		if strings.HasPrefix(f.Name, "blobs/") {
			blobs[strings.TrimPrefix(f.Name, "blobs/")] = data
		} else {
			entries[f.Name] = data
		}
	}
	bandJSON, ok := entries["band.json"]
	if !ok {
		return bandManifest{}, nil, fmt.Errorf("%w: archive has no band.json", ErrInvalidInput)
	}

	// The version is the file format's (T134): peek formatVersion, then dispatch. v1 is a
	// single UUID-based manifest; v2 is the folder format spread across sibling files. Any
	// other version is refused with a 400 — no migration code, per VLL's standing ruling.
	var peek struct {
		FormatVersion int `json:"formatVersion"`
	}
	if err := json.Unmarshal(bandJSON, &peek); err != nil {
		return bandManifest{}, nil, fmt.Errorf("%w: band.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	switch peek.FormatVersion {
	case BandExportFormatVersion: // v1
		var man bandManifest
		if err := json.Unmarshal(bandJSON, &man); err != nil {
			return bandManifest{}, nil, fmt.Errorf("%w: band.json is not valid JSON: %v", ErrInvalidInput, err)
		}
		return man, blobs, nil
	case BandExportFormatVersionV2: // v2 folder format
		// v2 has no blobs/ (amendment 4): parseV2 reads the file bytes from `<slug>/<filename>`, verifies
		// each declared blobHash, and returns the bytes keyed by content hash for the shared import core.
		return parseV2(entries)
	default:
		return bandManifest{}, nil, fmt.Errorf("%w: unsupported band export formatVersion %d (this server reads %d and %d)", ErrInvalidInput, peek.FormatVersion, BandExportFormatVersion, BandExportFormatVersionV2)
	}
}

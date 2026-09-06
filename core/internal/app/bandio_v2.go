package app

// Band export/import v2 (T134): the zip carries the FOLDER FORMAT so a band has one
// description outside the software instead of two (the human `cmd/seed` folder and the
// v1 server manifest). See bandio.go for the shared import core; this file is only the
// v2 <-> in-memory-manifest translation.
//
// Layout (amendment 4 — a .tband IS this directory zipped; all JSON is name/slug-based and hand-diffable,
// and the file bytes live under HUMAN names, no blobs/):
//
//	band.json               {formatVersion:2, exportedAt, name, members[]}
//	repertoire.json         {songs[]{slug,title,…,files[]{filename,blobHash,…}}}
//	setlists.json           {setlists[]{name,…,items[]{song:<slug>,…}}}
//	annotations/<slug>.json  {layers[],objects[]}   per song; ids kept VERBATIM
//	cues.json (optional)     {selections[],cues[]}  per-member, by username+slug
//	<slug>/<filename>        the file bytes, under human names; blobHash is an INTEGRITY field
//
// The version is the FILE FORMAT's: v1 -> v2 is a genuine bump (the files are rearranged),
// so a v1 reader would not find what it expects. We WRITE v2 always and READ v1 and v2.
//
// Identity: the human layer is names (username) and slugs; the ANNOTATION payload keeps
// its ids — layer.id, object.uuid and object.layerId survive verbatim so a round-trip
// preserves annotation identity, not merely counts. Only file refs (by filename) and
// owner refs (by username) are name-based; the reader resolves them back to the ids the
// shared import core (bandio.go) then re-mints along with every other relational id.
//
// Folder → v2 collisions (named, from migrating the two real band libraries — T134 hold
// review). These are where a hand-written seed folder and v2 disagree; the migration tool
// (phase 2) resolves them, and the format declares rather than derives:
//
//  1. `role` is a permission enum in v2 (admin/conductor/member), but FREE TEXT describing
//     what a person plays in the folder convention. Feeding prose to the enum lands silently
//     as `member` (enumwire: unknown string → zero). Migration moves the prose to `plays`
//     (an unknown key the reader ignores) and sets `role` to the enum.
//  2. The folder keeps `admin` BESIDE `members`; v2 puts everyone in `members[]` with a role.
//     A folder's `admin` must be folded into members[] on migration, never dropped (in one
//     library it is the only privileged account).
//  3. A `personal` layer needs an owner; the folder format never recorded one. There is no
//     correct in-file answer — a real EXPORT always has the owner, so this only bites
//     hand-authored/seeded folders, where the migration tool must supply it explicitly.
//  4. The directory never contains a hash: file bytes live under human `<slug>/<filename>` names, both
//     in the directory AND in the zip (amendment 4 removed blobs/). `blobHash` stays in repertoire.json
//     as an integrity field the reader verifies after reading the bytes.
//  5. The repertoire is the index: a `<slug>/` directory is a song only if its slug is
//     DECLARED in repertoire.json (so a stray dir like __pycache__ is not imported as a
//     song). The migration tool derives the file list; the format declares it via files[].

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"troubastack/core/internal/app/blob"
	"troubastack/core/internal/chartpdf"
	"troubastack/core/internal/domain"
)

// BandExportFormatVersionV2 is the version an export now writes. The reader accepts this
// and BandExportFormatVersion (v1); anything else is a 400.
const BandExportFormatVersionV2 = 2

// --- v2 wire shapes ------------------------------------------------------------------

type v2Band struct {
	FormatVersion int    `json:"formatVersion"`
	ExportedAt    string `json:"exportedAt,omitempty"`
	// T150: the band's durable declared id (see manifestBand.ID). omitempty — a pre-T150 folder has none.
	ID      string     `json:"id,omitempty"`
	Name    string     `json:"name"`
	Members []v2Member `json:"members"`
	// Shortname/Kind/Notes are the band's author-declared identity. T152: the app now STORES them (app.Band)
	// and both import and export carry them, so `make band=<shortname>` keeps working across a round-trip
	// (previously an export dropped them). Still omitempty: a pre-T152 export / an in-app band omits them.
	Shortname string `json:"shortname,omitempty"`
	Kind      string `json:"kind,omitempty"`
	Notes     string `json:"notes,omitempty"`
}

type v2Member struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarKind  string `json:"avatarKind,omitempty"`
	Role        string `json:"role"`
	// Plays is the folder's instrument prose (⟨P1⟩): the legacy folder's `role` was free text naming
	// what a person plays. It is documentation the reader ignores — `role` above is the permission enum.
	Plays string `json:"plays,omitempty"`
}

type v2Repertoire struct {
	Songs []v2Song `json:"songs"`
}

type v2Song struct {
	Slug   string   `json:"slug"`
	Title  string   `json:"title"`
	Artist string   `json:"artist,omitempty"`
	Key    string   `json:"key,omitempty"`
	Tempo  int      `json:"tempo,omitempty"`
	Meter  string   `json:"meter,omitempty"`
	Notes  string   `json:"notes,omitempty"`
	Tags   []string `json:"tags,omitempty"`
	Files  []v2File `json:"files,omitempty"`
}

type v2File struct {
	Filename     string `json:"filename"`
	ContentType  string `json:"contentType,omitempty"`
	Size         int64  `json:"size,omitempty"`
	BlobHash     string `json:"blobHash,omitempty"`
	DisplayOrder int    `json:"displayOrder,omitempty"`
	Generated    bool   `json:"generated,omitempty"`
	Revision     int    `json:"revision,omitempty"`
	// ⟨F1⟩: no chartSource field — for a generated chart the bytes under <slug>/<filename> ARE the
	// source (rendered on import), so a second copy would drift silently past the blobHash check.
}

type v2SetlistsFile struct {
	Setlists []v2Setlist `json:"setlists"`
}

type v2Setlist struct {
	// T150: the setlist's durable declared id. omitempty — a pre-T150 folder has none.
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name"`
	EventDate string          `json:"eventDate,omitempty"`
	Venue     string          `json:"venue,omitempty"`
	Notes     string          `json:"notes,omitempty"`
	Items     []v2SetlistItem `json:"items"`
}

type v2SetlistItem struct {
	Song            string `json:"song"` // repertoire slug; EMPTY for an intermission (T153)
	KeyOverride     string `json:"keyOverride,omitempty"`
	TempoOverride   int    `json:"tempoOverride,omitempty"`
	Notes           string `json:"notes,omitempty"`
	OnCall          bool   `json:"onCall,omitempty"`
	TransposeChords bool   `json:"transposeChords,omitempty"`
	// T153: absent ⇒ "song", so every folder written before T153 keeps its meaning. An intermission
	// declares kind:"intermission" and carries no `song`; its Label is the band's own words.
	Kind  string `json:"kind,omitempty"`
	Label string `json:"label,omitempty"`
}

type v2Annotations struct {
	Layers  []v2Layer  `json:"layers"`
	Objects []v2Object `json:"objects"`
}

type v2Layer struct {
	ID        string `json:"id"`             // KEPT verbatim
	File      string `json:"file,omitempty"` // filename within the song; "" = song-level
	Name      string `json:"name,omitempty"`
	Owner     string `json:"owner,omitempty"` // username, or the _shared_ sentinel
	Zone      string `json:"zone,omitempty"`
	Order     int    `json:"order,omitempty"`
	Access    string `json:"access,omitempty"`
	Mandatory bool   `json:"mandatory,omitempty"`
	RoleTag   string `json:"roleTag,omitempty"`
}

type v2Object struct {
	UUID      string    `json:"uuid"`  // KEPT verbatim
	Layer     string    `json:"layer"` // layer id, KEPT verbatim
	Type      string    `json:"type"`
	Points    []v2Point `json:"points,omitempty"`
	Page      int       `json:"page,omitempty"`
	Text      string    `json:"text,omitempty"`
	Style     v2Style   `json:"style"`
	Owner     string    `json:"owner,omitempty"` // username, or the _shared_ sentinel
	Scope     string    `json:"scope,omitempty"`
	Version   uint64    `json:"version,omitempty"`
	CreatedAt int64     `json:"createdAt,omitempty"`
	Order     int       `json:"order,omitempty"`
}

type v2Point struct {
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
	Pressure float64 `json:"pressure,omitempty"`
}

type v2Style struct {
	Color    string  `json:"color,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
	Width    float64 `json:"width,omitempty"`
	FontSize float64 `json:"fontSize,omitempty"`
	Fill     *bool   `json:"fill,omitempty"`
	Stroke   *bool   `json:"stroke,omitempty"`
	Blend    string  `json:"blend,omitempty"`
}

type v2CuesFile struct {
	Selections []v2Selection `json:"selections,omitempty"`
	Cues       []v2Cue       `json:"cues,omitempty"`
}

type v2Selection struct {
	Member string   `json:"member"` // username
	Song   string   `json:"song"`   // slug
	Files  []string `json:"files"`  // filenames
}

type v2Cue struct {
	Member string    `json:"member"` // username
	Song   string    `json:"song"`   // slug
	Cues   []SongCue `json:"cues"`
}

// --- writer: in-memory manifest -> v2 files ------------------------------------------

// marshalV2 translates the (UUID-based) in-memory manifest into the v2 zip entries (name + slug based):
// entryName -> bytes for every JSON file AND every file's bytes under `<slug>/<filename>` (amendment 4 —
// no `blobs/`, the archive IS the folder). getBlob fetches a file's bytes by its content hash; blobHash
// stays in repertoire.json as an integrity field. A song's slug is the one STORED on it (T139); it is
// emitted verbatim so a round-trip preserves an author's chosen identifier — only a song with no stored
// slug (predating the field) falls back to deriving one from the title. Slugs are kept unique within the
// band and filenames are made path-safe + unique within a song, so a `<slug>/<filename>` entry is always
// one safe two-segment path and annotations can reference a file by filename.
func marshalV2(man bandManifest, getBlob func(string) ([]byte, error)) (map[string][]byte, error) {
	files := map[string][]byte{}

	// Member id -> username (for owner refs). Also emit band.json members.
	userByID := map[string]string{}
	band := v2Band{
		FormatVersion: BandExportFormatVersionV2, ExportedAt: man.ExportedAt, ID: man.Band.ID, Name: man.Band.Name,
		// T150/T152: emit the declared id + author-declared identity so a folder round-trip preserves them.
		Shortname: man.Band.Shortname, Kind: man.Band.Kind, Notes: man.Band.Notes,
	}
	for _, m := range man.Members {
		userByID[m.ID] = m.Username
		band.Members = append(band.Members, v2Member{
			Username: m.Username, DisplayName: m.DisplayName, Email: m.Email,
			AvatarKind: string(m.AvatarKind), Role: string(m.Role),
		})
	}

	// Song id -> slug (unique within band); file id -> filename (unique within song) + slug.
	slugBySong := map[string]string{}
	fileNameByID := map[string]string{} // fileID -> filename as emitted
	usedSlugs := map[string]bool{}
	rep := v2Repertoire{}
	for _, ms := range man.Songs {
		// Emit the STORED slug; derive from the title only for a song that has none (predates T139).
		// uniqueKey still guards against a collision after the fallback.
		base := ms.Slug
		if base == "" {
			base = Slugify(ms.Title)
		}
		slug := uniqueKey(base, usedSlugs)
		slugBySong[ms.ID] = slug
		vs := v2Song{
			Slug: slug, Title: ms.Title, Artist: ms.Artist, Key: ms.Key,
			Tempo: ms.Tempo, Meter: ms.Meter, Notes: ms.Notes, Tags: ms.Tags,
		}
		usedNames := map[string]bool{}
		for _, mf := range ms.Files {
			name := uniqueName(safeFilename(mf.Filename), usedNames)
			fileNameByID[mf.ID] = name
			// The bytes live under the human name; no blobs/ (amendment 4). ⟨F1⟩: a GENERATED chart's
			// bytes ARE its source (rendered on import) — a rendered PDF is a derived artefact and does
			// not belong in a diffable folder. A normal file writes its blob bytes.
			var data []byte
			hash := mf.BlobHash
			if mf.Generated {
				data = []byte(mf.ChartSource)
				hash = blob.HashOf(data)
			} else if mf.BlobHash != "" && getBlob != nil {
				d, gerr := getBlob(mf.BlobHash)
				if gerr != nil {
					return nil, fmt.Errorf("export v2: file %q blob %s: %w", name, mf.BlobHash, gerr)
				}
				data = d
			}
			vs.Files = append(vs.Files, v2File{
				Filename: name, ContentType: mf.ContentType, Size: mf.Size, BlobHash: hash,
				DisplayOrder: mf.DisplayOrder, Generated: mf.Generated, Revision: mf.Revision,
			})
			if data != nil {
				files[slug+"/"+name] = data
			}
		}
		rep.Songs = append(rep.Songs, vs)
	}

	// Annotations, one file per song (only for songs that have any).
	ownerToName := func(ownerID string) string {
		if ownerID == "" || ownerID == domain.SharedOwner {
			return ownerID
		}
		return userByID[ownerID]
	}
	for songID, ann := range man.Annotations {
		slug := slugBySong[songID]
		if slug == "" {
			return nil, fmt.Errorf("export v2: annotations for unknown song %q", songID)
		}
		va := v2Annotations{}
		for _, l := range ann.Layers {
			va.Layers = append(va.Layers, v2Layer{
				ID: l.ID, File: fileNameByID[l.FileID], Name: l.Name, Owner: ownerToName(l.OwnerID),
				Zone: domain.ZoneToString(l.Zone), Order: l.Order, Access: domain.AccessToString(l.Access),
				Mandatory: l.Mandatory, RoleTag: l.RoleTag,
			})
		}
		for _, o := range ann.Objects {
			vo := v2Object{
				UUID: o.UUID, Layer: o.LayerID, Type: domain.ObjectTypeToString(o.Type), Page: o.Page,
				Text: o.Text, Owner: ownerToName(o.OwnerID), Scope: domain.ScopeToString(o.Scope),
				Version: o.Version, CreatedAt: o.CreatedAt, Order: o.Order,
				Style: v2Style{
					Color: o.Style.Color, Opacity: o.Style.Opacity, Width: o.Style.Width,
					FontSize: o.Style.FontSize, Fill: o.Style.Fill, Stroke: o.Style.Stroke, Blend: o.Style.Blend,
				},
			}
			for _, p := range o.Points {
				vo.Points = append(vo.Points, v2Point{X: p.X, Y: p.Y, Pressure: p.Pressure})
			}
			va.Objects = append(va.Objects, vo)
		}
		b, err := marshalIndent(va)
		if err != nil {
			return nil, err
		}
		files["annotations/"+slug+".json"] = b
	}

	// Setlists.
	sls := v2SetlistsFile{}
	for _, msl := range man.Setlists {
		vsl := v2Setlist{ID: msl.ID, Name: msl.Name, EventDate: msl.EventDate, Venue: msl.Venue, Notes: msl.Notes}
		for _, it := range msl.Items {
			// T153: an intermission has no song, so it has no slug to resolve. Export it by kind+label;
			// the unknown-song guard below still protects every entry that CLAIMS to be a song.
			if it.Kind == SetlistKindIntermission {
				vsl.Items = append(vsl.Items, v2SetlistItem{Kind: it.Kind, Label: it.Label, Notes: it.Notes})
				continue
			}
			slug := slugBySong[it.SongRef]
			if slug == "" {
				return nil, fmt.Errorf("export v2: setlist %q references unknown song %q", msl.Name, it.SongRef)
			}
			vsl.Items = append(vsl.Items, v2SetlistItem{
				Song: slug, KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride,
				Notes: it.Notes, OnCall: it.OnCall, TransposeChords: it.TransposeChords,
			})
		}
		sls.Setlists = append(sls.Setlists, vsl)
	}

	// Cues (file selections + song cues), per member, keyed by username + slug + filenames.
	cues := v2CuesFile{}
	for _, sel := range man.FileSelections {
		slug := slugBySong[sel.SongRef]
		if slug == "" {
			continue
		}
		names := make([]string, 0, len(sel.FileRefs))
		for _, fid := range sel.FileRefs {
			if n := fileNameByID[fid]; n != "" {
				names = append(names, n)
			}
		}
		cues.Selections = append(cues.Selections, v2Selection{Member: sel.MemberUsername, Song: slug, Files: names})
	}
	for _, sc := range man.SongCues {
		slug := slugBySong[sc.SongRef]
		if slug == "" {
			continue
		}
		cues.Cues = append(cues.Cues, v2Cue{Member: sc.MemberUsername, Song: slug, Cues: sc.Cues})
	}

	// Serialize the always-present files.
	var err error
	if files["band.json"], err = marshalIndent(band); err != nil {
		return nil, err
	}
	if files["repertoire.json"], err = marshalIndent(rep); err != nil {
		return nil, err
	}
	if files["setlists.json"], err = marshalIndent(sls); err != nil {
		return nil, err
	}
	if len(cues.Selections) > 0 || len(cues.Cues) > 0 {
		if files["cues.json"], err = marshalIndent(cues); err != nil {
			return nil, err
		}
	}
	return files, nil
}

func marshalIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// writeV2Zip writes the v2 entry set (JSON files + `<slug>/<filename>` bytes) into a deterministic zip
// — sorted entry names, so the same band exports byte-identically.
func writeV2Zip(files map[string][]byte) ([]byte, error) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(files))
	for n := range files {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(files[n]); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// --- reader: v2 files -> in-memory manifest ------------------------------------------

// parseV2 turns the v2 file set into the same bandManifest the v1 path produces, so the
// shared validateImport + ImportBand run unchanged. It mints synthetic, internally
// consistent ids (they are all re-minted on import anyway); the ids that MUST survive —
// layer.id, object.uuid, object.layerId — are copied verbatim. Name/slug/filename refs
// that do not resolve are a hard error (all-or-nothing).
func parseV2(entries map[string][]byte) (bandManifest, map[string][]byte, error) {
	var band v2Band
	if err := json.Unmarshal(entries["band.json"], &band); err != nil {
		return bandManifest{}, nil, fmt.Errorf("%w: band.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	man := bandManifest{
		FormatVersion: BandExportFormatVersionV2,
		ExportedAt:    band.ExportedAt,
		// T150/T152: carry the folder's declared identity (id + shortname/kind/notes) into the manifest so
		// import resolves + stores it.
		Band:        manifestBand{ID: band.ID, Name: band.Name, Shortname: band.Shortname, Kind: band.Kind, Notes: band.Notes},
		Annotations: map[string]manifestAnnots{},
	}

	// Members: synthetic id m-<username>; the id is only a manifest-internal join key.
	memberID := func(username string) string { return "m-" + username }
	for _, m := range band.Members {
		man.Members = append(man.Members, manifestMember{
			ID: memberID(m.Username), Username: m.Username, DisplayName: m.DisplayName,
			Email: m.Email, AvatarKind: AvatarKind(m.AvatarKind), Role: Role(m.Role),
		})
	}

	// Repertoire: synthetic song id s-<slug>, file id f-<slug>-<i>. Build lookup maps.
	var rep v2Repertoire
	if raw, ok := entries["repertoire.json"]; ok {
		if err := json.Unmarshal(raw, &rep); err != nil {
			return bandManifest{}, nil, fmt.Errorf("%w: repertoire.json is not valid JSON: %v", ErrInvalidInput, err)
		}
	}
	songIDBySlug := map[string]string{}
	fileIDBySlugName := map[string]string{} // "slug\x00filename" -> synthetic file id
	for _, vs := range rep.Songs {
		if vs.Slug == "" {
			return bandManifest{}, nil, fmt.Errorf("%w: repertoire song %q has an empty slug", ErrInvalidInput, vs.Title)
		}
		if _, dup := songIDBySlug[vs.Slug]; dup {
			return bandManifest{}, nil, fmt.Errorf("%w: duplicate song slug %q", ErrInvalidInput, vs.Slug)
		}
		// ⟨P7⟩: slug/filename become the `<slug>/<filename>` entry name (attacker-controllable under
		// amendment 4). Refuse anything that isn't a single safe path segment, so the constructed entry
		// name cannot traverse — matched against the manifest, never filepath.Clean'd.
		if pathUnsafe(vs.Slug) {
			return bandManifest{}, nil, fmt.Errorf("%w: unsafe song slug %q", ErrInvalidInput, vs.Slug)
		}
		sid := "s-" + vs.Slug
		songIDBySlug[vs.Slug] = sid
		ms := manifestSong{
			// T139: carry the DECLARED slug verbatim so the import writer stores it — the author's
			// identifier survives the round-trip, and a later title edit will not rename it.
			ID: sid, Slug: vs.Slug, Title: vs.Title, Artist: vs.Artist, Key: vs.Key,
			Tempo: vs.Tempo, Meter: vs.Meter, Tags: vs.Tags, Notes: vs.Notes,
		}
		for i, vf := range vs.Files {
			if pathUnsafe(vf.Filename) {
				return bandManifest{}, nil, fmt.Errorf("%w: unsafe filename %q in song %q", ErrInvalidInput, vf.Filename, vs.Slug)
			}
			fid := fmt.Sprintf("f-%s-%d", vs.Slug, i)
			fileIDBySlugName[vs.Slug+"\x00"+vf.Filename] = fid
			ms.Files = append(ms.Files, manifestFile{
				ID: fid, Filename: vf.Filename, ContentType: vf.ContentType, Size: vf.Size,
				BlobHash: vf.BlobHash, DisplayOrder: vf.DisplayOrder, Generated: vf.Generated,
				Revision: vf.Revision, // ⟨F1⟩: ChartSource is filled from the file bytes in the read loop below.
			})
		}
		man.Songs = append(man.Songs, ms)
	}

	ownerFromName := func(name string) (string, error) {
		if name == "" || name == domain.SharedOwner {
			return name, nil
		}
		// must be a known member username
		for _, m := range band.Members {
			if m.Username == name {
				return memberID(name), nil
			}
		}
		return "", fmt.Errorf("%w: annotation owner %q is not a band member", ErrInvalidInput, name)
	}

	// Annotations: annotations/<slug>.json. Keep ids verbatim; resolve file+owner refs.
	for name, raw := range entries {
		if !strings.HasPrefix(name, "annotations/") || !strings.HasSuffix(name, ".json") {
			continue
		}
		slug := strings.TrimSuffix(strings.TrimPrefix(name, "annotations/"), ".json")
		songID, ok := songIDBySlug[slug]
		if !ok {
			return bandManifest{}, nil, fmt.Errorf("%w: annotations for unknown song slug %q", ErrInvalidInput, slug)
		}
		var va v2Annotations
		if err := json.Unmarshal(raw, &va); err != nil {
			return bandManifest{}, nil, fmt.Errorf("%w: %s is not valid JSON: %v", ErrInvalidInput, name, err)
		}
		ann := manifestAnnots{}
		for _, vl := range va.Layers {
			fileID := ""
			if vl.File != "" {
				fileID, ok = fileIDBySlugName[slug+"\x00"+vl.File]
				if !ok {
					return bandManifest{}, nil, fmt.Errorf("%w: layer %q references unknown file %q in song %q", ErrInvalidInput, vl.ID, vl.File, slug)
				}
			}
			owner, err := ownerFromName(vl.Owner)
			if err != nil {
				return bandManifest{}, nil, err
			}
			ann.Layers = append(ann.Layers, domain.Layer{
				ID: vl.ID, FileID: fileID, Name: vl.Name, OwnerID: owner,
				Zone: domain.ZoneFromString(vl.Zone), Order: vl.Order,
				Access: domain.AccessFromString(vl.Access), Mandatory: vl.Mandatory, RoleTag: vl.RoleTag,
			})
		}
		for _, vo := range va.Objects {
			owner, err := ownerFromName(vo.Owner)
			if err != nil {
				return bandManifest{}, nil, err
			}
			o := domain.Object{
				UUID: vo.UUID, Type: domain.ObjectTypeFromString(vo.Type), Page: vo.Page, Text: vo.Text,
				OwnerID: owner, Scope: domain.ScopeFromString(vo.Scope), LayerID: vo.Layer,
				Version: vo.Version, CreatedAt: vo.CreatedAt, Order: vo.Order,
				Style: domain.Style{
					Color: vo.Style.Color, Opacity: vo.Style.Opacity, Width: vo.Style.Width,
					FontSize: vo.Style.FontSize, Fill: vo.Style.Fill, Stroke: vo.Style.Stroke, Blend: vo.Style.Blend,
				},
			}
			for _, p := range vo.Points {
				o.Points = append(o.Points, domain.Point{X: p.X, Y: p.Y, Pressure: p.Pressure})
			}
			ann.Objects = append(ann.Objects, o)
		}
		man.Annotations[songID] = ann
	}

	// Setlists.
	var sls v2SetlistsFile
	if raw, ok := entries["setlists.json"]; ok {
		if err := json.Unmarshal(raw, &sls); err != nil {
			return bandManifest{}, nil, fmt.Errorf("%w: setlists.json is not valid JSON: %v", ErrInvalidInput, err)
		}
	}
	for _, vsl := range sls.Setlists {
		msl := manifestSetlist{ID: vsl.ID, Name: vsl.Name, EventDate: vsl.EventDate, Venue: vsl.Venue, Notes: vsl.Notes}
		for idx, it := range vsl.Items {
			// T153: an intermission occupies a position like any entry but references no song. Everything
			// else in the loop (Position from array order, T140) applies to it unchanged.
			if it.Kind == SetlistKindIntermission {
				msl.Items = append(msl.Items, manifestItem{
					Position: idx, Notes: it.Notes, Kind: it.Kind, Label: it.Label,
				})
				continue
			}
			songID, ok := songIDBySlug[it.Song]
			if !ok {
				return bandManifest{}, nil, fmt.Errorf("%w: setlist %q references unknown song slug %q", ErrInvalidInput, vsl.Name, it.Song)
			}
			// T140: the v2 folder expresses running order as ARRAY ORDER (v2SetlistItem has no position);
			// materialise it as Position here. Without this every imported item is Position 0, so retrieval
			// (SortSetlistItems) falls back to UUID order and the concert plays scrambled — hit in a real
			// rehearsal.
			msl.Items = append(msl.Items, manifestItem{
				SongRef: songID, Position: idx, KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride,
				Notes: it.Notes, OnCall: it.OnCall, TransposeChords: it.TransposeChords,
			})
		}
		man.Setlists = append(man.Setlists, msl)
	}

	// Cues (optional).
	if raw, ok := entries["cues.json"]; ok {
		var cf v2CuesFile
		if err := json.Unmarshal(raw, &cf); err != nil {
			return bandManifest{}, nil, fmt.Errorf("%w: cues.json is not valid JSON: %v", ErrInvalidInput, err)
		}
		for _, s := range cf.Selections {
			songID, ok := songIDBySlug[s.Song]
			if !ok {
				return bandManifest{}, nil, fmt.Errorf("%w: file selection references unknown song slug %q", ErrInvalidInput, s.Song)
			}
			refs := make([]string, 0, len(s.Files))
			for _, fn := range s.Files {
				fid, ok := fileIDBySlugName[s.Song+"\x00"+fn]
				if !ok {
					return bandManifest{}, nil, fmt.Errorf("%w: file selection references unknown file %q in song %q", ErrInvalidInput, fn, s.Song)
				}
				refs = append(refs, fid)
			}
			man.FileSelections = append(man.FileSelections, manifestSelection{MemberUsername: s.Member, SongRef: songID, FileRefs: refs})
		}
		for _, c := range cf.Cues {
			songID, ok := songIDBySlug[c.Song]
			if !ok {
				return bandManifest{}, nil, fmt.Errorf("%w: song cues reference unknown song slug %q", ErrInvalidInput, c.Song)
			}
			man.SongCues = append(man.SongCues, manifestCue{MemberUsername: c.Member, SongRef: songID, Cues: c.Cues})
		}
	}

	// Read file bytes from `<slug>/<filename>` (amendment 4 — no blobs/). Verify the declared blobHash as
	// an INTEGRITY check (a stale hash that packed quietly would import the wrong bytes under a right
	// name), then key the bytes by content hash so the shared ImportBand core runs unchanged.
	blobs := map[string][]byte{}
	allowed := map[string]bool{"band.json": true, "repertoire.json": true, "setlists.json": true, "cues.json": true}
	for i := range man.Songs {
		slug := strings.TrimPrefix(man.Songs[i].ID, "s-")
		allowed["annotations/"+slug+".json"] = true
		for j := range man.Songs[i].Files {
			f := &man.Songs[i].Files[j]
			entry := slug + "/" + f.Filename
			data, ok := entries[entry]
			if !ok {
				return bandManifest{}, nil, fmt.Errorf("%w: declared file %q has no bytes in the archive", ErrInvalidInput, entry)
			}
			// ⟨P2⟩ integrity is against the archive bytes as stored (the SOURCE for a generated chart).
			if f.BlobHash != "" && f.BlobHash != blob.HashOf(data) {
				return bandManifest{}, nil, fmt.Errorf("%w: file %q content does not match its declared blobHash", ErrInvalidInput, entry)
			}
			allowed[entry] = true
			if f.Generated {
				// ⟨F1⟩: the bytes ARE the chart source; render to the PDF that gets stored, and keep the
				// source so a round-trip re-exports the same text (import re-renders, like CreateTextChart).
				pdf, rerr := chartpdf.Render(string(data))
				if rerr != nil {
					return bandManifest{}, nil, fmt.Errorf("%w: generated chart %q does not render: %v", ErrInvalidInput, entry, rerr)
				}
				f.ChartSource = string(data)
				f.BlobHash = blob.HashOf(pdf)
				blobs[f.BlobHash] = pdf
			} else {
				h := blob.HashOf(data)
				f.BlobHash = h
				blobs[h] = data
			}
		}
	}
	// ⟨P7⟩: every entry must be a known JSON name or a DECLARED `<slug>/<filename>`. A zip entry that is
	// neither is REFUSED, not ignored — under amendment 4 entry names are attacker-controllable user
	// data, and this is matched against the manifest rather than cleaned, so a traversal / absolute /
	// Unicode-normalised name simply is not in `allowed`.
	for name := range entries {
		if !allowed[name] {
			return bandManifest{}, nil, fmt.Errorf("%w: unexpected archive entry %q (not a declared file or a known manifest file)", ErrInvalidInput, name)
		}
	}

	return man, blobs, nil
}

// --- helpers -------------------------------------------------------------------------

// pathUnsafe reports whether a slug or filename could escape its `<slug>/<filename>` entry — a path
// separator, `.`/`..`, a NUL, or a Windows drive letter. ⟨P7⟩: the manifest refuses these on READ, so a
// constructed entry name is always a single safe two-segment path (no filepath.Clean, no resolution).
func pathUnsafe(s string) bool {
	return s == "" || s == "." || s == ".." || strings.ContainsAny(s, "/\\\x00") || (len(s) >= 2 && s[1] == ':')
}

// safeFilename defangs a stored filename into a single safe path segment for the `<slug>/<filename>`
// entry on EXPORT (a slug is already safe via slugify). Path separators and NUL become '-'; a leading
// drive letter is defanged; empty/dot names fall back. So an export can never produce an archive its own
// ⟨P7⟩ reader would refuse.
func safeFilename(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	if len(name) >= 2 && name[1] == ':' {
		name = "-" + name[1:]
	}
	return name
}

// Slugify lowercases, keeps [a-z0-9], and collapses every other run to a single '-'. This is the ONE
// slug rule for the whole module (T139): it governs newly-created songs only — an existing song's slug is
// stored and never re-derived — and cmd/seed calls this same function so the two cannot drift apart.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	dash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
		} else if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "song"
	}
	return out
}

// uniqueKey returns key, or key-2, key-3, … so it is unique within used; marks it used.
func uniqueKey(key string, used map[string]bool) string {
	cand := key
	for i := 2; used[cand]; i++ {
		cand = fmt.Sprintf("%s-%d", key, i)
	}
	used[cand] = true
	return cand
}

// uniqueName makes a filename unique within a song by suffixing before the extension.
func uniqueName(name string, used map[string]bool) string {
	if name == "" {
		name = "file"
	}
	if !used[name] {
		used[name] = true
		return name
	}
	base, ext := name, ""
	if i := strings.LastIndexByte(name, '.'); i > 0 {
		base, ext = name[:i], name[i:]
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("%s-%d%s", base, i, ext)
		if !used[cand] {
			used[cand] = true
			return cand
		}
	}
}

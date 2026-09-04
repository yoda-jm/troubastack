package app

// Band export/import v2 (T134): the zip carries the FOLDER FORMAT so a band has one
// description outside the software instead of two (the human `cmd/seed` folder and the
// v1 server manifest). See bandio.go for the shared import core; this file is only the
// v2 <-> in-memory-manifest translation.
//
// Layout (all JSON is name/slug-based and hand-diffable; blobs are unchanged from v1):
//
//	band.json               {formatVersion:2, exportedAt, name, members[]}
//	repertoire.json         {songs[]{slug,title,…,files[]}}
//	setlists.json           {setlists[]{name,…,items[]{song:<slug>,…}}}
//	annotations/<slug>.json  {layers[],objects[]}   per song; ids kept VERBATIM
//	cues.json (optional)     {selections[],cues[]}  per-member, by username+slug
//	blobs/<sha256>           file bytes, content-addressed
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
//  4. The directory must never contain a hash: file bytes live under human `<slug>/<filename>`
//     names in the canonical directory, and the PACKER (phase 2) content-addresses them to
//     `blobs/<sha256>` at zip time, filling `blobHash` in repertoire.json. The reader here
//     resolves bytes by `blobHash`, so a packed zip imports; the directory stays diffable.
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

	"troubastack/core/internal/domain"
)

// BandExportFormatVersionV2 is the version an export now writes. The reader accepts this
// and BandExportFormatVersion (v1); anything else is a 400.
const BandExportFormatVersionV2 = 2

// --- v2 wire shapes ------------------------------------------------------------------

type v2Band struct {
	FormatVersion int        `json:"formatVersion"`
	ExportedAt    string     `json:"exportedAt,omitempty"`
	Name          string     `json:"name"`
	Members       []v2Member `json:"members"`
}

type v2Member struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Email       string `json:"email,omitempty"`
	AvatarKind  string `json:"avatarKind,omitempty"`
	Role        string `json:"role"`
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
	ChartSource  string `json:"chartSource,omitempty"`
}

type v2SetlistsFile struct {
	Setlists []v2Setlist `json:"setlists"`
}

type v2Setlist struct {
	Name      string          `json:"name"`
	EventDate string          `json:"eventDate,omitempty"`
	Venue     string          `json:"venue,omitempty"`
	Notes     string          `json:"notes,omitempty"`
	Items     []v2SetlistItem `json:"items"`
}

type v2SetlistItem struct {
	Song            string `json:"song"` // repertoire slug
	KeyOverride     string `json:"keyOverride,omitempty"`
	TempoOverride   int    `json:"tempoOverride,omitempty"`
	Notes           string `json:"notes,omitempty"`
	OnCall          bool   `json:"onCall,omitempty"`
	TransposeChords bool   `json:"transposeChords,omitempty"`
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

// marshalV2 translates the (UUID-based) in-memory manifest into the v2 file set (name +
// slug based). Returns entryName -> JSON bytes for every non-blob entry; the caller adds
// blobs/. Slugs are derived from titles (unique within the band) and filenames are made
// unique within a song, so annotations/<slug>.json can reference a file by filename.
func marshalV2(man bandManifest) (map[string][]byte, error) {
	files := map[string][]byte{}

	// Member id -> username (for owner refs). Also emit band.json members.
	userByID := map[string]string{}
	band := v2Band{FormatVersion: BandExportFormatVersionV2, ExportedAt: man.ExportedAt, Name: man.Band.Name}
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
		slug := uniqueKey(slugify(ms.Title), usedSlugs)
		slugBySong[ms.ID] = slug
		vs := v2Song{
			Slug: slug, Title: ms.Title, Artist: ms.Artist, Key: ms.Key,
			Tempo: ms.Tempo, Meter: ms.Meter, Notes: ms.Notes, Tags: ms.Tags,
		}
		usedNames := map[string]bool{}
		for _, mf := range ms.Files {
			name := uniqueName(mf.Filename, usedNames)
			fileNameByID[mf.ID] = name
			vs.Files = append(vs.Files, v2File{
				Filename: name, ContentType: mf.ContentType, Size: mf.Size, BlobHash: mf.BlobHash,
				DisplayOrder: mf.DisplayOrder, Generated: mf.Generated, Revision: mf.Revision,
				ChartSource: mf.ChartSource,
			})
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
		vsl := v2Setlist{Name: msl.Name, EventDate: msl.EventDate, Venue: msl.Venue, Notes: msl.Notes}
		for _, it := range msl.Items {
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

// writeV2Zip writes the v2 file set + blobs/ into a deterministic zip.
func writeV2Zip(files map[string][]byte, blobs map[string][]byte) ([]byte, error) {
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
	hashes := make([]string, 0, len(blobs))
	for h := range blobs {
		hashes = append(hashes, h)
	}
	sort.Strings(hashes)
	for _, h := range hashes {
		w, err := zw.Create("blobs/" + h)
		if err != nil {
			return nil, err
		}
		if _, err := w.Write(blobs[h]); err != nil {
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
func parseV2(entries map[string][]byte) (bandManifest, error) {
	var band v2Band
	if err := json.Unmarshal(entries["band.json"], &band); err != nil {
		return bandManifest{}, fmt.Errorf("%w: band.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	man := bandManifest{
		FormatVersion: BandExportFormatVersionV2,
		ExportedAt:    band.ExportedAt,
		Band:          manifestBand{Name: band.Name},
		Annotations:   map[string]manifestAnnots{},
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
			return bandManifest{}, fmt.Errorf("%w: repertoire.json is not valid JSON: %v", ErrInvalidInput, err)
		}
	}
	songIDBySlug := map[string]string{}
	fileIDBySlugName := map[string]string{} // "slug\x00filename" -> synthetic file id
	for _, vs := range rep.Songs {
		if vs.Slug == "" {
			return bandManifest{}, fmt.Errorf("%w: repertoire song %q has an empty slug", ErrInvalidInput, vs.Title)
		}
		if _, dup := songIDBySlug[vs.Slug]; dup {
			return bandManifest{}, fmt.Errorf("%w: duplicate song slug %q", ErrInvalidInput, vs.Slug)
		}
		sid := "s-" + vs.Slug
		songIDBySlug[vs.Slug] = sid
		ms := manifestSong{
			ID: sid, Title: vs.Title, Artist: vs.Artist, Key: vs.Key,
			Tempo: vs.Tempo, Meter: vs.Meter, Tags: vs.Tags, Notes: vs.Notes,
		}
		for i, vf := range vs.Files {
			fid := fmt.Sprintf("f-%s-%d", vs.Slug, i)
			fileIDBySlugName[vs.Slug+"\x00"+vf.Filename] = fid
			ms.Files = append(ms.Files, manifestFile{
				ID: fid, Filename: vf.Filename, ContentType: vf.ContentType, Size: vf.Size,
				BlobHash: vf.BlobHash, DisplayOrder: vf.DisplayOrder, Generated: vf.Generated,
				Revision: vf.Revision, ChartSource: vf.ChartSource,
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
			return bandManifest{}, fmt.Errorf("%w: annotations for unknown song slug %q", ErrInvalidInput, slug)
		}
		var va v2Annotations
		if err := json.Unmarshal(raw, &va); err != nil {
			return bandManifest{}, fmt.Errorf("%w: %s is not valid JSON: %v", ErrInvalidInput, name, err)
		}
		ann := manifestAnnots{}
		for _, vl := range va.Layers {
			fileID := ""
			if vl.File != "" {
				fileID, ok = fileIDBySlugName[slug+"\x00"+vl.File]
				if !ok {
					return bandManifest{}, fmt.Errorf("%w: layer %q references unknown file %q in song %q", ErrInvalidInput, vl.ID, vl.File, slug)
				}
			}
			owner, err := ownerFromName(vl.Owner)
			if err != nil {
				return bandManifest{}, err
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
				return bandManifest{}, err
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
			return bandManifest{}, fmt.Errorf("%w: setlists.json is not valid JSON: %v", ErrInvalidInput, err)
		}
	}
	for _, vsl := range sls.Setlists {
		msl := manifestSetlist{Name: vsl.Name, EventDate: vsl.EventDate, Venue: vsl.Venue, Notes: vsl.Notes}
		for _, it := range vsl.Items {
			songID, ok := songIDBySlug[it.Song]
			if !ok {
				return bandManifest{}, fmt.Errorf("%w: setlist %q references unknown song slug %q", ErrInvalidInput, vsl.Name, it.Song)
			}
			msl.Items = append(msl.Items, manifestItem{
				SongRef: songID, KeyOverride: it.KeyOverride, TempoOverride: it.TempoOverride,
				Notes: it.Notes, OnCall: it.OnCall, TransposeChords: it.TransposeChords,
			})
		}
		man.Setlists = append(man.Setlists, msl)
	}

	// Cues (optional).
	if raw, ok := entries["cues.json"]; ok {
		var cf v2CuesFile
		if err := json.Unmarshal(raw, &cf); err != nil {
			return bandManifest{}, fmt.Errorf("%w: cues.json is not valid JSON: %v", ErrInvalidInput, err)
		}
		for _, s := range cf.Selections {
			songID, ok := songIDBySlug[s.Song]
			if !ok {
				return bandManifest{}, fmt.Errorf("%w: file selection references unknown song slug %q", ErrInvalidInput, s.Song)
			}
			refs := make([]string, 0, len(s.Files))
			for _, fn := range s.Files {
				fid, ok := fileIDBySlugName[s.Song+"\x00"+fn]
				if !ok {
					return bandManifest{}, fmt.Errorf("%w: file selection references unknown file %q in song %q", ErrInvalidInput, fn, s.Song)
				}
				refs = append(refs, fid)
			}
			man.FileSelections = append(man.FileSelections, manifestSelection{MemberUsername: s.Member, SongRef: songID, FileRefs: refs})
		}
		for _, c := range cf.Cues {
			songID, ok := songIDBySlug[c.Song]
			if !ok {
				return bandManifest{}, fmt.Errorf("%w: song cues reference unknown song slug %q", ErrInvalidInput, c.Song)
			}
			man.SongCues = append(man.SongCues, manifestCue{MemberUsername: c.Member, SongRef: songID, Cues: c.Cues})
		}
	}

	return man, nil
}

// --- helpers -------------------------------------------------------------------------

// slugify lowercases, keeps [a-z0-9], and collapses every other run to a single '-'.
func slugify(s string) string {
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

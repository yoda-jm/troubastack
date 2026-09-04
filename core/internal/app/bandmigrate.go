package app

// The legacy→canonical migration (T134 phase 2, stage C, ⟨F3⟩ BRIDGE). The real band directories were
// written in the seed's FOLDER VOCABULARY — `admin` beside `members`, `display`, prose `role` (the
// instrument), `conductor` — and their repertoire declares songs but derives files by globbing. This
// turns such a folder into a CANONICAL v2 directory (one vocabulary, `members[]` with `displayName` + the
// role enum + `plays`; a declared `files[]`).
//
// It is a bridge with an end, not a permanent translator: cmd/seed calls it on-read so nothing breaks in
// the window before the folders are rewritten, WARNING (naming the folder) when it fires; stage D rewrites
// the folders on disk and DELETES this file. It is IDEMPOTENT — a folder already at formatVersion 2 passes
// through untouched, so migrate-on-read of a migrated folder is a no-op.

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"troubastack/core/internal/app/blob"
)

// legacyBand / legacyMember mirror the seed's band.json folder vocabulary (core/cmd/seed). role is FREE
// TEXT (the instrument); conductor is a promotion flag.
type legacyBand struct {
	Name      string         `json:"name"`
	Shortname string         `json:"shortname"`
	Kind      string         `json:"kind"`
	Notes     string         `json:"notes"`
	Members   []legacyMember `json:"members"`
	Admin     legacyMember   `json:"admin"`
}

type legacyMember struct {
	Username  string `json:"username"`
	Display   string `json:"display"`
	Role      string `json:"role"`
	Conductor bool   `json:"conductor,omitempty"`
}

type legacyRepertoire struct {
	Songs []struct {
		Slug   string   `json:"slug"`
		Title  string   `json:"title"`
		Artist string   `json:"artist"`
		Key    string   `json:"key"`
		Tempo  int      `json:"tempo"`
		Meter  string   `json:"meter"`
		Notes  string   `json:"notes"`
		Tags   []string `json:"tags"`
	} `json:"songs"`
}

// MigrateLegacyFolder reads a band directory from fsys and returns its CANONICAL v2 entries (name→bytes).
// wasLegacy reports whether a translation happened (false = the folder was already canonical and passed
// through). A canonical folder round-trips unchanged (idempotent).
func MigrateLegacyFolder(fsys fs.FS) (entries map[string][]byte, wasLegacy bool, err error) {
	bandRaw, err := fs.ReadFile(fsys, "band.json")
	if err != nil {
		return nil, false, fmt.Errorf("%w: migrate: no band.json", ErrInvalidInput)
	}
	// Detect legacy VOCAB, not the formatVersion stamp: a scratch migration left the real libraries
	// stamped formatVersion:2 while still carrying the folder vocabulary (a separate `admin` block, member
	// `display`/`conductor` keys), which the reader would mishandle silently. Trusting the stamp would
	// pass those through untranslated. A truly-canonical folder has none of those tells and passes through
	// via readCanonicalDir (declared-only, so strays are excluded — Gap B).
	if !looksLegacy(bandRaw) {
		e, werr := readCanonicalDir(fsys)
		return e, false, werr
	}

	var lb legacyBand
	if err := json.Unmarshal(bandRaw, &lb); err != nil {
		return nil, false, fmt.Errorf("%w: migrate: band.json is not valid JSON: %v", ErrInvalidInput, err)
	}
	entries = map[string][]byte{}

	// band.json: fold admin into members[]; display→displayName; prose role→plays; role becomes the enum.
	// shortname/kind/notes are carried through as reader-ignored keys (so `make band=<shortname>` survives).
	v2b := v2Band{FormatVersion: BandExportFormatVersionV2, Name: lb.Name, Shortname: lb.Shortname, Kind: lb.Kind, Notes: lb.Notes}
	appendMember := func(m legacyMember, role string) {
		v2b.Members = append(v2b.Members, v2Member{
			Username: m.Username, DisplayName: m.Display, Role: role, Plays: m.Role,
		})
	}
	if lb.Admin.Username != "" {
		appendMember(lb.Admin, string(RoleAdmin))
	}
	for _, m := range lb.Members {
		role := string(RoleMember)
		if m.Conductor {
			role = string(RoleConductor)
		}
		appendMember(m, role)
	}
	b, err := marshalIndent(v2b)
	if err != nil {
		return nil, false, err
	}
	entries["band.json"] = b

	// repertoire.json: declared songs; DERIVE files[] by globbing (the migration derives, the format
	// declares — amendment 3). PDFs are files; *.txt are generated charts whose bytes ARE their source
	// (⟨F1⟩). blobHash is computed from the bytes on disk.
	var lr legacyRepertoire
	if raw, rerr := fs.ReadFile(fsys, "repertoire.json"); rerr == nil {
		if err := json.Unmarshal(raw, &lr); err != nil {
			return nil, false, fmt.Errorf("%w: migrate: repertoire.json is not valid JSON: %v", ErrInvalidInput, err)
		}
	}
	rep := v2Repertoire{}
	for _, s := range lr.Songs {
		slug := s.Slug
		if slug == "" {
			slug = slugify(s.Title)
		}
		vs := v2Song{Slug: slug, Title: s.Title, Artist: s.Artist, Key: s.Key, Tempo: s.Tempo, Meter: s.Meter, Notes: s.Notes, Tags: s.Tags}
		files, ferr := migrateSongFiles(fsys, slug)
		if ferr != nil {
			return nil, false, ferr
		}
		order := 0
		for _, mf := range files {
			mf.file.DisplayOrder = order
			order++
			vs.Files = append(vs.Files, mf.file)
			entries[slug+"/"+mf.file.Filename] = mf.data
		}
		rep.Songs = append(rep.Songs, vs)
	}
	if b, err := marshalIndent(rep); err != nil {
		return nil, false, err
	} else {
		entries["repertoire.json"] = b
	}

	// setlists.json is already slug-based (the legacy shape IS the v2 shape) — pass through verbatim.
	if raw, rerr := fs.ReadFile(fsys, "setlists.json"); rerr == nil {
		entries["setlists.json"] = raw
	}
	// cues.json + annotations/<slug>.json pass through if present: a partially-migrated ("hybrid") folder
	// may already carry real annotations (a scratch migration added them), and translating the member
	// vocab must NOT drop them. A stray top-level annotations.json (not annotations/<slug>.json) is NOT
	// read, so it is excluded. Only annotations for a DECLARED slug survive parseV2 on import.
	if raw, rerr := fs.ReadFile(fsys, "cues.json"); rerr == nil {
		entries["cues.json"] = raw
	}
	declared := map[string]bool{}
	for _, s := range rep.Songs {
		declared[s.Slug] = true
	}
	if annEntries, derr := fs.ReadDir(fsys, "annotations"); derr == nil {
		for _, e := range annEntries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
				continue
			}
			slug := strings.TrimSuffix(e.Name(), ".json")
			if !declared[slug] {
				continue // an annotations file for an undeclared song — dropped, not carried
			}
			b, rerr := fs.ReadFile(fsys, "annotations/"+e.Name())
			if rerr != nil {
				return nil, false, fmt.Errorf("%w: migrate: annotations/%s: %v", ErrInvalidInput, e.Name(), rerr)
			}
			entries["annotations/"+e.Name()] = b
		}
	}

	return entries, true, nil
}

type migratedFile struct {
	file v2File
	data []byte
}

// migrateSongFiles globs a song's `<slug>/` directory: *.pdf become plain files, *.txt become generated
// charts (bytes = source). Ordered by the seed's score<aux<part priority then name, for a stable
// displayOrder. A song directory with no files is allowed (a repertoire-only song).
func migrateSongFiles(fsys fs.FS, slug string) ([]migratedFile, error) {
	pdfs, _ := fs.Glob(fsys, slug+"/*.pdf")
	txts, _ := fs.Glob(fsys, slug+"/*.txt")
	sort.SliceStable(pdfs, func(i, j int) bool {
		if pi, pj := scorePriorityMigrate(pdfs[i]), scorePriorityMigrate(pdfs[j]); pi != pj {
			return pi < pj
		}
		return pdfs[i] < pdfs[j]
	})
	// lyrics.txt first, then the rest sorted — matching the seed's text-chart ordering.
	sort.SliceStable(txts, func(i, j int) bool {
		li, lj := strings.EqualFold(path.Base(txts[i]), "lyrics.txt"), strings.EqualFold(path.Base(txts[j]), "lyrics.txt")
		if li != lj {
			return li
		}
		return txts[i] < txts[j]
	})
	var out []migratedFile
	for _, p := range pdfs {
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		name := safeFilename(path.Base(p))
		out = append(out, migratedFile{
			file: v2File{Filename: name, ContentType: "application/pdf", Size: int64(len(data)), BlobHash: blob.HashOf(data)},
			data: data,
		})
	}
	for _, p := range txts {
		data, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		name := safeFilename(path.Base(p))
		out = append(out, migratedFile{
			// ⟨F1⟩: a generated chart's bytes ARE its source; blobHash hashes the source; import renders.
			file: v2File{Filename: name, ContentType: "application/pdf", Size: int64(len(data)), BlobHash: blob.HashOf(data), Generated: true},
			data: data,
		})
	}
	return out, nil
}

// scorePriorityMigrate mirrors cmd/seed's scorePriority: full scores (0) before aux (1) before parts (2).
func scorePriorityMigrate(p string) int {
	l := strings.ToLower(path.Base(p))
	for _, k := range []string{"bass", "basse", "drum", "batterie", "percussion", "piano", "guitar", "guitare", "flute", "flûte", "musicien"} {
		if strings.Contains(l, k) {
			return 2
		}
	}
	for _, k := range []string{"traduction", "paroles", "gestes", "livret", "lyrics", "resolution", ".docx"} {
		if strings.Contains(l, k) {
			return 1
		}
	}
	return 0
}

// looksLegacy reports the folder-vocab tells that mean a band.json must be translated, REGARDLESS of a
// (possibly bogus) formatVersion stamp: a separate `admin` block, or a member keyed by `display` or
// `conductor`. A canonical band.json folds admin into members[] and uses `displayName`/`plays`, so it has
// none of these. This is what makes migrate-on-read robust to a folder stamped v2 but never converted.
func looksLegacy(bandJSON []byte) bool {
	var peek struct {
		Admin   json.RawMessage              `json:"admin"`
		Members []map[string]json.RawMessage `json:"members"`
	}
	if err := json.Unmarshal(bandJSON, &peek); err != nil {
		return false
	}
	if len(peek.Admin) > 0 && string(peek.Admin) != "null" {
		return true
	}
	for _, m := range peek.Members {
		if _, ok := m["display"]; ok {
			return true
		}
		if _, ok := m["conductor"]; ok {
			return true
		}
	}
	return false
}

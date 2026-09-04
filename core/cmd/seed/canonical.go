package main

// T134 phase 2 stage C: build a demo group's CANONICAL v2 band directory in memory (⟨F2⟩), so the demo
// seeds through the SAME pack→import→passwords path as a real band — the completeness test. The demo
// annotations are still computed by the anchor-coupled builders (annotations.go); we call them with the
// canonical refs (fileID = filename, ownerID/conductorID = username) so their output IS canonical, then
// serialize to annotations/<slug>.json. Generated (text) charts store their SOURCE (⟨F1⟩).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"troubastack/core/internal/app"
	"troubastack/core/internal/app/blob"
)

// --- canonical v2 JSON shapes (the file/owner keys the reader expects; NOT the seed's fileId/ownerId
// wire shape). cmd/seed emits these as bytes; app.parseV2 reads them. ---

type canonBand struct {
	FormatVersion int           `json:"formatVersion"`
	Name          string        `json:"name"`
	Members       []canonMember `json:"members"`
}
type canonMember struct {
	Username    string `json:"username"`
	DisplayName string `json:"displayName,omitempty"`
	Role        string `json:"role"`
	Plays       string `json:"plays,omitempty"`
}
type canonRepertoire struct {
	Songs []canonSong `json:"songs"`
}
type canonSong struct {
	Slug   string      `json:"slug"`
	Title  string      `json:"title"`
	Artist string      `json:"artist,omitempty"`
	Key    string      `json:"key,omitempty"`
	Tempo  int         `json:"tempo,omitempty"`
	Meter  string      `json:"meter,omitempty"`
	Notes  string      `json:"notes,omitempty"`
	Tags   []string    `json:"tags,omitempty"`
	Files  []canonFile `json:"files,omitempty"`
}
type canonFile struct {
	Filename     string `json:"filename"`
	ContentType  string `json:"contentType,omitempty"`
	Size         int64  `json:"size,omitempty"`
	BlobHash     string `json:"blobHash,omitempty"`
	DisplayOrder int    `json:"displayOrder,omitempty"`
	Generated    bool   `json:"generated,omitempty"`
}
type canonAnnotations struct {
	Layers  []canonLayer  `json:"layers"`
	Objects []canonObject `json:"objects"`
}
type canonLayer struct {
	ID        string `json:"id"`
	File      string `json:"file,omitempty"`
	Name      string `json:"name,omitempty"`
	Owner     string `json:"owner,omitempty"`
	Zone      string `json:"zone,omitempty"`
	Order     int    `json:"order,omitempty"`
	Access    string `json:"access,omitempty"`
	Mandatory bool   `json:"mandatory,omitempty"`
	RoleTag   string `json:"roleTag,omitempty"`
}
type canonObject struct {
	UUID   string       `json:"uuid"`
	Layer  string       `json:"layer"`
	Type   string       `json:"type"`
	Points []canonPoint `json:"points,omitempty"`
	Page   int          `json:"page,omitempty"`
	Text   string       `json:"text,omitempty"`
	Style  canonStyle   `json:"style"`
}
type canonPoint struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type canonStyle struct {
	Color    string  `json:"color,omitempty"`
	Opacity  float64 `json:"opacity,omitempty"`
	Width    float64 `json:"width,omitempty"`
	FontSize float64 `json:"fontSize,omitempty"`
	Fill     *bool   `json:"fill,omitempty"`
	Stroke   *bool   `json:"stroke,omitempty"`
	Blend    string  `json:"blend,omitempty"`
}
type canonSetlistsFile struct {
	Setlists []canonSetlist `json:"setlists"`
}
type canonSetlist struct {
	Name      string           `json:"name"`
	EventDate string           `json:"eventDate,omitempty"`
	Venue     string           `json:"venue,omitempty"`
	Notes     string           `json:"notes,omitempty"`
	Items     []canonSetlistIt `json:"items"`
}
type canonSetlistIt struct {
	Song            string `json:"song"`
	KeyOverride     string `json:"keyOverride,omitempty"`
	TempoOverride   int    `json:"tempoOverride,omitempty"`
	Notes           string `json:"notes,omitempty"`
	OnCall          bool   `json:"onCall,omitempty"`
	TransposeChords bool   `json:"transposeChords,omitempty"`
}
type canonCuesFile struct {
	Selections []canonSelection `json:"selections,omitempty"`
	Cues       []canonCueEntry  `json:"cues,omitempty"`
}
type canonSelection struct {
	Member string   `json:"member"`
	Song   string   `json:"song"`
	Files  []string `json:"files"`
}
type canonCueEntry struct {
	Member string      `json:"member"`
	Song   string      `json:"song"`
	Cues   []canonCue1 `json:"cues"`
}
type canonCue1 struct {
	Icon  string `json:"icon"`
	Color string `json:"color,omitempty"`
}

// groupToCanonical builds the canonical v2 entry set (name→bytes) for a demo group.
func groupToCanonical(g groupDef, people map[string]person) (map[string][]byte, error) {
	entries := map[string][]byte{}

	// band.json: admin folded into members[]; role enum from admin/conductor; the human role label
	// becomes `plays` documentation.
	band := canonBand{FormatVersion: 2, Name: g.name}
	roleOf := func(username string) string {
		switch {
		case username == g.admin:
			return "admin"
		case username == g.conductor:
			return "conductor"
		default:
			return "member"
		}
	}
	members := append([]string{g.admin}, g.members...)
	for _, u := range members {
		band.Members = append(band.Members, canonMember{
			Username: u, DisplayName: people[u].display, Role: roleOf(u), Plays: people[u].role,
		})
	}
	// conductor identity for the annotation builders (username), admin as the fallback.
	conductorID := g.conductor
	if conductorID == "" {
		conductorID = g.admin
	}
	userID := map[string]string{} // username -> username (identity): builder "ids" ARE usernames here.
	for _, u := range members {
		userID[u] = u
	}

	rep := canonRepertoire{}
	cues := canonCuesFile{}
	for _, s := range g.songs {
		slug := slugifySeed(s.title)
		cs := canonSong{Slug: slug, Title: s.title, Artist: s.artist, Key: s.key, Tempo: s.tempo, Meter: s.meter, Notes: s.notes, Tags: s.tags}

		// Files: the PDF pool, then the text chart (generated, source bytes ⟨F1⟩).
		srcs := s.files
		if len(srcs) == 0 && s.src.cacheName != "" {
			srcs = []pdfSource{s.src}
		}
		used := map[string]bool{}
		fileByDocTitle := map[string]string{} // docTitle -> emitted filename (for part annotation dispatch)
		order := 0
		for _, src := range srcs {
			res, err := resolvePDF(src)
			if err != nil {
				return nil, err
			}
			name := uniqueSeedName(safeSeedName(src.filename()), used)
			fileByDocTitle[src.docTitle] = name
			cs.Files = append(cs.Files, canonFile{
				Filename: name, ContentType: "application/pdf", Size: int64(len(res.data)),
				BlobHash: blob.HashOf(res.data), DisplayOrder: order, Generated: false,
			})
			entries[slug+"/"+name] = res.data
			order++
		}
		firstPDF := ""
		if len(cs.Files) > 0 {
			firstPDF = cs.Files[0].Filename
		}
		// Text chart: source bytes, generated:true; rendered on import.
		if s.textChartPath != "" {
			source, err := readSeedFile(s.textChartPath)
			if err != nil {
				return nil, fmt.Errorf("read text chart %q: %w", s.textChartPath, err)
			}
			name := uniqueSeedName(safeSeedName(filepath.Base(s.textChartPath)), used)
			cs.Files = append(cs.Files, canonFile{
				Filename: name, ContentType: "application/pdf", Size: int64(len(source)),
				BlobHash: blob.HashOf(source), DisplayOrder: order, Generated: true,
			})
			entries[slug+"/"+name] = source
			order++
		}
		rep.Songs = append(rep.Songs, cs)

		// Annotations: dispatch the anchored builders with canonical refs, merge, serialize.
		if firstPDF != "" {
			ann := buildSongAnnotationsCanon(slug, s, firstPDF, fileByDocTitle, userID, conductorID)
			if len(ann.Layers) > 0 || len(ann.Objects) > 0 {
				b, err := json.MarshalIndent(ann, "", "  ")
				if err != nil {
					return nil, err
				}
				entries["annotations/"+slug+".json"] = b
			}
		}

		// Cues + file selections, per member, by username + slug + filename.
		for u, cl := range s.cuesFor {
			ce := canonCueEntry{Member: u, Song: slug}
			for _, c := range cl {
				ce.Cues = append(ce.Cues, canonCue1{Icon: c.icon, Color: c.color})
			}
			if len(ce.Cues) > 0 {
				cues.Cues = append(cues.Cues, ce)
			}
		}
		for u, idxs := range s.myFilesFor {
			var names []string
			for _, i := range idxs {
				if i >= 0 && i < len(cs.Files) {
					names = append(names, cs.Files[i].Filename)
				}
			}
			if len(names) > 0 {
				cues.Selections = append(cues.Selections, canonSelection{Member: u, Song: slug, Files: names})
			}
		}
	}

	// Setlists: map each override's song TITLE to its slug.
	slugByTitle := map[string]string{}
	for _, s := range g.songs {
		slugByTitle[s.title] = slugifySeed(s.title)
	}
	sls := canonSetlistsFile{}
	for _, sl := range g.setlists {
		csl := canonSetlist{Name: sl.name, EventDate: sl.eventDate, Venue: sl.venue, Notes: sl.notes}
		items := sl.overrides
		if len(sl.items) > 0 {
			items = sl.items
		}
		for _, ov := range items {
			slug, ok := slugByTitle[ov.song]
			if !ok {
				return nil, fmt.Errorf("setlist %q references unknown song %q", sl.name, ov.song)
			}
			csl.Items = append(csl.Items, canonSetlistIt{
				Song: slug, KeyOverride: ov.keyOverride, TempoOverride: ov.tempoOverride,
				Notes: ov.notes, OnCall: ov.onCall, TransposeChords: ov.transposeChords,
			})
		}
		sls.Setlists = append(sls.Setlists, csl)
	}

	var err error
	if entries["band.json"], err = jsonIndent(band); err != nil {
		return nil, err
	}
	if entries["repertoire.json"], err = jsonIndent(rep); err != nil {
		return nil, err
	}
	if entries["setlists.json"], err = jsonIndent(sls); err != nil {
		return nil, err
	}
	if len(cues.Selections) > 0 || len(cues.Cues) > 0 {
		if entries["cues.json"], err = jsonIndent(cues); err != nil {
			return nil, err
		}
	}
	return entries, nil
}

// buildSongAnnotationsCanon dispatches the title-keyed builders (main + per-part) with canonical refs and
// merges them. Mirrors the dispatch that used to live in seedGroup, but fileID = filename and the owner
// "ids" are usernames, so the builders' output is already canonical.
func buildSongAnnotationsCanon(slug string, s songDef, firstPDF string, fileByDocTitle map[string]string, userID map[string]string, conductorID string) canonAnnotations {
	var im annotationsImport
	switch s.title {
	case "The Open Road":
		im = buildOpenRoadAnnotations(slug, firstPDF, userID, conductorID, mustAnchors("open-road-leadsheet"))
	case "House of the Rising Sun":
		im = buildBandChartAnnotations(slug, firstPDF, s.title, userID, conductorID, mustAnchors("house-rising-sun-tab"))
	case "Amazing Grace":
		im = buildBandChartAnnotations(slug, firstPDF, s.title, userID, conductorID, mustAnchors("amazing-grace"))
	case "Greensleeves":
		im = buildBandChartAnnotations(slug, firstPDF, s.title, userID, conductorID, mustAnchors("greensleeves"))
	case "Canon in D":
		im = buildCanonAnnotations(slug, firstPDF, userID, conductorID, mustAnchors("canon-violin1"))
	case "Eine kleine Nachtmusik":
		im = buildEineKleineAnnotations(slug, firstPDF, userID, conductorID, mustAnchors("ek-violin1"))
	}
	out := wireToCanon(im)

	// Per-part annotations on the OTHER files (B11), targeted by docTitle → filename.
	part := func(docTitle string, build func(fileID string) annotationsImport) {
		if fn := fileByDocTitle[docTitle]; fn != "" {
			merged := wireToCanon(build(fn))
			out.Layers = append(out.Layers, merged.Layers...)
			out.Objects = append(out.Objects, merged.Objects...)
		}
	}
	switch s.title {
	case "House of the Rising Sun":
		part("Drums", func(f string) annotationsImport {
			return buildDrumPartAnnotations(slug, f, mustAnchors("house-rising-sun-drums"))
		})
	case "The Open Road":
		part("Guitar", func(f string) annotationsImport {
			return buildOpenRoadGuitarAnnotations(slug, f, userID, conductorID, mustAnchors("open-road-guitar"))
		})
	case "Canon in D":
		part("Full score", func(f string) annotationsImport {
			return buildScoreConductorAnnotations(slug, f, "canon", conductorID, mustAnchors("canon-score"))
		})
		part("Cello", func(f string) annotationsImport { return buildCelloBowingAnnotations(slug, f, "canon", userID) })
	case "Eine kleine Nachtmusik":
		part("Full score", func(f string) annotationsImport {
			return buildScoreConductorAnnotations(slug, f, "ek", conductorID, mustAnchors("ek-score"))
		})
		part("Cello", func(f string) annotationsImport { return buildCelloBowingAnnotations(slug, f, "ek", userID) })
	}
	return out
}

// wireToCanon converts a builder's wire annotations (fileId/ownerId) into the canonical file/owner shape.
func wireToCanon(im annotationsImport) canonAnnotations {
	var out canonAnnotations
	for _, l := range im.Layers {
		out.Layers = append(out.Layers, canonLayer{
			ID: l.ID, File: l.FileID, Name: l.Name, Owner: l.OwnerID, Zone: l.Zone,
			Order: l.Order, Access: l.Access, Mandatory: l.Mandatory, RoleTag: l.RoleTag,
		})
	}
	for _, o := range im.Objects {
		co := canonObject{
			UUID: o.UUID, Layer: o.LayerID, Type: o.Type, Page: o.Page, Text: o.Text,
			Style: canonStyle{Color: o.Style.Color, Opacity: o.Style.Opacity, Width: o.Style.Width,
				FontSize: o.Style.FontSize, Fill: o.Style.Fill, Stroke: o.Style.Stroke, Blend: o.Style.Blend},
		}
		for _, p := range o.Points {
			co.Points = append(co.Points, canonPoint{X: p.X, Y: p.Y})
		}
		out.Objects = append(out.Objects, co)
	}
	return out
}

func jsonIndent(v any) ([]byte, error) { return json.MarshalIndent(v, "", "  ") }

// slugifySeed mirrors app.slugify (lowercase, [a-z0-9], collapse other runs to '-').
func slugifySeed(s string) string {
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

func safeSeedName(name string) string {
	name = strings.Map(func(r rune) rune {
		if r == '/' || r == '\\' || r == 0 {
			return '-'
		}
		return r
	}, name)
	if name == "" || name == "." || name == ".." {
		return "file"
	}
	return name
}

func uniqueSeedName(name string, used map[string]bool) string {
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

func readSeedFile(path string) ([]byte, error) { return os.ReadFile(path) }

// --- seeding via import (T134 phase 2 stage C, ⟨P8⟩ import-first, passwords-second) ---

// importBand POSTs a .tband to the import endpoint with a dispositions map (username→"create"/…).
func (c *apiClient) importBand(zipBytes []byte, dispositions map[string]string, out any) error {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("file", "band.tband")
	if err != nil {
		return err
	}
	if _, err := fw.Write(zipBytes); err != nil {
		return err
	}
	if len(dispositions) > 0 {
		dj, _ := json.Marshal(dispositions)
		if err := mw.WriteField("dispositions", string(dj)); err != nil {
			return err
		}
	}
	if err := mw.Close(); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodPost, c.base+"/api/bands/import", &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return c.do(req, out)
}

// setMemberPassword gives a passwordless imported member a password over the PUBLIC API only (⟨P8⟩): the
// admin issues a reset token, then it is redeemed. There is no direct set-password call.
func setMemberPassword(admin *apiClient, bandID, userID, password string) error {
	var tok struct {
		Token string `json:"token"`
	}
	if err := admin.postJSON("/api/bands/"+bandID+"/members/"+userID+"/password-reset", nil, &tok); err != nil {
		return err
	}
	pub := newAPIClient(admin.base) // the redeem endpoint is public (no session)
	return pub.postJSON("/api/password-reset/"+tok.Token, map[string]string{"newPassword": password}, nil)
}

// seedGroupViaImport seeds one group the T134-phase-2 way: build its canonical band directory (a demo
// group in memory; a local band read directly from its canonical folder), pack it, register the admin (the
// importer), import with every other member `create`d, then give those members passwords. Import FIRST,
// passwords SECOND — pre-creating members would make them consent-required and drop their personal
// content (⟨P8⟩).
func seedGroupViaImport(addr, password string, g groupDef, people map[string]person) (seededGroup, error) {
	var entries map[string][]byte
	if g.folderPath != "" {
		var err error
		entries, err = app.ReadCanonicalDir(os.DirFS(g.folderPath))
		if err != nil {
			return seededGroup{}, fmt.Errorf("read %s: %w", g.folderPath, err)
		}
	} else {
		var err error
		entries, err = groupToCanonical(g, people)
		if err != nil {
			return seededGroup{}, fmt.Errorf("build canonical %q: %w", g.name, err)
		}
	}

	zipBytes, size, err := app.PackEntries(entries)
	if err != nil {
		return seededGroup{}, fmt.Errorf("pack %q: %w", g.name, err)
	}

	// Register ONLY the admin (the importer), who needs a password to drive the import.
	if err := registerUser(addr, person{username: g.admin, display: people[g.admin].display, role: people[g.admin].role}, password); err != nil {
		return seededGroup{}, err
	}
	admin, err := login(addr, g.admin, password)
	if err != nil {
		return seededGroup{}, err
	}

	disp := map[string]string{}
	for _, u := range g.members {
		if u != g.admin {
			disp[u] = "create"
		}
	}
	var report struct {
		Band struct {
			ID string `json:"id"`
		} `json:"band"`
		Created                []string `json:"created"`
		Songs, Files, Setlists int
		DroppedLayers          int `json:"droppedLayers"`
	}
	if err := admin.importBand(zipBytes, disp, &report); err != nil {
		return seededGroup{}, fmt.Errorf("import %q: %w", g.name, err)
	}
	if report.DroppedLayers > 0 {
		return seededGroup{}, fmt.Errorf("import %q dropped %d layers (a member's personal content was lost — import ordering bug)", g.name, report.DroppedLayers)
	}
	fmt.Printf("   imported %q (%d bytes): %d songs, %d files, %d setlists; created %v\n", g.name, size, report.Songs, report.Files, report.Setlists, report.Created)

	// ⟨P8⟩ passwords, second: give each created member the demo password via the reset flow.
	ids, _, err := memberIDs(admin, report.Band.ID, g.admin)
	if err != nil {
		return seededGroup{}, err
	}
	for _, u := range report.Created {
		uid, ok := ids[u]
		if !ok {
			continue
		}
		if err := setMemberPassword(admin, report.Band.ID, uid, password); err != nil {
			return seededGroup{}, fmt.Errorf("set password for %s: %w", u, err)
		}
	}

	titles := make([]string, 0, len(g.songs))
	for _, s := range g.songs {
		titles = append(titles, s.title)
	}
	return seededGroup{def: g, songs: titles}, nil
}

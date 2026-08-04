// Command seed populates a running TroubaCore server with a demo dataset over
// HTTP — users, bands, an orchestra, songs, and a sheet-music PDF per song — then
// prints a clear guide for browsing it. It drives the public REST API exactly as
// the SPA does (register → login → create band → invite → accept → create song →
// upload file), so it doubles as an end-to-end smoke test of the "normal web"
// surface.
//
// It is resilient: PDFs are fetched from public-domain URLs when possible and
// otherwise generated locally, so the seed NEVER fails for lack of internet. It is
// idempotent-ish: a user/band that already exists is reused rather than erroring.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

func main() {
	addr := flag.String("addr", "http://localhost:8080", "base URL of the running troubacore server")
	password := flag.String("password", "demo", "password set for every demo user")
	flag.Parse()

	if err := run(*addr, *password); err != nil {
		log.Fatalf("seed: %v", err)
	}
}

// person is a demo user definition.
type person struct {
	username string
	display  string
	role     string // human-readable role label for the guide (instrument/part)
}

// songDef pairs a song title/artist + settable metadata with its PDF source(s).
//
// A song normally has one shared PDF (src). To showcase the multi-file /
// per-member "my files" feature, a song may instead define several pool files
// (files): each is uploaded to the shared pool with a distinct human title
// (pdfSource.docTitle). When files is set it takes precedence over src.
//
// myFilesFor curates a specific member's PERSONAL ordered selection over those
// pool files: keyed by username, the value is a list of indices into files in
// the exact order that member should see them (a non-default subset+order). The
// seeder PUTs this via the my-files endpoint as that user.
type songDef struct {
	title      string
	artist     string
	key        string // musical key (settable on the song page)
	tempo      int    // BPM
	tags       []string
	notes      string
	src        pdfSource
	files      []pdfSource      // optional multi-file shared pool (overrides src)
	myFilesFor map[string][]int // username -> ordered indices into files (custom my-files)
	// cuesFor seeds each member's PERSONAL song cues (T50): username -> that member's
	// ordered icon+color list. Self-only, so the seeder PUTs each list as that user;
	// they ride the member's per-member bake (visible when baking "my parts").
	cuesFor map[string][]cueDef
	// textChartPath is an optional committed chart-dialect source (docs/demo-charts/*.chart)
	// added to the pool as a REAL text chart (T19) — POST /text-charts renders it to a PDF
	// server-side, so the demo shows the text-chart file type, not only uploaded PDFs (B10).
	textChartPath string
}

// cueDef is one personal song cue (T50): a curated glyph id + an optional "#rrggbb"
// tint ("" = neutral). See docs/tasks/T50-song-cues.md §2 for the id set.
type cueDef struct {
	icon  string
	color string
}

// overrideDef is a per-setlist-item override (settable on the setlist page).
type overrideDef struct {
	song          string // song title to match
	keyOverride   string
	tempoOverride int
	notes         string
}

// setlistDef is a setlist to create for a group, with optional item overrides.
type setlistDef struct {
	name      string
	eventDate string // ISO yyyy-mm-dd
	venue     string
	notes     string
	overrides []overrideDef
}

// groupDef is a band or orchestra to create.
type groupDef struct {
	name      string
	kind      string // "Band" or "Orchestra" (label only)
	admin     string // username of the admin/owner
	members   []string
	conductor string // optional: a member to promote to the "conductor" role
	songs     []songDef
	setlist   setlistDef
}

func run(addr, password string) error {
	if err := ensureAssetsDir(); err != nil {
		return err
	}

	people := []person{
		{"marie", "Marie", "singer (band admin)"},
		{"leo", "Leo", "guitar"},
		{"sasha", "Sasha", "bass"},
		{"maestro", "Anya", "conductor (orchestra admin)"},
		{"flora", "Flora", "flute"},
		{"cory", "Cory", "cello"},
	}

	groups := []groupDef{
		{
			name:      "The Troubadours",
			kind:      "Band",
			admin:     "marie",
			members:   []string{"leo", "sasha"},
			conductor: "leo", // promoted to the conductor role; owns the conductor cues
			songs: []songDef{
				// An ORIGINAL demo song with a REAL committed chart (lead sheet + guitar
				// tab) from docs/demo-charts — the FLAGSHIP annotation showcase (see
				// buildOpenRoadAnnotations). All demo content is copyright-safe: original
				// or public-domain (DEMO-VID Part A) — no copyrighted song titles/lyrics.
				{title: "The Open Road", artist: "", key: "G", tempo: 92, tags: []string{"original", "demo"}, notes: "Original demo song — lead sheet + a guitarist's intro-riff sheet. Capo 2.",
					files: []pdfSource{
						{cacheName: "open-road-leadsheet.pdf", localPath: "../docs/demo-charts/open-road-leadsheet.pdf", docTitle: "Lead sheet", title: "The Open Road", subtitle: "original demo song", pages: 1},
						{cacheName: "open-road-guitar.pdf", localPath: "../docs/demo-charts/open-road-guitar.pdf", docTitle: "Guitar", title: "The Open Road — Guitar", subtitle: "intro riff", pages: 1},
					},
					// A REAL text chart alongside the PDFs — the demo shows the T19 chart type
					// (server-rendered from chart-dialect source), not only PDFs (B10).
					textChartPath: "../docs/demo-charts/open-road-lyrics.chart",
					// Personal cues (T50): Marie sings + plays the RED electric (VLL's canonical
					// "mic + red guitar" example, shown per-member on setlist rows + in the app).
					cuesFor: map[string][]cueDef{
						"marie": {{icon: "mic"}, {icon: "guitar-electric", color: "#e11d48"}},
						"leo":   {{icon: "guitar-acoustic"}},
						"sasha": {{icon: "bass", color: "#2563eb"}},
					}},
				// TRADITIONAL / PUBLIC DOMAIN — the MULTI-FILE showcase: real committed
				// parts (a guitar tab + a drum groove) PLUS a rendered chord chart (text
				// chart). Members curate their own "my files" view; parts carry their own
				// per-file annotations (B11). Replaces the old copyrighted multi-file song.
				{title: "House of the Rising Sun", artist: "Traditional", key: "Am", tempo: 72, tags: []string{"folk", "public-domain"}, notes: "Traditional; soft 6/8, let the arpeggio ring.",
					files: []pdfSource{
						{cacheName: "house-rising-sun-tab.pdf", localPath: "../docs/demo-charts/house-rising-sun-tab.pdf", docTitle: "Guitar tab", title: "House of the Rising Sun — Guitar", subtitle: "traditional (guitar tab)", pages: 1},
						{cacheName: "house-rising-sun-drums.pdf", localPath: "../docs/demo-charts/house-rising-sun-drums.pdf", docTitle: "Drums", title: "House of the Rising Sun — Drums", subtitle: "traditional (drum groove)", pages: 1},
					},
					// The rendered chord chart (lead sheet) joins the pool after the parts.
					textChartPath: "../docs/demo-charts/house-of-the-rising-sun.chart",
					// Leo (guitar) curates his own focused view: just the guitar tab.
					myFilesFor: map[string][]int{
						"leo": {0},
					},
					cuesFor: map[string][]cueDef{
						"marie": {{icon: "mic"}},
						"leo":   {{icon: "guitar-acoustic"}},
						"sasha": {{icon: "bass", color: "#2563eb"}},
					}},
				// PUBLIC DOMAIN hymn (John Newton, 1779) — a committed lead sheet PLUS a
				// chord text chart.
				{title: "Amazing Grace", artist: "Traditional", key: "G", tempo: 72, tags: []string{"hymn", "public-domain"}, notes: "Newton, 1779; gentle 3/4.",
					src:           pdfSource{cacheName: "amazing-grace.pdf", localPath: "../docs/demo-charts/amazing-grace.pdf", docTitle: "Lead sheet", title: "Amazing Grace", subtitle: "words: John Newton, 1779 (public domain)", pages: 1},
					textChartPath: "../docs/demo-charts/amazing-grace.chart",
					cuesFor: map[string][]cueDef{
						"marie": {{icon: "mic"}},
						"sasha": {{icon: "keys", color: "#7c3aed"}},
					}},
			},
			setlist: setlistDef{
				name: "Sat @ The Anchor", eventDate: "2026-07-04", venue: "The Anchor Pub", notes: "60-minute set.",
				overrides: []overrideDef{
					{song: "House of the Rising Sun", keyOverride: "Bm", notes: "Lift it a tone for the room."},
					{song: "The Open Road", notes: "Encore — everyone in on the last chorus."},
				},
			},
		},
		{
			name:      "City Chamber Orchestra",
			kind:      "Orchestra",
			admin:     "maestro",
			members:   []string{"flora", "cory"},
			conductor: "flora", // promoted to the "conductor" role to show role management
			songs: []songDef{
				{title: "Eine kleine Nachtmusik", artist: "W. A. Mozart", key: "G", tempo: 140, tags: []string{"classical", "public-domain"}, notes: "Allegro; K. 525.",
					src: pdfSource{
						cacheName: "eine-kleine-nachtmusik.pdf",
						title:     "Eine kleine Nachtmusik",
						subtitle:  "W. A. Mozart (K. 525)",
						pages:     4,
						// Mutopia now ships this edition only as a multi-PDF .zip (no
						// stable single-PDF URL), so this is best-effort; the generated
						// fallback covers it when it 404s.
						urls: []string{
							"https://www.mutopiaproject.org/ftp/MozartWA/KV525/eine_kleine_nachtmusik/eine_kleine_nachtmusik-a4.pdf",
						},
					}},
				// PUBLIC DOMAIN (Pachelbel, 1680) — the orchestra's MULTI-PART showcase: real,
				// engraved per-instrument parts (Violin I, Viola, Cello) so multiple players each
				// open their own desk's part. Rendered from committed LilyPond source
				// (docs/demo-charts/lilypond/canon-*.ly). Replaces the fetched Bach Air (Pachelbel
				// is the more legible multi-part piece — DEMO-VID Part A slice 2).
				{title: "Canon in D", artist: "J. Pachelbel", key: "D", tempo: 56, tags: []string{"classical", "public-domain"}, notes: "Canon over the famous ground bass; Andante.",
					files: []pdfSource{
						{cacheName: "canon-violin1.pdf", localPath: "../docs/demo-charts/canon-violin1.pdf", docTitle: "Violin I", title: "Canon in D — Violin I", subtitle: "Pachelbel", pages: 1},
						{cacheName: "canon-viola.pdf", localPath: "../docs/demo-charts/canon-viola.pdf", docTitle: "Viola", title: "Canon in D — Viola", subtitle: "Pachelbel", pages: 1},
						{cacheName: "canon-cello.pdf", localPath: "../docs/demo-charts/canon-cello.pdf", docTitle: "Cello", title: "Canon in D — Cello", subtitle: "Pachelbel", pages: 1},
					},
					// Cory (cello) curates a personal view of just the cello part.
					myFilesFor: map[string][]int{
						"cory": {2},
					}},
			},
			setlist: setlistDef{
				name: "Spring Concert", eventDate: "2026-05-15", venue: "City Hall", notes: "Strings programme.",
				overrides: []overrideDef{
					{song: "Eine kleine Nachtmusik", tempoOverride: 132, notes: "Take the repeat."},
					{song: "Canon in D", notes: "Watch the tempo — don't rush the bass."},
				},
			},
		},
	}

	// 1. Register every user (idempotent: skip if already present).
	fmt.Println(">> registering demo users…")
	for _, p := range people {
		if err := registerUser(addr, p, password); err != nil {
			return err
		}
	}

	// Track how PDFs were obtained for the final report.
	var fetched, generated int

	// 2. Per group: admin creates it, invites members (who accept), then songs + PDFs.
	seeded := make([]seededGroup, 0, len(groups))
	for _, g := range groups {
		fmt.Printf(">> building %s %q (admin %s)…\n", g.kind, g.name, g.admin)
		sg, f, gen, err := seedGroup(addr, password, g)
		if err != nil {
			return err
		}
		fetched += f
		generated += gen
		seeded = append(seeded, sg)
	}

	fmt.Printf(">> done. PDFs: %d fetched (public domain), %d generated (fallback).\n\n", fetched, generated)
	printGuide(addr, password, people, seeded, fetched, generated)
	return nil
}

// seededGroup records what was created for the guide.
type seededGroup struct {
	def   groupDef
	songs []string // song titles actually present
}

func registerUser(addr string, p person, password string) error {
	c := newAPIClient(addr)
	err := c.postJSON("/api/auth/register", map[string]string{
		"username": p.username, "displayName": p.display, "password": password,
	}, nil)
	if err == nil {
		fmt.Printf("   + registered %s (%s)\n", p.username, p.display)
		return nil
	}
	if isConflict(err) {
		fmt.Printf("   = %s already exists, skipping\n", p.username)
		return nil
	}
	return fmt.Errorf("register %s: %w", p.username, err)
}

// login returns an authenticated client for username.
func login(addr, username, password string) (*apiClient, error) {
	c := newAPIClient(addr)
	if err := c.postJSON("/api/auth/login", map[string]string{
		"username": username, "password": password,
	}, nil); err != nil {
		return nil, fmt.Errorf("login %s: %w", username, err)
	}
	return c, nil
}

type bandResp struct {
	Band struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"band"`
}

type bandsResp struct {
	Bands []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"bands"`
}

type inviteResp struct {
	Invite struct {
		ID string `json:"id"`
	} `json:"invite"`
}

type invitesResp struct {
	Invites []struct {
		ID     string `json:"id"`
		BandID string `json:"bandId"`
	} `json:"invites"`
}

type songResp struct {
	Song struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"song"`
}

type songsResp struct {
	Songs []struct {
		ID    string `json:"id"`
		Title string `json:"title"`
	} `json:"songs"`
}

type filesResp struct {
	Files []struct {
		ID          string `json:"id"`
		Filename    string `json:"filename"`
		ContentType string `json:"contentType"`
	} `json:"files"`
}

// seedGroup runs the full real flow for one group and returns counts of how its
// PDFs were obtained.
func seedGroup(addr, password string, g groupDef) (seededGroup, int, int, error) {
	var fetched, generated int
	admin, err := login(addr, g.admin, password)
	if err != nil {
		return seededGroup{}, 0, 0, err
	}

	// Find an existing band by name (idempotency) or create it.
	bandID, err := findOrCreateBand(admin, g.name)
	if err != nil {
		return seededGroup{}, 0, 0, err
	}
	fmt.Printf("   band id %s\n", bandID)

	// Invite each member; the member logs in and accepts the matching invite.
	for _, m := range g.members {
		if err := inviteAndAccept(addr, password, admin, bandID, m); err != nil {
			return seededGroup{}, 0, 0, err
		}
	}

	// Songs (admin creates) + a PDF each.
	existing, err := songTitlesOf(admin, bandID)
	if err != nil {
		return seededGroup{}, 0, 0, err
	}
	var titles []string
	songIDByTitle := map[string]string{}
	// Collect what each song needs for the annotation-import step (run after the
	// loop so we have every song id + its PDF fileId).
	type songAnn struct {
		songID    string
		title     string
		generated bool
		pages     int
	}
	var anns []songAnn
	for _, s := range g.songs {
		titles = append(titles, s.title)
		songID, ok := existing[s.title]
		if !ok {
			var sr songResp
			if err := admin.postJSON("/api/bands/"+bandID+"/songs",
				map[string]string{"title": s.title, "artist": s.artist}, &sr); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("create song %q: %w", s.title, err)
			}
			songID = sr.Song.ID
			fmt.Printf("   + song %q\n", s.title)
		} else {
			fmt.Printf("   = song %q exists\n", s.title)
		}
		songIDByTitle[s.title] = songID

		// Set song metadata (key/tempo/tags/notes) — idempotent PATCH.
		if s.key != "" || s.tempo != 0 || len(s.tags) > 0 || s.notes != "" {
			if err := admin.patchJSON("/api/bands/"+bandID+"/songs/"+songID, map[string]any{
				"key": s.key, "tempo": s.tempo, "tags": s.tags, "notes": s.notes,
			}, nil); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("set metadata for %q: %w", s.title, err)
			}
		}

		// Effective shared-pool sources: the multi-file pool if defined, else the
		// single src. The first source drives the song's annotation layout.
		sources := s.files
		if len(sources) == 0 {
			sources = []pdfSource{s.src}
		}
		primary := sources[0]

		// Skip the PDF upload(s) if the song already has files (idempotency).
		var fr filesResp
		if err := admin.getJSON("/api/bands/"+bandID+"/songs/"+songID+"/files", &fr); err != nil {
			return seededGroup{}, 0, 0, err
		}
		if len(fr.Files) > 0 {
			fmt.Printf("     = already has %d file(s)\n", len(fr.Files))
			// Re-runs reuse the existing PDF; learn its origin from the cache meta
			// so annotation coords match (generated layout vs fetched/unknown).
			// "generated" here means a pdf.go STAFF placeholder (known systemTopY layout).
			// A committed real chart (localPath — Open Road, House, Amazing Grace) is NOT a
			// staff placeholder, so it takes the generic/known-coord path, not the staff one.
			anns = append(anns, songAnn{songID: songID, title: s.title, generated: primary.localPath == "" && !cachedFetched(primary), pages: primary.pages})
			continue
		}
		// Upload every pool file in order, assigning a matching displayOrder so the
		// default ordering is deterministic.
		uploaded := make([]string, 0, len(sources))
		for i, src := range sources {
			res, err := resolvePDF(src)
			if err != nil {
				return seededGroup{}, 0, 0, err
			}
			if res.fetched {
				fetched++
			} else {
				generated++
			}
			var fileOut struct {
				File struct {
					ID string `json:"id"`
				} `json:"file"`
			}
			if err := admin.uploadFile("/api/bands/"+bandID+"/songs/"+songID+"/files",
				src.filename(), "application/pdf", res.data, &fileOut); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("upload pdf %q for %q: %w", src.filename(), s.title, err)
			}
			if err := admin.patchJSON("/api/bands/"+bandID+"/songs/"+songID+"/files/"+fileOut.File.ID,
				map[string]any{"displayOrder": i}, nil); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("set displayOrder for %q: %w", src.filename(), err)
			}
			uploaded = append(uploaded, fileOut.File.ID)
			fmt.Printf("     + file %q (%s, %s, %d bytes)\n", src.filename(), src.cacheName, res.origin, len(res.data))
		}

		// B10: a REAL text chart from committed chart-dialect source — POST /text-charts
		// renders it to a PDF server-side, so the demo shows the T19 chart type (not only
		// uploaded PDFs). It enters the pool after the PDFs (highest displayOrder).
		if s.textChartPath != "" {
			source, rerr := os.ReadFile(s.textChartPath)
			if rerr != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("read text chart %q for %q: %w", s.textChartPath, s.title, rerr)
			}
			var chartOut struct {
				File struct {
					ID string `json:"id"`
				} `json:"file"`
			}
			if err := admin.postJSON("/api/bands/"+bandID+"/songs/"+songID+"/text-charts",
				map[string]string{"source": string(source)}, &chartOut); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("create text chart for %q: %w", s.title, err)
			}
			if err := admin.patchJSON("/api/bands/"+bandID+"/songs/"+songID+"/files/"+chartOut.File.ID,
				map[string]any{"displayOrder": len(uploaded)}, nil); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("set displayOrder for text chart of %q: %w", s.title, err)
			}
			uploaded = append(uploaded, chartOut.File.ID)
			fmt.Printf("     + text chart %q (rendered from %s)\n", s.title, s.textChartPath)
		}

		// Curate per-member personal file selections via the my-files endpoint.
		for username, order := range s.myFilesFor {
			ids := make([]string, 0, len(order))
			for _, idx := range order {
				if idx >= 0 && idx < len(uploaded) {
					ids = append(ids, uploaded[idx])
				}
			}
			member := admin
			if username != g.admin {
				member, err = login(addr, username, password)
				if err != nil {
					return seededGroup{}, 0, 0, err
				}
			}
			if err := member.putJSON("/api/bands/"+bandID+"/songs/"+songID+"/my-files",
				map[string]any{"fileIds": ids}, nil); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("set my-files for %s on %q: %w", username, s.title, err)
			}
			titles := make([]string, len(order))
			for i, idx := range order {
				if idx >= 0 && idx < len(sources) {
					titles[i] = sources[idx].filename()
				}
			}
			fmt.Printf("     + %s my-files: [%s]\n", username, strings.Join(titles, ", "))
		}

		// Seed per-member personal cues (T50) via the my-cues endpoint, as each user.
		for username, cues := range s.cuesFor {
			payload := make([]map[string]string, 0, len(cues))
			labels := make([]string, 0, len(cues))
			for _, c := range cues {
				payload = append(payload, map[string]string{"icon": c.icon, "color": c.color})
				labels = append(labels, c.icon)
			}
			member := admin
			if username != g.admin {
				member, err = login(addr, username, password)
				if err != nil {
					return seededGroup{}, 0, 0, err
				}
			}
			if err := member.putJSON("/api/bands/"+bandID+"/songs/"+songID+"/my-cues",
				map[string]any{"cues": payload}, nil); err != nil {
				return seededGroup{}, 0, 0, fmt.Errorf("set my-cues for %s on %q: %w", username, s.title, err)
			}
			fmt.Printf("     + %s cues: [%s]\n", username, strings.Join(labels, ", "))
		}
		// generated = a pdf.go STAFF placeholder (systemTopY layout). A committed real chart
		// (localPath) is NOT a staff placeholder → generic/known-coord placement.
		anns = append(anns, songAnn{songID: songID, title: s.title, generated: primary.localPath == "" && !cachedFetched(primary), pages: primary.pages})
	}

	// Promote a member to the conductor role BEFORE importing annotations, so the
	// conductor-zone "cues" layer can be owned/authored by that conductor (#3). The
	// conductor ROLE — not the band admin — governs write access to the cues.
	if g.conductor != "" {
		if err := promoteConductor(admin, bandID, g.conductor); err != nil {
			return seededGroup{}, 0, 0, err
		}
	}

	// Annotation layers + objects on each song's PDF (idempotent import). Needs the
	// member username→id map (layer ownerId). The conductor-zone cues are owned by the
	// promoted conductor (g.conductor); if none, they fall back to the admin id.
	userID, adminID, err := memberIDs(admin, bandID, g.admin)
	if err != nil {
		return seededGroup{}, 0, 0, err
	}
	conductorID := adminID
	if g.conductor != "" {
		if id, ok := userID[g.conductor]; ok {
			conductorID = id
		}
	}
	for _, a := range anns {
		fileID, err := firstPDFFileID(admin, bandID, a.songID)
		if err != nil {
			return seededGroup{}, 0, 0, err
		}
		if fileID == "" {
			continue
		}
		// "The Open Road" uses a local lead-sheet chart with its own known layout, so
		// it gets chart-specific annotation coords; every other song uses the
		// generated/fetched-layout builder.
		im := buildSongAnnotations(a.songID, fileID, a.title, g.kind, userID, conductorID, a.generated, a.pages)
		switch a.title {
		case "The Open Road":
			im = buildOpenRoadAnnotations(a.songID, fileID, userID, conductorID)
		case "House of the Rising Sun", "Amazing Grace":
			// Committed band charts (guitar tab / lead sheet) — a light showcase placed for
			// their known content band, not the staff-relative or orchestral generic layout.
			im = buildBandChartAnnotations(a.songID, fileID, a.title, userID, conductorID)
		case "Canon in D":
			// Committed LilyPond orchestral part (staff at top) — clean placement below it.
			im = buildCanonAnnotations(a.songID, fileID, userID, conductorID)
		}
		var got annotationsImport
		if err := admin.postJSON("/api/bands/"+bandID+"/songs/"+a.songID+"/annotations/import", im, &got); err != nil {
			return seededGroup{}, 0, 0, fmt.Errorf("import annotations for %q: %w", a.title, err)
		}
		fmt.Printf("     + annotations %q: %d layers, %d objects\n", a.title, len(got.Layers), len(got.Objects))

		// B11: give House of the Rising Sun's DRUMS part its OWN annotations (the first
		// PDF — the guitar tab — keeps the section-form set above), so switching file tabs
		// visibly demonstrates per-file scoping (T40) — distinct ink over distinct PDFs.
		// Additive imports; idempotent by id.
		if a.title == "House of the Rising Sun" {
			parts := []struct {
				filename string
				build    func(songID, fileID string) annotationsImport
			}{
				{"Drums", func(s, f string) annotationsImport { return buildDrumPartAnnotations(s, f) }},
			}
			for _, p := range parts {
				fid, err := pdfFileIDByFilename(admin, bandID, a.songID, p.filename)
				if err != nil {
					return seededGroup{}, 0, 0, err
				}
				if fid == "" {
					continue
				}
				var pg annotationsImport
				if err := admin.postJSON("/api/bands/"+bandID+"/songs/"+a.songID+"/annotations/import", p.build(a.songID, fid), &pg); err != nil {
					return seededGroup{}, 0, 0, fmt.Errorf("import %s annotations for %q: %w", p.filename, a.title, err)
				}
				fmt.Printf("     + annotations %q [%s]: %d layers, %d objects\n", a.title, p.filename, len(pg.Layers), len(pg.Objects))
			}
		}
	}

	// Build a setlist (showcases setlists + per-item overrides).
	if g.setlist.name != "" {
		if err := seedSetlist(admin, bandID, g.setlist, titles, songIDByTitle); err != nil {
			return seededGroup{}, 0, 0, err
		}
	}

	return seededGroup{def: g, songs: titles}, fetched, generated, nil
}

// memberIDs returns a username→userId map for the band plus the admin's user id
// (used as the conductor layer's ownerId).
func memberIDs(admin *apiClient, bandID, adminUsername string) (map[string]string, string, error) {
	var mr membersResp
	if err := admin.getJSON("/api/bands/"+bandID+"/members", &mr); err != nil {
		return nil, "", err
	}
	out := map[string]string{}
	adminID := ""
	for _, m := range mr.Members {
		out[m.User.Username] = m.User.ID
		if m.User.Username == adminUsername {
			adminID = m.User.ID
		}
	}
	return out, adminID, nil
}

// firstPDFFileID returns the id of the song's first application/pdf file (the
// annotation layers attach to it), or "" if none.
func firstPDFFileID(admin *apiClient, bandID, songID string) (string, error) {
	var fr filesResp
	if err := admin.getJSON("/api/bands/"+bandID+"/songs/"+songID+"/files", &fr); err != nil {
		return "", err
	}
	for _, f := range fr.Files {
		if f.ContentType == "application/pdf" {
			return f.ID, nil
		}
	}
	if len(fr.Files) > 0 {
		return fr.Files[0].ID, nil
	}
	return "", nil
}

// pdfFileIDByFilename returns the id of the song's PDF file with the given filename
// (the docTitle the seed uploads it under, e.g. "Part - Vocals"), or "" if absent.
func pdfFileIDByFilename(admin *apiClient, bandID, songID, filename string) (string, error) {
	var fr filesResp
	if err := admin.getJSON("/api/bands/"+bandID+"/songs/"+songID+"/files", &fr); err != nil {
		return "", err
	}
	for _, f := range fr.Files {
		if f.Filename == filename && f.ContentType == "application/pdf" {
			return f.ID, nil
		}
	}
	return "", nil
}

func findOrCreateBand(admin *apiClient, name string) (string, error) {
	var br bandsResp
	if err := admin.getJSON("/api/bands", &br); err != nil {
		return "", err
	}
	for _, b := range br.Bands {
		if strings.EqualFold(b.Name, name) {
			return b.ID, nil
		}
	}
	var created bandResp
	if err := admin.postJSON("/api/bands", map[string]string{"name": name}, &created); err != nil {
		return "", fmt.Errorf("create band %q: %w", name, err)
	}
	return created.Band.ID, nil
}

// inviteAndAccept runs the real consent flow: admin invites by username, the
// invited user logs in, finds the pending invite for this band, and accepts it.
func inviteAndAccept(addr, password string, admin *apiClient, bandID, username string) error {
	// Admin invites (ignore conflicts — a re-run may already have an open/used invite).
	var ir inviteResp
	err := admin.postJSON("/api/bands/"+bandID+"/invites",
		map[string]string{"identifier": username, "kind": "username"}, &ir)
	if err != nil && !isConflict(err) {
		return fmt.Errorf("invite %s: %w", username, err)
	}

	member, err := login(addr, username, password)
	if err != nil {
		return err
	}
	// Already a member? Nothing to accept.
	var br bandsResp
	if err := member.getJSON("/api/bands", &br); err != nil {
		return err
	}
	for _, b := range br.Bands {
		if b.ID == bandID {
			fmt.Printf("   = %s already a member\n", username)
			return nil
		}
	}
	// Find the pending invite for this band and accept it.
	var inv invitesResp
	if err := member.getJSON("/api/invites", &inv); err != nil {
		return err
	}
	for _, i := range inv.Invites {
		if i.BandID == bandID {
			if err := member.postJSON("/api/invites/"+i.ID+"/accept", nil, nil); err != nil {
				return fmt.Errorf("accept invite for %s: %w", username, err)
			}
			fmt.Printf("   + %s invited and accepted\n", username)
			return nil
		}
	}
	return fmt.Errorf("no pending invite found for %s on band %s", username, bandID)
}

func songTitlesOf(admin *apiClient, bandID string) (map[string]string, error) {
	var sr songsResp
	if err := admin.getJSON("/api/bands/"+bandID+"/songs", &sr); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, s := range sr.Songs {
		out[s.Title] = s.ID
	}
	return out, nil
}

type membersResp struct {
	Members []struct {
		User struct {
			ID       string `json:"id"`
			Username string `json:"username"`
		} `json:"user"`
		Role string `json:"role"`
	} `json:"members"`
}

type setlistResp struct {
	Setlist struct {
		ID string `json:"id"`
	} `json:"setlist"`
}

type setlistsResp struct {
	Setlists []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"setlists"`
}

type setlistDetailResp struct {
	Items []struct {
		ID     string `json:"id"`
		SongID string `json:"songId"`
	} `json:"items"`
}

type itemResp struct {
	Item struct {
		ID string `json:"id"`
	} `json:"item"`
}

// promoteConductor sets a member's role to "conductor" (idempotent).
func promoteConductor(admin *apiClient, bandID, username string) error {
	var mr membersResp
	if err := admin.getJSON("/api/bands/"+bandID+"/members", &mr); err != nil {
		return err
	}
	for _, m := range mr.Members {
		if m.User.Username == username {
			if m.Role == "conductor" {
				return nil
			}
			if err := admin.patchJSON("/api/bands/"+bandID+"/members/"+m.User.ID,
				map[string]string{"role": "conductor"}, nil); err != nil {
				return fmt.Errorf("promote %s to conductor: %w", username, err)
			}
			fmt.Printf("   + %s promoted to conductor\n", username)
			return nil
		}
	}
	return nil
}

// seedSetlist creates a named setlist (idempotent), adds the group's songs in
// order if it's empty, then applies per-item key/tempo/notes overrides.
func seedSetlist(admin *apiClient, bandID string, sd setlistDef, orderedTitles []string, songIDByTitle map[string]string) error {
	var sls setlistsResp
	if err := admin.getJSON("/api/bands/"+bandID+"/setlists", &sls); err != nil {
		return err
	}
	setlistID := ""
	for _, s := range sls.Setlists {
		if strings.EqualFold(s.Name, sd.name) {
			setlistID = s.ID
			break
		}
	}
	if setlistID == "" {
		var sr setlistResp
		if err := admin.postJSON("/api/bands/"+bandID+"/setlists", map[string]string{
			"name": sd.name, "eventDate": sd.eventDate, "venue": sd.venue, "notes": sd.notes,
		}, &sr); err != nil {
			return fmt.Errorf("create setlist %q: %w", sd.name, err)
		}
		setlistID = sr.Setlist.ID
		fmt.Printf("   + setlist %q\n", sd.name)
	} else {
		fmt.Printf("   = setlist %q exists\n", sd.name)
	}

	var detail setlistDetailResp
	if err := admin.getJSON("/api/bands/"+bandID+"/setlists/"+setlistID, &detail); err != nil {
		return err
	}
	itemBySong := map[string]string{}
	for _, it := range detail.Items {
		itemBySong[it.SongID] = it.ID
	}
	if len(detail.Items) == 0 {
		for _, title := range orderedTitles {
			songID, ok := songIDByTitle[title]
			if !ok {
				continue
			}
			var ir itemResp
			if err := admin.postJSON("/api/bands/"+bandID+"/setlists/"+setlistID+"/items",
				map[string]string{"songId": songID}, &ir); err != nil {
				return fmt.Errorf("add setlist item %q: %w", title, err)
			}
			itemBySong[songID] = ir.Item.ID
		}
		fmt.Printf("     + %d songs added to setlist\n", len(orderedTitles))
	}

	for _, ov := range sd.overrides {
		songID, ok := songIDByTitle[ov.song]
		if !ok {
			continue
		}
		itemID, ok := itemBySong[songID]
		if !ok {
			continue
		}
		body := map[string]any{}
		if ov.keyOverride != "" {
			body["keyOverride"] = ov.keyOverride
		}
		if ov.tempoOverride != 0 {
			body["tempoOverride"] = ov.tempoOverride
		}
		if ov.notes != "" {
			body["notes"] = ov.notes
		}
		if len(body) == 0 {
			continue
		}
		if err := admin.patchJSON("/api/bands/"+bandID+"/setlists/"+setlistID+"/items/"+itemID, body, nil); err != nil {
			return fmt.Errorf("override setlist item %q: %w", ov.song, err)
		}
	}
	return nil
}

// printGuide writes a human-friendly summary + instructions to stdout.
func printGuide(addr, password string, people []person, groups []seededGroup, fetched, generated int) {
	w := os.Stdout
	line := strings.Repeat("=", 70)
	fmt.Fprintln(w, line)
	fmt.Fprintln(w, "  TROUBASTACK DEMO — READY")
	fmt.Fprintln(w, line)
	fmt.Fprintln(w)

	// Credentials table.
	fmt.Fprintln(w, "  CREDENTIALS  (every password is the same)")
	fmt.Fprintf(w, "  +%s+%s+%s+\n", dash(12), dash(10), dash(34))
	fmt.Fprintf(w, "  | %-10s | %-8s | %-32s |\n", "username", "password", "role")
	fmt.Fprintf(w, "  +%s+%s+%s+\n", dash(12), dash(10), dash(34))
	for _, p := range people {
		fmt.Fprintf(w, "  | %-10s | %-8s | %-32s |\n", p.username, password, p.role)
	}
	fmt.Fprintf(w, "  +%s+%s+%s+\n", dash(12), dash(10), dash(34))
	fmt.Fprintln(w)

	// Structure.
	fmt.Fprintln(w, "  STRUCTURE")
	for _, g := range groups {
		conductor := ""
		if g.def.conductor != "" {
			conductor = "; conductor: " + g.def.conductor
		}
		fmt.Fprintf(w, "  • %s  \"%s\"  (admin: %s; members: %s%s)\n",
			g.def.kind, g.def.name, g.def.admin, strings.Join(g.def.members, ", "), conductor)
		for _, s := range g.def.songs {
			meta := []string{}
			if s.key != "" {
				meta = append(meta, "key "+s.key)
			}
			if s.tempo != 0 {
				meta = append(meta, fmt.Sprintf("%d bpm", s.tempo))
			}
			if len(s.tags) > 0 {
				meta = append(meta, "#"+strings.Join(s.tags, " #"))
			}
			suffix := ""
			if len(meta) > 0 {
				suffix = "  (" + strings.Join(meta, ", ") + ")"
			}
			files := "[PDF + annotations]"
			if len(s.files) > 1 {
				files = fmt.Sprintf("[%d files + annotations]", len(s.files))
			}
			fmt.Fprintf(w, "      - %s%s  %s\n", s.title, suffix, files)
			// Spell out any curated per-member file selections (my-files).
			for username, order := range s.myFilesFor {
				titles := make([]string, 0, len(order))
				for _, idx := range order {
					if idx >= 0 && idx < len(s.files) {
						titles = append(titles, s.files[idx].filename())
					}
				}
				fmt.Fprintf(w, "          • %s's \"my files\": %s\n", username, strings.Join(titles, " → "))
			}
		}
		if sl := g.def.setlist; sl.name != "" {
			when := sl.eventDate
			if sl.venue != "" {
				when = strings.TrimSpace(when + " @ " + sl.venue)
			}
			fmt.Fprintf(w, "      ♪ setlist \"%s\" (%s) — %d songs, with key/tempo overrides\n",
				sl.name, when, len(g.def.songs))
		}
	}
	fmt.Fprintln(w)

	// PDFs note.
	fmt.Fprintf(w, "  SHEET MUSIC: %d PDF(s) fetched (public domain), %d generated locally.\n", fetched, generated)
	fmt.Fprintln(w, "  Cached under core/cmd/seed/assets/ (gitignored) — re-runs reuse them.")
	fmt.Fprintln(w)

	// What to do.
	fmt.Fprintln(w, "  WHAT TO DO")
	fmt.Fprintln(w, "  1. Open the app:")
	fmt.Fprintln(w, "       • dev/hot-reload SPA :  http://localhost:5173   (make dev)")
	fmt.Fprintf(w, "       • single binary      :  %s   (make run / make demo)\n", addr)
	fmt.Fprintln(w, "  2. Log in as  marie / "+password+"  →  The Troubadours")
	fmt.Fprintln(w, "       'House of the Rising Sun' is a MULTI-FILE song: its shared pool has")
	fmt.Fprintln(w, "       a Guitar tab, a Drums groove, and a rendered chord chart. Leo has a")
	fmt.Fprintln(w, "       curated personal 'my files' selection — just the Guitar tab — so his")
	fmt.Fprintln(w, "       viewer opens to that alone, while bandmates see the default all.")
	fmt.Fprintln(w, "       OPEN 'The Open Road' in the Studio viewer to see its LAYERED")
	fmt.Fprintln(w, "       ANNOTATIONS on the lead sheet: conductor cues (red, mandatory), a")
	fmt.Fprintln(w, "       shared Form/Chorus highlight (amber), and Marie's personal 'My")
	fmt.Fprintln(w, "       notes' (capo, breathe, a circled change). All content is original")
	fmt.Fprintln(w, "       or public-domain (copyright-safe).")
	fmt.Fprintln(w, "       Toggle layers on/off and zoom in.")
	fmt.Fprintln(w, "  3. Log in as  maestro / "+password+"  →  City Chamber Orchestra")
	fmt.Fprintln(w, "       open a piece for the same layered view — plus a player's green")
	fmt.Fprintln(w, "       'Bowing/Breath marks' personal layer instead of chords.")
	fmt.Fprintln(w, "  4. Other members (leo, sasha / flora, cory) already accepted their")
	fmt.Fprintln(w, "       invites — log in as any of them to see the shared band + songs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  WHAT YOU CAN SET  (all of this works today — no canvas needed)")
	fmt.Fprintln(w, "   • Song page  : key, tempo, tags, notes; upload / rename / reorder / delete files.")
	fmt.Fprintln(w, "   • Band page  : rename, change member roles (admin/conductor/member), remove / leave,")
	fmt.Fprintln(w, "                  list & revoke pending invites.")
	fmt.Fprintln(w, "   • Setlists   : create, add / remove / reorder songs, override key / tempo / notes per song.")
	fmt.Fprintln(w, "   • Viewer     : open any song to see ~3 layered annotation layers (conductor / shared / part);")
	fmt.Fprintln(w, "                  toggle layer visibility and zoom — view-only for now.")
	fmt.Fprintln(w, "   (The seed has already set song metadata, promoted a conductor, built a setlist, and")
	fmt.Fprintln(w, "    imported conductor cues + section markings + a part layer onto each song's PDF.)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  RESET:  stop the server and  rm -rf troubadata  (wipes users/bands/blobs).")
	fmt.Fprintln(w, line)
}

func dash(n int) string { return strings.Repeat("-", n) }

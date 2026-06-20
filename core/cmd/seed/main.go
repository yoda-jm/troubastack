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

// songDef pairs a song title/artist + settable metadata with its PDF source.
type songDef struct {
	title  string
	artist string
	key    string // musical key (settable on the song page)
	tempo  int    // BPM
	tags   []string
	notes  string
	src    pdfSource
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
			name:    "The Troubadours",
			kind:    "Band",
			admin:   "marie",
			members: []string{"leo", "sasha"},
			songs: []songDef{
				{title: "Wonderwall", artist: "Oasis", key: "F#m", tempo: 87, tags: []string{"britpop", "singalong"}, notes: "Capo 2; everyone in on the last chorus.",
					src: pdfSource{cacheName: "wonderwall.pdf", title: "Wonderwall", subtitle: "Oasis", pages: 3}},
				{title: "Hallelujah", artist: "Leonard Cohen", key: "C", tempo: 60, tags: []string{"ballad"}, notes: "Slow 6/8; watch the dynamics.",
					src: pdfSource{cacheName: "hallelujah.pdf", title: "Hallelujah", subtitle: "Leonard Cohen", pages: 4}},
				{title: "Black Hole Sun", artist: "Soundgarden", key: "G", tempo: 105, tags: []string{"grunge", "encore"},
					src: pdfSource{cacheName: "black-hole-sun.pdf", title: "Black Hole Sun", subtitle: "Soundgarden", pages: 3}},
			},
			setlist: setlistDef{
				name: "Sat @ The Anchor", eventDate: "2026-07-04", venue: "The Anchor Pub", notes: "60-minute set.",
				overrides: []overrideDef{
					{song: "Wonderwall", keyOverride: "Em", notes: "Acoustic intro, capo 2."},
					{song: "Black Hole Sun", tempoOverride: 98},
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
				{title: "Air on the G String", artist: "J. S. Bach", key: "D", tempo: 60, tags: []string{"classical", "public-domain"}, notes: "BWV 1068; molto espressivo.",
					src: pdfSource{
						cacheName: "air-on-the-g-string.pdf",
						title:     "Air on the G String",
						subtitle:  "J. S. Bach (BWV 1068)",
						pages:     2,
						// Verified direct public-domain PDF from the Mutopia Project.
						urls: []string{
							"https://www.mutopiaproject.org/ftp/BachJS/BWV1068/bach_air_bmv_1068/bach_air_bmv_1068-a4.pdf",
							"https://www.mutopiaproject.org/ftp/BachJS/BWV1068/bach_air_bmv_1068/bach_air_bmv_1068-let.pdf",
						},
					}},
			},
			setlist: setlistDef{
				name: "Spring Concert", eventDate: "2026-05-15", venue: "City Hall", notes: "Strings programme.",
				overrides: []overrideDef{
					{song: "Eine kleine Nachtmusik", tempoOverride: 132, notes: "Take the repeat."},
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
		ID       string `json:"id"`
		Filename string `json:"filename"`
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

		// Skip the PDF upload if the song already has files (idempotency).
		var fr filesResp
		if err := admin.getJSON("/api/bands/"+bandID+"/songs/"+songID+"/files", &fr); err != nil {
			return seededGroup{}, 0, 0, err
		}
		if len(fr.Files) > 0 {
			fmt.Printf("     = already has %d file(s)\n", len(fr.Files))
			continue
		}
		res, err := resolvePDF(s.src)
		if err != nil {
			return seededGroup{}, 0, 0, err
		}
		if res.fetched {
			fetched++
		} else {
			generated++
		}
		if err := admin.uploadFile("/api/bands/"+bandID+"/songs/"+songID+"/files",
			s.src.cacheName, "application/pdf", res.data, nil); err != nil {
			return seededGroup{}, 0, 0, fmt.Errorf("upload pdf for %q: %w", s.title, err)
		}
		fmt.Printf("     + PDF %s (%s, %d bytes)\n", s.src.cacheName, res.origin, len(res.data))
	}

	// Promote a member to the conductor role (showcases role management).
	if g.conductor != "" {
		if err := promoteConductor(admin, bandID, g.conductor); err != nil {
			return seededGroup{}, 0, 0, err
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
			fmt.Fprintf(w, "      - %s%s  [PDF]\n", s.title, suffix)
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
	fmt.Fprintln(w, "       open 'Wonderwall' (editor placeholder for now); the PDF is")
	fmt.Fprintln(w, "       attached and downloadable via GET /api/files/{id}.")
	fmt.Fprintln(w, "  3. Log in as  maestro / "+password+"  →  City Chamber Orchestra")
	fmt.Fprintln(w, "       for the public-domain classical pieces.")
	fmt.Fprintln(w, "  4. Other members (leo, sasha / flora, cory) already accepted their")
	fmt.Fprintln(w, "       invites — log in as any of them to see the shared band + songs.")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  WHAT YOU CAN SET  (all of this works today — no canvas needed)")
	fmt.Fprintln(w, "   • Song page  : key, tempo, tags, notes; upload / rename / reorder / delete files.")
	fmt.Fprintln(w, "   • Band page  : rename, change member roles (admin/conductor/member), remove / leave,")
	fmt.Fprintln(w, "                  list & revoke pending invites.")
	fmt.Fprintln(w, "   • Setlists   : create, add / remove / reorder songs, override key / tempo / notes per song.")
	fmt.Fprintln(w, "   (The seed has already set song metadata, promoted a conductor, and built a setlist per group.)")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "  RESET:  stop the server and  rm -rf troubadata  (wipes users/bands/blobs).")
	fmt.Fprintln(w, line)
}

func dash(n int) string { return strings.Repeat("-", n) }

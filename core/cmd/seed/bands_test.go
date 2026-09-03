package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBand drops a band folder (manifest + optional repertoire) under dir.
func writeBand(t *testing.T, dir, slug, manifest, repertoire string) {
	t.Helper()
	bd := filepath.Join(dir, slug)
	if err := os.MkdirAll(bd, 0o755); err != nil {
		t.Fatal(err)
	}
	if manifest != "" {
		if err := os.WriteFile(filepath.Join(bd, "band.json"), []byte(manifest), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if repertoire != "" {
		if err := os.WriteFile(filepath.Join(bd, "repertoire.json"), []byte(repertoire), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

const validManifest = `{
  "name": "My Band", "shortname": "myband", "notes": "n",
  "admin": {"username": "alice", "display": "Alice", "role": "bass"},
  "members": [{"username": "bob", "display": "Bob", "role": "drums"}]
}`
const validRepertoire = `{"songs":[{"slug":"s1","title":"Song One","artist":"A"},{"slug":"s2","title":"Song Two","artist":"B"}]}`

func TestLoadLocalBands_MissingDir(t *testing.T) {
	t.Setenv("TROUBA_BANDS_DIR", filepath.Join(t.TempDir(), "does-not-exist"))
	g, p, err := loadLocalBands()
	if err != nil || g != nil || p != nil {
		t.Fatalf("missing dir → (%v, %v, %v), want (nil, nil, nil)", g, p, err)
	}
}

func TestLoadLocalBands_Fixture(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TROUBA_BANDS_DIR", dir)
	writeBand(t, dir, "myband", validManifest, validRepertoire)
	// a folder without band.json must be silently skipped
	if err := os.MkdirAll(filepath.Join(dir, "not-a-band"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a manifest with no shortname → shortname falls back to the folder name
	writeBand(t, dir, "othr", `{"name":"Other","admin":{"username":"cara"}}`, "")

	groups, people, err := loadLocalBands()
	if err != nil {
		t.Fatalf("loadLocalBands: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("got %d groups, want 2 (the no-band.json folder must be skipped)", len(groups))
	}
	byShort := map[string]groupDef{}
	for _, g := range groups {
		byShort[g.shortname] = g
	}
	mb, ok := byShort["myband"]
	if !ok {
		t.Fatal("myband not loaded")
	}
	if mb.name != "My Band" || mb.admin != "alice" || !mb.personal || mb.kind != "Band" {
		t.Errorf("myband = %+v, want name/admin/personal/kind set", mb)
	}
	if len(mb.members) != 1 || mb.members[0] != "bob" {
		t.Errorf("myband members = %v, want [bob]", mb.members)
	}
	if len(mb.songs) != 2 {
		t.Errorf("myband songs = %d, want 2", len(mb.songs))
	}
	if _, ok := byShort["othr"]; !ok {
		t.Error("shortname did not fall back to the folder name 'othr'")
	}
	// people = admins + members of the loaded bands, deduped
	names := map[string]bool{}
	for _, p := range people {
		names[p.username] = true
	}
	for _, want := range []string{"alice", "bob", "cara"} {
		if !names[want] {
			t.Errorf("person %q missing", want)
		}
	}
}

func TestLoadLocalBands_BadManifests(t *testing.T) {
	for _, c := range []struct {
		name, manifest string
	}{
		{"malformed json", `{"name": "X"`},
		{"missing name", `{"admin":{"username":"a"}}`},
		{"missing admin", `{"name":"X"}`},
	} {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("TROUBA_BANDS_DIR", dir)
			writeBand(t, dir, "b", c.manifest, "")
			if _, _, err := loadLocalBands(); err == nil {
				t.Errorf("%s: want a path-qualified error, got nil", c.name)
			}
		})
	}
}

// TestSelectGroups is the demo-isolation property: a plain seed builds only the demo (non-personal)
// groups and none of a personal band's members; -band selects exactly that band; -only matches by
// name. Pure — no server.
func TestSelectGroups(t *testing.T) {
	groups := []groupDef{
		{name: "The Troubadours", admin: "marie", members: []string{"leo", "sasha"}},
		{name: "City Chamber Orchestra", admin: "maestro", members: []string{"flora"}},
		{name: "My Band", shortname: "myband", admin: "alice", members: []string{"bob"}, personal: true},
	}
	people := []person{{username: "marie"}, {username: "leo"}, {username: "sasha"},
		{username: "maestro"}, {username: "flora"}, {username: "alice"}, {username: "bob"}}

	set := func(ps []person) map[string]bool {
		m := map[string]bool{}
		for _, p := range ps {
			m[p.username] = true
		}
		return m
	}

	t.Run("neither → demo groups only, no personal members", func(t *testing.T) {
		g, p, err := selectGroups(groups, people, "", nil)
		if err != nil {
			t.Fatal(err)
		}
		if len(g) != 2 {
			t.Fatalf("got %d groups, want 2 (personal skipped)", len(g))
		}
		for _, x := range g {
			if x.personal {
				t.Errorf("personal band %q leaked into a plain seed", x.name)
			}
		}
		names := set(p)
		if names["alice"] || names["bob"] {
			t.Errorf("personal band's members leaked into a plain seed: %v", names)
		}
	})

	t.Run("-band myband → exactly that band + its people", func(t *testing.T) {
		g, p, err := selectGroups(groups, people, "", []string{"myband"})
		if err != nil {
			t.Fatal(err)
		}
		if len(g) != 1 || g[0].name != "My Band" {
			t.Fatalf("got %+v, want just My Band", g)
		}
		names := set(p)
		if !names["alice"] || !names["bob"] || names["marie"] {
			t.Errorf("people = %v, want exactly {alice,bob}", names)
		}
	})

	t.Run("-only orchestra → matched by name", func(t *testing.T) {
		g, _, err := selectGroups(groups, people, "orchestra", nil)
		if err != nil || len(g) != 1 || g[0].name != "City Chamber Orchestra" {
			t.Fatalf("got %+v (err %v), want City Chamber Orchestra", g, err)
		}
	})

	t.Run("-band nope → actionable error", func(t *testing.T) {
		if _, _, err := selectGroups(groups, people, "", []string{"nope"}); err == nil {
			t.Error("want an error naming band.json shortname, got nil")
		}
	})

	t.Run("-band alpha -band bns → both bands, others excluded", func(t *testing.T) {
		gs := []groupDef{
			{name: "Blue Note Singers", shortname: "bns", admin: "nikos", members: []string{"ana"}, personal: true},
			{name: "Alpha Band", shortname: "alpha", admin: "ana", members: []string{"bo"}, personal: true},
			{name: "Other Band", shortname: "othr", admin: "zoe", personal: true},
		}
		ps := []person{{username: "nikos"}, {username: "ana"}, {username: "bo"}, {username: "zoe"}}
		g, p, err := selectGroups(gs, ps, "", []string{"alpha", "bns"})
		if err != nil {
			t.Fatal(err)
		}
		if len(g) != 2 {
			t.Fatalf("got %d groups, want 2 (alpha+bns)", len(g))
		}
		names := set(p)
		// teeth: othr's admin zoe must NOT be pulled in, or the filter isn't filtering.
		if !names["nikos"] || !names["ana"] || !names["bo"] || names["zoe"] {
			t.Errorf("people = %v, want {nikos,ana,bo} and NOT zoe", names)
		}
	})

	t.Run("-band bns -band typo → fails loud, naming the miss", func(t *testing.T) {
		gs := []groupDef{{name: "Blue Note Singers", shortname: "bns", admin: "nikos", personal: true}}
		_, _, err := selectGroups(gs, []person{{username: "nikos"}}, "", []string{"bns", "typo"})
		// teeth: a partial match must ERROR (not silently seed just bns), and name "typo".
		if err == nil || !strings.Contains(err.Error(), "typo") {
			t.Errorf("want an error naming the unmatched \"typo\", got %v", err)
		}
	})

	t.Run("-band bns,alpha (one flag, comma) → NOT split, fails loud", func(t *testing.T) {
		gs := []groupDef{
			{name: "Blue Note Singers", shortname: "bns", admin: "nikos", personal: true},
			{name: "Alpha Band", shortname: "alpha", admin: "ana", personal: true},
		}
		ps := []person{{username: "nikos"}, {username: "ana"}}
		// A single -band value keeps its comma: it's the literal shortname "bns,alpha", which
		// matches nothing. This is the shape VLL rejected — it must error, not seed both.
		_, _, err := selectGroups(gs, ps, "", []string{"bns,alpha"})
		if err == nil || !strings.Contains(err.Error(), "bns,alpha") {
			t.Errorf("want an error naming the literal \"bns,alpha\", got %v", err)
		}
	})
}

// TestLoadRepertoire_MultipleChartParts (B15): a song folder with lyrics.txt + guitar-bass.txt
// yields two chart parts, lyrics.txt FIRST, named after the files; only lyrics.txt → one; none → none.
func TestLoadRepertoire_MultipleChartParts(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repertoire.json"), []byte(
		`{"songs":[{"slug":"hc","title":"HC","artist":"E"},{"slug":"only","title":"Only","artist":"X"},{"slug":"none","title":"None","artist":"X"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	writePart := func(slug, file, body string) {
		bd := filepath.Join(dir, slug)
		os.MkdirAll(bd, 0o755)
		if err := os.WriteFile(filepath.Join(bd, file), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writePart("hc", "guitar-bass.txt", "# HC\n\n## V\nAm\n") // sorts before lyrics.txt
	writePart("hc", "lyrics.txt", "# HC\n\n## V\nx\n")
	writePart("only", "lyrics.txt", "# Only\n")
	// "none" has no folder

	songs, err := loadRepertoire(dir)
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]songDef{}
	for _, s := range songs {
		by[s.title] = s
	}
	hc := by["HC"]
	if len(hc.textCharts) != 2 {
		t.Fatalf("HC parts = %d, want 2", len(hc.textCharts))
	}
	if filepath.Base(hc.textCharts[0].path) != "lyrics.txt" || hc.textCharts[0].name != "lyrics" {
		t.Errorf("first part = %+v, want lyrics.txt named 'lyrics'", hc.textCharts[0])
	}
	if filepath.Base(hc.textCharts[1].path) != "guitar-bass.txt" || hc.textCharts[1].name != "guitar-bass" {
		t.Errorf("second part = %+v, want guitar-bass.txt named 'guitar-bass'", hc.textCharts[1])
	}
	if len(by["Only"].textCharts) != 1 || by["Only"].textCharts[0].name != "lyrics" {
		t.Errorf("Only parts = %+v, want one 'lyrics'", by["Only"].textCharts)
	}
	if len(by["None"].textCharts) != 0 {
		t.Errorf("None parts = %d, want 0", len(by["None"].textCharts))
	}
}

// writeFile drops an extra file into a band folder (T100: setlists.json).
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestScorePriority: the full score outranks a single-instrument part or a lyrics/translation
// sheet, in both English and French, so the song's default file is the whole arrangement.
func TestScorePriority(t *testing.T) {
	cases := []struct {
		name string
		want int
	}{
		{"Faith arr Mark Bryner.pdf", 0},
		{"Bewitched.pdf", 0},
		{"Africa(Toto).pdf", 0},
		{"Africa.docx.pdf", 1},                  // a doc exported to PDF, not the score
		{"Hymne à l amour avec paroles.pdf", 1}, // lyrics sheet
		{"Dear Heart (pour flûte).pdf", 2},
		{"BASS_690383-faith.pdf", 2},
		{"Mr Sandman percussion.pdf", 2},
		{"Jingle Bells-basse-resolution.pdf", 2}, // "basse" wins over "resolution"
		{"All That Jazz (Piano)-Partition musicien.pdf", 2},
	}
	for _, c := range cases {
		if got := scorePriority(c.name); got != c.want {
			t.Errorf("scorePriority(%q) = %d, want %d", c.name, got, c.want)
		}
	}
}

// TestLoadRepertoire_FullScoreFirst: a song folder with a score + instrument parts + a lyrics
// sheet attaches the FULL SCORE as the first file. Teeth: the bass part must not sort first.
func TestLoadRepertoire_FullScoreFirst(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "repertoire.json"),
		[]byte(`{"songs":[{"slug":"faith","title":"Faith","artist":"Stevie Wonder"}]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	bd := filepath.Join(dir, "faith")
	os.MkdirAll(bd, 0o755)
	for _, f := range []string{"BASS_690383-faith.pdf", "DRUMS_690383-faith.pdf", "Faith arr Mark Bryner.pdf", "Faith avec paroles.pdf"} {
		if err := os.WriteFile(filepath.Join(bd, f), []byte("%PDF-1.4\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	songs, err := loadRepertoire(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 1 || len(songs[0].files) != 4 {
		t.Fatalf("got %d songs / %d files, want 1 / 4", len(songs), len(songs[0].files))
	}
	if songs[0].files[0].docTitle != "Faith arr Mark Bryner" {
		t.Errorf("first file = %q, want the full score \"Faith arr Mark Bryner\"", songs[0].files[0].docTitle)
	}
	// the two instrument parts must land last, after score and lyrics
	last := songs[0].files[3].docTitle
	if last != "BASS_690383-faith" && last != "DRUMS_690383-faith" {
		t.Errorf("last file = %q, want an instrument part", last)
	}
}

// TestLoadLocalBands_Conductor: a band.json member marked "conductor": true becomes the group's
// conductor (the seed promotes them). Teeth: a plain member leaves conductor empty.
func TestLoadLocalBands_Conductor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TROUBA_BANDS_DIR", dir)
	writeBand(t, dir, "bns", `{
	  "name": "Blue Note Singers", "shortname": "bns",
	  "admin": {"username": "ana", "display": "Ana"},
	  "members": [
	    {"username": "nikos", "display": "Nikos", "role": "direction", "conductor": true},
	    {"username": "pat", "display": "Pat", "role": "alto"}
	  ]
	}`, "")
	writeBand(t, dir, "plain", `{
	  "name": "Plain", "shortname": "plain",
	  "admin": {"username": "al"}, "members": [{"username": "bo"}]
	}`, "")

	groups, _, err := loadLocalBands()
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]groupDef{}
	for _, g := range groups {
		by[g.shortname] = g
	}
	if by["bns"].conductor != "nikos" {
		t.Errorf("bns conductor = %q, want \"nikos\"", by["bns"].conductor)
	}
	if by["plain"].conductor != "" {
		t.Errorf("plain conductor = %q, want empty (no member flagged)", by["plain"].conductor)
	}
}

// TestBandsDirCandidates (T129): TROUBA_BANDS_DIR wins outright; otherwise the runtime root comes
// FIRST so a fresh checkout defaults OUT of the source tree, with the historical cwd-relative paths
// after it for compatibility. Discriminating: the runtime root is candidate[0] — a naive "keep
// cwd-first" ordering would put "../bands" first, so reverting to it fails this.
func TestBandsDirCandidates(t *testing.T) {
	if got := bandsDirCandidates("/srv/mybands", "/home/x/.local/share/troubastack"); len(got) != 1 || got[0] != "/srv/mybands" {
		t.Errorf("TROUBA_BANDS_DIR set: got %v, want exactly [/srv/mybands]", got)
	}
	got := bandsDirCandidates("", "/rt")
	if len(got) == 0 || got[0] != "/rt/bands" {
		t.Fatalf("unset: got %v, want the runtime root /rt/bands FIRST", got)
	}
	// the historical cwd-relative paths must survive, after the runtime root
	rest := got[1:]
	want := []string{"../bands", "bands", "../../bands", "../../../bands"}
	if len(rest) != len(want) {
		t.Fatalf("cwd fallbacks = %v, want %v", rest, want)
	}
	for i := range want {
		if rest[i] != want[i] {
			t.Errorf("fallback[%d] = %q, want %q", i, rest[i], want[i])
		}
	}
}

func TestTroubaHome(t *testing.T) {
	t.Setenv("TROUBA_HOME", "/explicit/root")
	if got := troubaHome(); got != "/explicit/root" {
		t.Errorf("TROUBA_HOME set: got %q, want it verbatim", got)
	}
	t.Setenv("TROUBA_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/xdg")
	if got := troubaHome(); got != "/xdg/troubastack" {
		t.Errorf("XDG_DATA_HOME: got %q, want /xdg/troubastack", got)
	}
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "/home/vll")
	if got := troubaHome(); got != "/home/vll/.local/share/troubastack" {
		t.Errorf("HOME default: got %q, want /home/vll/.local/share/troubastack", got)
	}
}

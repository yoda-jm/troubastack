package main

import (
	"os"
	"path/filepath"
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
		g, p, err := selectGroups(groups, people, "", "")
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
		g, p, err := selectGroups(groups, people, "", "myband")
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
		g, _, err := selectGroups(groups, people, "orchestra", "")
		if err != nil || len(g) != 1 || g[0].name != "City Chamber Orchestra" {
			t.Fatalf("got %+v (err %v), want City Chamber Orchestra", g, err)
		}
	})

	t.Run("-band nope → actionable error", func(t *testing.T) {
		if _, _, err := selectGroups(groups, people, "", "nope"); err == nil {
			t.Error("want an error naming band.json shortname, got nil")
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

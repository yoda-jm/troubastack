package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestExampleFileInSync (CFG01) pins the committed core/troubacore.example.ini to
// the generator output — the docs can't rot: change a knob/comment and this fails
// until you regenerate `troubacore --print-default-config > troubacore.example.ini`.
func TestExampleFileInSync(t *testing.T) {
	const path = "../../troubacore.example.ini" // core/troubacore.example.ini
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read committed example: %v", err)
	}
	if got := PrintDefault(); got != string(committed) {
		t.Fatalf("troubacore.example.ini is stale — regenerate with\n"+
			"  go run ./cmd/troubacore --print-default-config > troubacore.example.ini\n"+
			"--- want (generator) ---\n%s\n--- got (committed) ---\n%s", got, committed)
	}
}

// TestDefault checks the built-in defaults match today's behavior exactly (the
// "no file, no env" baseline).
func TestDefault(t *testing.T) {
	c := Default()
	want := map[string]string{
		"TROUBACORE_ADDR":        ":8080",
		"TROUBA_SECURE_COOKIES":  "false",
		"TROUBA_APP_STORE":       "mem",
		"TROUBA_STORE":           "file",
		"TROUBA_DATA_DIR":        "./troubadata",
		"TROUBA_DATABASE_URL":    "",
		"TROUBA_NO_MDNS":         "true", // mdns.enabled default
		"TROUBA_MDNS_NAME":       "",
		"TROUBA_PDFTOPPM":        "pdftoppm",
		"TROUBA_NODE":            "node",
		"TROUBA_BAKE_CLI":        "../web/bake/dist/cli.js",
		"TROUBA_DIE_WITH_PARENT": "false",
	}
	for _, k := range knobs {
		if got := k.get(&c); got != want[k.env] {
			t.Errorf("default %s (%s.%s) = %q, want %q", k.env, k.section, k.key, got, want[k.env])
		}
	}
}

// TestLoad_FileThenEnv exercises the precedence chain in both directions on the
// two knobs the acceptance criterion names (addr + data_dir): the file overrides
// the default, and an env var overrides the file.
func TestLoad_FileThenEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "troubacore.ini")
	if err := os.WriteFile(path, []byte("[server]\naddr = :9999\n[storage]\ndata_dir = /srv/data\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// File only: file values apply, everything else stays default.
	c, err := Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":9999" || c.Storage.DataDir != "/srv/data" {
		t.Fatalf("file layer: addr=%q data_dir=%q, want :9999 / /srv/data", c.Server.Addr, c.Storage.DataDir)
	}
	if c.Storage.AppStore != "mem" {
		t.Fatalf("untouched knob drifted: app_store=%q, want mem", c.Storage.AppStore)
	}

	// Env overrides the file (both directions covered: addr from env, data_dir stays file).
	t.Setenv("TROUBACORE_ADDR", ":7777")
	c, err = Load(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if c.Server.Addr != ":7777" {
		t.Fatalf("env should override file: addr=%q, want :7777", c.Server.Addr)
	}
	if c.Storage.DataDir != "/srv/data" {
		t.Fatalf("file value should survive when env unset: data_dir=%q, want /srv/data", c.Storage.DataDir)
	}
}

// TestLoad_MissingFile: an explicitly-named missing file is a startup error; a
// missing DEFAULT file is silently fine (defaults + env).
func TestLoad_MissingFile(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.ini"), true); err == nil {
		t.Fatal("explicit missing --config file should error")
	}
	// path "" with no ./troubacore.ini present in the temp cwd → no error.
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	os.Chdir(t.TempDir())
	if _, err := Load("", false); err != nil {
		t.Fatalf("missing default file should be fine, got %v", err)
	}
}

// TestLoad_MDNSInversion: the file uses the positive form (enabled), while the env
// var TROUBA_NO_MDNS keeps its negative "== 1 disables" sense unchanged.
func TestLoad_MDNSInversion(t *testing.T) {
	if c, _ := Load("", false); !c.MDNS.Enabled {
		t.Fatal("mdns default should be enabled")
	}
	t.Setenv("TROUBA_NO_MDNS", "1")
	if c, _ := Load("", false); c.MDNS.Enabled {
		t.Fatal("TROUBA_NO_MDNS=1 should disable mdns")
	}
}

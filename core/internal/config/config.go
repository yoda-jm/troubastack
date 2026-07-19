// Package config resolves troubacore's runtime configuration from three layers
// (ADR 0004): built-in defaults < INI file < TROUBA_* environment variables. CLI
// flags (currently only --config / --print-default-config) sit above these and are
// handled in the composition root. Loading is called from main.go; every subsystem
// keeps receiving already-resolved values, per ADR 0002 — nothing else reads env.
//
// The knob table below is the single source of truth: it drives both loading and
// the generated example file (`--print-default-config`), so the documented example
// can never drift from the code (a byte-equality test enforces it).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"gopkg.in/ini.v1"
)

// Config is the fully resolved server configuration. Fields are plain values —
// the composition root reads them directly.
type Config struct {
	Server struct {
		Addr          string // TROUBACORE_ADDR
		SecureCookies bool   // TROUBA_SECURE_COOKIES
		AppsDir       string // TROUBA_APPS_DIR
	}
	Storage struct {
		AppStore    string // TROUBA_APP_STORE
		Store       string // TROUBA_STORE
		DataDir     string // TROUBA_DATA_DIR
		DatabaseURL string // TROUBA_DATABASE_URL
	}
	MDNS struct {
		Enabled bool   // positive form; inverts TROUBA_NO_MDNS
		Name    string // TROUBA_MDNS_NAME
	}
	Bake struct {
		Pdftoppm string // TROUBA_PDFTOPPM
		Node     string // TROUBA_NODE
		CLI      string // TROUBA_BAKE_CLI
		KeepRevs int    // TROUBA_BAKE_KEEP_REVS
	}
	Dev struct {
		DieWithParent bool // TROUBA_DIE_WITH_PARENT
	}
}

// kind classifies a knob's value type for parsing + rendering.
type kind int

const (
	kindString kind = iota
	kindBool
	// kindBoolInv is a bool whose ENV var uses the NEGATIVE sense (set=1 disables)
	// while the file/struct use the positive sense. Only TROUBA_NO_MDNS.
	kindBoolInv
)

// knob describes one configuration value: where it lives in the file, which env
// var overrides it, its default (as it appears in the example file), a one-line
// comment, and getter/setters bridging the typed Config.
type knob struct {
	section string
	key     string
	env     string
	kind    kind
	def     string // default rendered in the example file (string form)
	comment string
	get     func(*Config) string  // current value as a string (for tests/debug)
	set     func(*Config, string) // parse+assign a raw string value
}

// knobs is the ordered, authoritative list. Order here IS the example-file order.
var knobs = []knob{
	{"server", "addr", "TROUBACORE_ADDR", kindString, ":8080", "listen address",
		func(c *Config) string { return c.Server.Addr },
		func(c *Config, v string) { c.Server.Addr = v }},
	{"server", "secure_cookies", "TROUBA_SECURE_COOKIES", kindBool, "false", "Secure flag on the session cookie — enable only behind TLS",
		func(c *Config) string { return boolStr(c.Server.SecureCookies) },
		func(c *Config, v string) { c.Server.SecureCookies = parseBool(v) }},
	{"server", "apps_dir", "TROUBA_APPS_DIR", kindString, "", "directory of downloadable native app binaries (OPS02); empty = none. The Docker image sets this to the embedded apps/ dir",
		func(c *Config) string { return c.Server.AppsDir },
		func(c *Config, v string) { c.Server.AppsDir = v }},

	{"storage", "app_store", "TROUBA_APP_STORE", kindString, "mem", "relational backend: mem | file",
		func(c *Config) string { return c.Storage.AppStore },
		func(c *Config, v string) { c.Storage.AppStore = v }},
	{"storage", "store", "TROUBA_STORE", kindString, "file", "annotation store backend: mem | file | git | pg",
		func(c *Config) string { return c.Storage.Store },
		func(c *Config, v string) { c.Storage.Store = v }},
	{"storage", "data_dir", "TROUBA_DATA_DIR", kindString, "./troubadata", "data root (blobs, bakes, file backends)",
		func(c *Config) string { return c.Storage.DataDir },
		func(c *Config, v string) { c.Storage.DataDir = v }},
	{"storage", "database_url", "TROUBA_DATABASE_URL", kindString, "", "Postgres URL (only when store = pg) — may carry credentials, see header",
		func(c *Config) string { return c.Storage.DatabaseURL },
		func(c *Config, v string) { c.Storage.DatabaseURL = v }},

	{"mdns", "enabled", "TROUBA_NO_MDNS", kindBoolInv, "true", "advertise this core on the LAN (_troubacore._tcp); env TROUBA_NO_MDNS=1 disables",
		func(c *Config) string { return boolStr(c.MDNS.Enabled) },
		func(c *Config, v string) { c.MDNS.Enabled = parseBool(v) }},
	{"mdns", "name", "TROUBA_MDNS_NAME", kindString, "", "mDNS instance name (default: the host name)",
		func(c *Config) string { return c.MDNS.Name },
		func(c *Config, v string) { c.MDNS.Name = v }},

	{"bake", "pdftoppm", "TROUBA_PDFTOPPM", kindString, "pdftoppm", "poppler rasteriser binary (found on PATH by default)",
		func(c *Config) string { return c.Bake.Pdftoppm },
		func(c *Config, v string) { c.Bake.Pdftoppm = v }},
	{"bake", "node", "TROUBA_NODE", kindString, "node", "Node binary for the web/bake overlay worker",
		func(c *Config) string { return c.Bake.Node },
		func(c *Config, v string) { c.Bake.Node = v }},
	{"bake", "cli", "TROUBA_BAKE_CLI", kindString, "../web/bake/dist/cli.js", "web/bake CLI path (repo-relative default works when core runs from core/)",
		func(c *Config) string { return c.Bake.CLI },
		func(c *Config, v string) { c.Bake.CLI = v }},
	{"bake", "keep_revs", "TROUBA_BAKE_KEEP_REVS", kindString, "0", "retention: `troubacore gc` keeps only the newest N baked concert revisions per setlist (0 = keep all; a final-locked rev is never pruned)",
		func(c *Config) string { return strconv.Itoa(c.Bake.KeepRevs) },
		func(c *Config, v string) { n, _ := strconv.Atoi(v); c.Bake.KeepRevs = n }},

	{"dev", "die_with_parent", "TROUBA_DIE_WITH_PARENT", kindBool, "false", "dev: exit when the launching parent process dies (set by make dev/run/demo)",
		func(c *Config) string { return boolStr(c.Dev.DieWithParent) },
		func(c *Config, v string) { c.Dev.DieWithParent = parseBool(v) }},
}

// Default returns the built-in defaults — the bottom precedence layer, and the
// exact behavior when no file and no env vars are present (byte-for-byte today's).
func Default() Config {
	var c Config
	for _, k := range knobs {
		k.set(&c, k.def)
	}
	return c
}

// Load resolves defaults < file < env. If path is "" the default location
// (./troubacore.ini) is tried and a missing file is silently fine. If explicit is
// true (operator passed --config / TROUBA_CONFIG), a missing/unreadable file is a
// fatal startup error — fail loud on operator intent.
func Load(path string, explicit bool) (Config, error) {
	c := Default()

	if path == "" {
		path = "./troubacore.ini"
	}
	if _, err := os.Stat(path); err != nil {
		if explicit {
			return c, fmt.Errorf("config: cannot read %q: %w", path, err)
		}
		// default location, not present → defaults + env only.
	} else {
		f, err := ini.Load(path)
		if err != nil {
			return c, fmt.Errorf("config: parse %q: %w", path, err)
		}
		for _, k := range knobs {
			if sec, err := f.GetSection(k.section); err == nil && sec.HasKey(k.key) {
				k.set(&c, sec.Key(k.key).String())
			}
		}
	}

	// Env vars override the file. A set-but-empty string env var is treated as
	// unset (matches today's os.Getenv("")==default behavior).
	for _, k := range knobs {
		v, ok := os.LookupEnv(k.env)
		if !ok || v == "" {
			continue
		}
		if k.kind == kindBoolInv {
			// negative-sense env: TROUBA_NO_MDNS=1 → disabled. Preserve today's exact
			// "== 1" semantics (any other value leaves the positive default/file value).
			k.set(&c, boolStr(v != "1"))
			continue
		}
		k.set(&c, v)
	}
	return c, nil
}

// PrintDefault renders the fully-commented example INI (every knob at its default,
// commented out) — the body of `troubacore --print-default-config`. Deterministic:
// knob order is fixed and a byte-equality test pins the committed example to it.
func PrintDefault() string {
	var b strings.Builder
	b.WriteString(exampleHeader)

	lastSection := ""
	for _, k := range knobs {
		if k.section != lastSection {
			if lastSection != "" {
				b.WriteString("\n")
			}
			fmt.Fprintf(&b, "[%s]\n", k.section)
			lastSection = k.section
		}
		fmt.Fprintf(&b, "; %s\n", k.comment)
		if k.def == "" {
			// no trailing space after "=" for empty defaults (survives whitespace-
			// stripping editors/hooks, keeping the byte-equality test robust).
			fmt.Fprintf(&b, ";%s =\n", k.key)
		} else {
			fmt.Fprintf(&b, ";%s = %s\n", k.key, k.def)
		}
	}

	b.WriteString(smtpSection)
	return b.String()
}

const exampleHeader = `; troubacore.ini — example configuration (GENERATED, do not hand-edit).
;
; Regenerate with:  troubacore --print-default-config > troubacore.example.ini
; A byte-equality test keeps this file in sync with the code.
;
; Precedence (most specific wins): built-in defaults < this file < TROUBA_* env
; vars < CLI flags. Every key maps 1:1 to an existing TROUBA_* env var, which still
; works as an override. Default file location is ./troubacore.ini (NOT under the
; data dir); override with --config <path> or TROUBA_CONFIG. A missing default file
; is fine; a missing --config file is a startup error.
;
; Every knob below is shown commented-out at its default — uncomment to change.
;
; SECRETS: database_url (and any future smtp.pass) may carry credentials. Prefer
; injecting those via the environment (env still wins over this file), and if you
; do put them here, chmod 600 the file.

`

// smtpSection is a fully-commented, reserved forward hook — no code reads it yet.
const smtpSection = `
[smtp]
; NOT READ BY ANY CODE YET — reserved forward hook for a future self-service
; "forgot password" flow (T21 is deliberately email-free: admins/operators mint
; one-time reset links out-of-band). Shape only.
;host = smtp.example.org
;port = 587
;from = troubacore@example.org
;user =
;pass =
;tls = true
`

// boolStr / parseBool bridge the ini/string world and Go bools. parseBool accepts
// the common truthy spellings so a hand-edited file is forgiving; env bools with
// negative sense are handled in Load.
func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func parseBool(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

package main

import (
	"path/filepath"
	"testing"

	"troubastack/core/internal/config"
)

// The resolution path is the code that actually broke in the field, and the existing
// overlay_skip_test bypasses config entirely — so it is tested here, directly. The load-bearing
// vector is "only candidate #2 exists": "take the first that exists" and the regression "always take
// the first candidate" give different answers, so it discriminates. Teeth: reverting resolveBakeCLI
// to always return config.DefaultBakeCLI makes the first case fail.
func TestResolveBakeCLI(t *testing.T) {
	const exeDir, cwd = "/opt/troubastack", "/home/vll"
	c1 := filepath.Join(exeDir, "bake", "dist", "cli.js")
	c2 := filepath.Join(exeDir, "..", "web", "bake", "dist", "cli.js")
	c3 := filepath.Join(cwd, "..", "web", "bake", "dist", "cli.js")
	only := func(present ...string) func(string) bool {
		set := map[string]bool{}
		for _, p := range present {
			set[p] = true
		}
		return func(p string) bool { return set[p] }
	}

	// Discriminating: only candidate #2 exists. First-that-exists → #2; a naive "always first" → #1.
	if got := resolveBakeCLI(config.DefaultBakeCLI, exeDir, cwd, only(c2)); got != c2 {
		t.Errorf("only #2 present: got %q, want %q (a naive always-first would give %q)", got, c2, c1)
	}
	// #1 (next to the binary) wins when present — the documented non-Docker layout.
	if got := resolveBakeCLI(config.DefaultBakeCLI, exeDir, cwd, only(c1, c2, c3)); got != c1 {
		t.Errorf("#1 present: got %q, want %q", got, c1)
	}
	// None exist → the LAST candidate, so the bake error names a concrete path, not "".
	if got := resolveBakeCLI(config.DefaultBakeCLI, exeDir, cwd, only()); got != c3 {
		t.Errorf("none present: got %q, want the last candidate %q", got, c3)
	}
	// Operator set it explicitly → used VERBATIM, no search, even for a path that does not exist.
	const explicit = "/etc/troubastack/renderer.js"
	if got := resolveBakeCLI(explicit, exeDir, cwd, only(c1, c2, c3)); got != explicit {
		t.Errorf("explicit value: got %q, want it verbatim (%q)", got, explicit)
	}
}

// bestErrorLine must surface the admin-readable failure, not Node's stack-frame header — the whole
// point of the boot warning is a sentence about the deployment (Fable, T128 review). Discriminating:
// the stack-frame line comes FIRST, so "first non-empty line" and "the Error: line" disagree.
func TestBestErrorLine(t *testing.T) {
	node := "node:internal/modules/cjs/loader:1503\n  throw err;\nError: Cannot find module '/x/cli.js'\n    at ..."
	if got := bestErrorLine(node); got != "Error: Cannot find module '/x/cli.js'" {
		t.Errorf("node error: got %q, want the Error: line (not the loader header)", got)
	}
	if got := bestErrorLine("\n\n  just a usage note\n"); got != "just a usage note" {
		t.Errorf("no Error line: got %q, want the first non-empty line", got)
	}
}

package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"troubastack/core/internal/bake"
	"troubastack/core/internal/config"
)

// resolveBakeCLI decides the web/bake worker path (T128). If the operator set TROUBA_BAKE_CLI /
// bake.cli to anything other than the built-in default, it is used VERBATIM — no search, so an
// explicit (even wrong) path surfaces as itself. Otherwise we search candidates BINARY-relative
// first, so a bare-binary self-hoster works with no env var, and keep today's cwd-relative default
// last. First existing candidate wins; if none exist the last is returned, so the eventual bake
// error names a concrete path rather than "".
//
// NB: config.Load hands back a value, not its provenance (CFG01), so an operator who explicitly sets
// the exact default string is indistinguishable from "unset" here and gets the search too. That is a
// deliberate, documented compromise, not an accident.
// resolveBakeCLI delegates to config.ResolveBakeCLI (A61 moved the logic there so cmd/mkbaked shares
// one resolver). Kept as a thin local alias so this file and its test read unchanged.
func resolveBakeCLI(configured, exeDir, cwd string, exists func(string) bool) string {
	return config.ResolveBakeCLI(configured, exeDir, cwd, exists)
}

// preflightBake resolves the bake toolchain at startup and RUNS the worker, not just stats it, so a
// core that cannot bake says so at boot instead of the evening of a gig. It warns and continues — a
// degraded core still serves charts, and a server that boots degraded beats one that will not boot.
// One info line names the absolute resolved path of each tool; a warning names the path + the env
// var that fixes it. (This is a different line from BRAND04's identity line; both stay, each one line.)
func preflightBake(bc bake.Config) {
	pdftoppm := lookPath(bc.Pdftoppm)
	node := lookPath(bc.Node)
	cliAbs := absOr(bc.BakeCLI)
	log.Printf("troubacore: bake toolchain — pdftoppm=%s node=%s renderer=%s",
		orMissing(pdftoppm), orMissing(node), cliAbs)

	if pdftoppm == "" {
		log.Printf("troubacore: WARNING bake unavailable — pdftoppm not on PATH; install poppler-utils or set TROUBA_PDFTOPPM")
	}
	if node == "" {
		log.Printf("troubacore: WARNING bake unavailable — node not on PATH; set TROUBA_NODE")
		return // no point probing the renderer without a node to run it
	}
	if err := probeBakeWorker(node, bc.BakeCLI); err != nil {
		log.Printf("troubacore: WARNING bake unavailable — the web/bake worker at %s did not run: %v; "+
			"set TROUBA_BAKE_CLI to your web/bake/dist/cli.js and ensure @napi-rs/canvas AND its "+
			"platform package (@napi-rs/canvas-linux-x64-gnu) are in the node_modules beside it", cliAbs, err)
	}
}

// probeBakeWorker runs `node <cli>` with no arguments under a short timeout. Given no --in/--out the
// CLI prints "troubabake: usage: …" to stderr and exits non-zero; a module or native-binding load
// failure does NOT print it. So "the usage line came back" is the pass condition — which a plain stat
// cannot distinguish from a cli.js whose @napi-rs/canvas binding is missing (the incident this fixes).
func probeBakeWorker(node, cli string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, node, cli)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	_ = cmd.Run() // exits non-zero on the usage path; the OUTPUT is the signal, not the exit code
	if strings.Contains(out.String(), "troubabake") {
		return nil
	}
	if msg := bestErrorLine(out.String()); msg != "" {
		return fmt.Errorf("%s", msg)
	}
	return fmt.Errorf("no output (timed out or failed to launch)")
}

func lookPath(bin string) string {
	if p, err := exec.LookPath(bin); err == nil {
		return p
	}
	return ""
}

func absOr(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}

func orMissing(p string) string {
	if p == "" {
		return "(not found)"
	}
	return p
}

// bestErrorLine picks the line that tells an operator what is wrong. Node prints a stack-frame
// header first (e.g. "node:internal/modules/cjs/loader:1503"); the useful line — "Error: Cannot
// find module …" — is a few lines down. Prefer that; fall back to the first non-empty line. The
// reader here is an admin who does not know the codebase, so this must be a sentence about their
// deployment, not a citation of Node's internals (Fable, T128 review).
func bestErrorLine(s string) string {
	lines := strings.Split(s, "\n")
	for _, line := range lines {
		if t := strings.TrimSpace(line); strings.HasPrefix(t, "Error:") || strings.Contains(t, "Cannot find") {
			return t
		}
	}
	for _, line := range lines {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

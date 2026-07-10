# T29 — Embed the git version in the binary: REST endpoint + UI info modal

**Priority:** low (VLL: "can be useful… not yet") · **Size:** XS/S · **Area:** `core`
(build + one endpoint) + `web/studio` (about/info affordance) · **Raised by VLL
2026-07-10** during the stale-build incident (the "fix isn't working" report was a
stale browser bundle + a placeholder-SPA binary — with a visible version this would
have been a ten-second diagnosis).

## Context

There is currently NO way to tell what a running instance is: `:8080` may serve a
binary built without `make dist` (the "SPA not embedded" placeholder), a browser may
cache an old bundle against a newer API, and a preview box may run a branch build.
Version visibility turns "the fix isn't working" into "you're on `<sha>` from
Tuesday".

## Changes

1. **Stamp at build:** `go build -ldflags "-X main.version=$(git describe --always
   --dirty) -X main.builtAt=$(date -u +%Y-%m-%dT%H:%MZ)"` wired into the Makefile
   (`dist`/`run`/`demo`); empty ⇒ "dev".
2. **REST:** `GET /api/version` (unauthenticated, like `/healthz`) →
   `{"version":"<git describe>","builtAt":"…","spaEmbedded":true|false}` —
   `spaEmbedded` detected from whether the embedded index is the placeholder (it has
   a recognizable marker). This is the future compatibility-check hook (app ↔ server;
   VLL's stated motivation) — but NO compatibility ENFORCEMENT yet, display only.
3. **SPA:** stamp the same version into the bundle at build (vite `define` from the
   same git describe); show both (SPA build + server version, flagged if they
   differ!) in a small info popover from the footer/nav — the mismatch flag is the
   stale-cache detector.
4. Tests: endpoint shape; the SPA↔server mismatch renders the flag.

## Acceptance criteria

- `make dist && ./bin/troubacore` → `/api/version` returns the real sha; the UI
  popover shows matching SPA/server versions; a deliberately mismatched pair (old
  bundle, new server) shows the mismatch flag.
- `go run ./cmd/troubacore` (unstamped) → "dev", `spaEmbedded:false`, no crash.

## Out of scope

- Any version-gating/compatibility ENFORCEMENT (explicitly "not yet" — the endpoint
  is the hook); app-side (Kotlin) consumption (future task when needed).

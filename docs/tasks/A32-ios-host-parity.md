# A32 — iOS host BLIND parity pass (compiles-green bar; unverified on device)

**Priority:** normal (VLL 2026-07-20: "go ahead with the blind parity pass") ·
**Size:** M · **Area:** `app/shared/iosMain` (MainViewController + a transport
actual), `app/iosApp` README. **Mobile lane.** The bar is HONEST: everything
compiles in CI (iOS klibs + the exported framework); nothing is device-verified
until a Mac exists — label it so everywhere.

## Why now

`MainViewController.kt` predates A27/A31: it mounts a local concerts-list → Stage
wiring, so the iOS entry is two nav generations stale while ALL the real product
(28 commonMain files: Home, Stage + N-series, identity, cues, vectors) already
cross-compiles green. Drift compounds; this pass stops it.

## Scope (mirror MainActivity's four pieces)

1. **Mount the shared Home/nav** (A27/A31): Home → TroubaStage (concerts list,
   perform intent → StageScreen) / TroubaStudio (the EXISTING WKWebView host,
   embedded URL via `embeddedUrl()`) / identity line. Delete the local
   ConcertsScreen duplicate.
2. **iOS transport**: a Darwin-engine ktor `ManifestTransport` + the login/probe
   surface (mirror HttpTransport: origin-bound session per the A-track fix,
   `MeResp` wrapper, `probePresence`) — Keychain-backed via the existing Storage
   seam. Pure logic stays common; only the engine + cookie plumbing is iosMain.
3. **P201 host loop**: the auto-update poll (the MainActivity LaunchedEffect
   equivalent) keyed on the transient toggle; identity auto-match feed.
4. **Docs**: iosApp README updated (what's wired, what's UNVERIFIED, the
   first-Mac checklist: xcodegen, sim run, the ACCEPTANCE-P205 + R10 scripts to
   re-run on iOS).

## Explicitly OUT

- InkOverlay stays TODO (iOS never needs native ink — Edit is the WebView).
- Any claim of device behavior. No screenshots exist; say so.
- Simulator/device CI (needs macOS runners — a later, VLL-triggered add).

## Acceptance

- CI green: iOS klibs + framework export compile with the new host wiring; all
  existing shared tests untouched/green.
- Pure new logic (transport parsing, nav state) unit-tested in commonTest/iosTest
  where testable off-device.
- The UNVERIFIED label present in the README + the landing memo; the first-Mac
  checklist written.
- Gate as usual (code review is the verification here — state claims carefully).

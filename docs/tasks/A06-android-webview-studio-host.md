# A06 — Android WebViewHost actual: TroubaStudio in the app

**Priority:** A-track 6 (after A01; independent of A02–A05) · **Size:** M · **Area:** `app/shared` seams, `app/androidApp`

## Context

The app's second job (besides Stage) is hosting the **canonical** web editor in a
WebView — invariant I10: the editor is *never* reimplemented natively; the pure-web
Studio is the baseline and the app is a shell around it. Seam 1's contract is sketched
in `seams/WebViewHost.kt` (common + Android actual, currently `TODO()`).

Keep this task deliberately modest: **host Studio and make it fully usable** (login,
edit, realtime sync — all of which Studio already does by itself). The JS bridge gets a
working *transport* with a handshake, but no product features ride on it yet — the
bridge's real client is the ink overlay (A07, blocked) and until then Studio needs
nothing from the shell.

## Changes

1. **WebViewHost actual (androidMain):**
   - Wrap `android.webkit.WebView`: JavaScript on, DOM storage on (Studio keeps its
     session), `allowFileAccess = false`, safe defaults otherwise. Expose the underlying
     view so `androidApp` can put it in a Compose `AndroidView`.
   - `load(url)`; back-navigation support (expose `canGoBack`/`goBack` — wire to the
     Android back callback in `androidApp` so back navigates Studio history before
     leaving the screen).
   - Bridge transport per the stub's plan: `WebMessagePort` (fall back to
     `@JavascriptInterface` + `evaluateJavascript` if port setup fights you — note which
     you shipped). `postToStudio(message)` / `onStudioMessage(handler)` carry raw JSON
     strings, as the `expect` declares.
2. **Handshake (the only bridge traffic for now):** on page load, the shell sends
   `{"type":"hello","shell":"troubashare-android"}`. In `web/studio`, add a ~20-line
   `bridge.ts`: if a shell port/handler is present, reply `{"type":"ready"}` and export
   a `postToShell(json)` no-op-safe helper for later. No other Studio behavior may
   change; pure-browser Studio must be completely unaffected (feature-detect, I10).
3. **Server URL config (dev reality):** a small settings screen (or dialog) in
   `androidApp` for the core URL, persisted via the Storage seam prefs; default
   `http://10.0.2.2:8080` (host machine from the Android emulator). Dev builds must
   allow cleartext HTTP for LAN use: add a `network_security_config.xml` permitting
   cleartext for **debug builds only** — do not enable it in release config.
4. **Screen:** an "Edit" entry point beside "Concerts" opening the hosted Studio.
   Loading and error states (server unreachable ⇒ message + retry + back, not a blank
   WebView).

## Acceptance criteria

- With `make run` (or `make demo`) serving on the host: emulator app → Edit → log in as
  `marie`/`demo`, open Wonderwall, draw a stroke — and see it appear live in a desktop
  browser session of the same song (proves the full editor works inside the shell).
- Bridge handshake proven: shell logs Studio's `ready` reply (logcat screenshot or log
  line quoted in the PR).
- Pure-browser Studio unaffected: `make e2e` still green (bridge code is
  feature-detected and inert in the browser).
- Back button navigates Studio history, then exits the screen. Unreachable server shows
  the error state.
- `./gradlew :shared:check :androidApp:assembleDebug` green; platform code still confined
  to the seam files + `androidApp` (I15).

## Out of scope

- The ink overlay and any real bridge protocol beyond the handshake (A07 — blocked);
  embedding Studio's built assets into the APK (serving from core is fine for now; note
  it as a future offline-editing decision); auth token sharing between shell and WebView.

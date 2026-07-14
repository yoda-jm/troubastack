# T46 — Studio embedded mode (WebView hosting, I10)

**Priority:** normal (unblocks the mobile lane's Q1 app-bar work; VLL device QA
2026-07-14: Edit "feels like you launched a browser") · **Size:** S · **Area:**
`web/studio` (Shell + a session flag). The web-core half of the mobile-integration
ruling (reviews.md 2026-07-14); the app-bar half is the mobile lane's.

## Context

The Android app hosts Studio in a WebView (A06). Studio's own topbar (Bands/Invites/
profile/Log out — `Shell.tsx:130`) duplicates the app's chrome and makes Edit read as
an embedded browser. I10 says the editor stays web-only — so the fix is a Studio
"embedded mode" the app opts into, not a native editor. RULED: the signal is a URL
param at first load, NOT the A06 JS bridge (the handshake lands after first paint →
the nav would flash; and a param is e2e-testable in plain Playwright, no WebView).

## Changes

1. **Signal:** `?embedded=1` on any entry URL → set a session-wide flag (sessionStorage
   + app state) so it survives SPA navigation; absent param + absent stored flag =
   normal Studio. No bridge dependency for layout (the bridge may corroborate later
   for back-button integration — out of scope here).
2. **When embedded:** suppress the Shell topbar entirely (it already self-hides in the
   T27 fullscreen editor — generalize that conditional), and hide **Log out + account
   management** everywhere (the app owns the session it cookie-seeds; an in-WebView
   logout would silently break the app). Everything else is normal responsive Studio.
3. **Contract note for the mobile lane** (in the handoff): the app appends
   `?embedded=1` and deep-links contextually (`/bands/{id}/songs/{id}?embedded=1`).

## Acceptance criteria

- Plain-Playwright e2e: load with `?embedded=1` → no topbar, no logout affordance,
  flag survives client-side navigation to another page; load WITHOUT the param →
  Studio unchanged (explicit regression assertion). Pixels light+dark at the gate.
- `tsc -b` clean; no editor-behavior change (zeroshift/editor suites untouched).

## Out of scope

- The app-side bar/drawer (mobile lane); bridge back-button integration; any editor
  change; hiding anything beyond chrome (embedded users keep full Studio function).

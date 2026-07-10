# T32 — `crypto.randomUUID` missing on plain-HTTP origins: all creation silently throws

**Priority:** CRITICAL (field: the product cannot create annotations/layers on any
self-hosted plain-HTTP deployment — the primary deployment story) · **Size:** XS/S ·
**Area:** `web/studio` (+ e2e guard) · **Found:** VLL field report 2026-07-10, root
cause via his console: `Uncaught TypeError: crypto.randomUUID is not a function`.

## Context

`crypto.randomUUID()` exists only in **secure contexts** (HTTPS or localhost).
VLL's box is reached at `http://troubashare.leligeour.net:8080` — non-localhost
plain HTTP → insecure context → the API is absent. Every create path throws on
pointerup: `buildObject` (`src/editor.ts:142`), layer creation
(`Viewer.tsx:379`), and `Viewer.tsx:584`. The exception escapes `commitDraw`
BEFORE any T30 notice and before WetCanvas's `repaint()`, which also explains the
signature symptom: the wet stroke lingers until the next repaint (tool change),
then vanishes. Nothing is ever sent — "no network traffic, no error surface."

**Why no test ever caught it:** every e2e (local + CI) drives `http://localhost`,
which browsers treat as a SECURE context even over plain HTTP. The bug class is
invisible on localhost by construction.

## Changes

1. **`newUuid()` helper** (in `src/editor.ts` or a tiny `src/uuid.ts`): use
   `crypto.randomUUID()` when available, else a RFC-4122 v4 built from
   `crypto.getRandomValues` (available in insecure contexts): set version/variant
   bits, hex-format. Replace all three call sites. No new dependency.
2. **T30 completion — exceptions are silent ink too (VLL directive, 2026-07-10:
   "this kind of error needs to be caught and made visible to the user; it is not
   normal to just die silently"):** TWO layers, both required —
   a. **Targeted:** wrap the commit dispatch (`commitDraw` / layer-create
      handlers) so a thrown error surfaces through the existing T30 notice
      surface ("Couldn't place the annotation — <message>"); log via
      `console.error` for forensics.
   b. **Global backstop:** `window.addEventListener("error")` +
      `"unhandledrejection"` wired (in Shell) to a dismissible error banner
      showing the message — so ANY uncaught client error is visible to the user,
      not just commit-path ones. Keep it minimal: one banner component, latest
      error + dismiss, no error queue/reporting service. A React error boundary
      around the routed content for render crashes (same banner styling, with a
      reload hint).
3. **The e2e guard (the class-killer):** a spec that emulates the insecure
   context with `page.addInitScript(() => { delete (crypto as any).randomUUID })`
   (v8: deleting the prototype getter on Crypto — verify it actually removes it in
   Chromium; otherwise override with `undefined` via Object.defineProperty), then
   runs the standard draw flow and asserts the object commits + paints. This must
   FAIL on pre-fix code (proving it emulates VLL's box) and pass post-fix.

## Acceptance criteria

- The guard spec fails on pre-fix code with the same TypeError class, passes
  post-fix; the full editor e2e stays green.
- `newUuid()` output format matches server expectations (same shape as
  `crypto.randomUUID`: lowercase hyphenated v4).
- Draw + new-layer + (the `Viewer.tsx:584` path) all work with `randomUUID`
  deleted.
- A thrown commit-path error shows the T30 notice (assert via the guard spec on
  pre-fix behavior if convenient, or a targeted unit).
- The global backstop: a spec that injects a throwing handler (or re-deletes
  `randomUUID` with the targeted catch bypassed — executor's choice of cleanest
  forced-error) and asserts the error banner appears with the message and is
  dismissible. An uncaught rejection surfaces too.

## Out of scope

- TLS on the public box (OPS01's decision — this fix must work on plain-HTTP LAN
  deployments regardless, that's the product's self-hosted story).
- Polyfilling anything else; only the uuid call sites.

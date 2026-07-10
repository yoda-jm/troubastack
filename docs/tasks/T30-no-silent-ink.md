# T30 — No silent ink: every can't-commit state is visible (banner/read-only or notice)

**Priority:** medium-high (VLL UX principle, 2026-07-10: "if something cannot land,
popup an error or make the canvas strongly readonly") · **Size:** S ·
**Area:** `web/studio` (Viewer/WetCanvas/useSongSync presentation)

## Context

T28 fixed the worst silent-swallow (hidden active layer → auto-reveal), and server
rejections already surface (`reject-notice`, `role="alert"`). But the editor still
accepts ink gestures in states where the stroke can never land, and stays silent:

1. **`ensureActiveLayer()` returns null** (no file selected / not signed in / no
   creatable layer): `commitDraw` bails silently — the wet stroke renders, then
   evaporates with no message.
2. **Disconnected** (`connStatus` ≠ live): drawing appears optimistic; whether the
   mutation is queued, lost, or rejected on reconnect is invisible to the user.
3. Any future gate (RO-only context, final-locked, etc.) that declines a commit
   client-side.

Principle (VLL's, adopted): **the canvas must never eat a gesture silently** —
either it visibly can't be drawn on (read-only presentation), or the failure says
so at commit time.

## Design (resolved)

1. **Read-only presentation, up-front, for STATIC can't-commit states** — when there
   is no file, no session, or the user cannot write ANY layer of the current file:
   - draw tools disabled (grayed) in the toolbar; canvas cursor `not-allowed` while
     a draw tool would otherwise arm;
   - a slim persistent chip in the chrome: "Read-only — <reason>" (reuses the
     `.chip`/notice vocabulary; `data-testid="editor-readonly"`).
2. **Commit-time notice for DYNAMIC failures** — extend the existing
   `reject-notice` surface (same testid + alert role) to client-side declines:
   `ensureActiveLayer() === null` → "Couldn't place the annotation — <reason>";
   auto-clear on next successful commit (matches current reject behavior). The wet
   stroke is CLEARED when the commit is declined (no phantom ink lingering).
3. **Disconnected**: while `connStatus` ≠ live, show the chip ("Offline — changes
   can't be saved") and treat as read-only per (1); on reconnect the chip clears.
   (The sync client's queue semantics are NOT changed here — presentation only; if
   queued-offline mutations are ever wanted, that's its own task.)

## e2e

- Disconnected: kill the WS (route abort / server pause) → chip appears, draw tools
  disabled, a canvas drag leaves NO wet residue and NO object.
- No-file/no-session static state → chip + disabled tools.
- Dynamic decline (force ensureActiveLayer null, e.g. no file selected) → the
  reject-notice text appears; wet canvas cleared; `object-count` unchanged.
- Regression net: full editor suite still green (the read-only presentation must not
  trigger in normal editable states — assert its absence in one happy-path spec).

## Acceptance criteria

- `tsc -b studio` clean; new spec green + full editor suite green.
- Behavior: in every enumerated can't-commit state the user sees WHY (chip or
  notice) and never loses a stroke silently.

## Out of scope

- Offline mutation queueing/replay; server-side gate changes; Stage (read-only by
  design); the T29 version surface (separate).

# T10 — Split SongEditor.tsx

**Priority:** 10 (do **after** T05, T06, T07 have landed) · **Size:** L · **Area:** `web/studio`

## Context

`web/studio/src/pages/SongEditor.tsx` is ~3,000 lines and fuses at least six concerns:
PDF.js rasterization + zoom/DPR management, the dry overlay painting, the wet edit
canvas + pointer gesture state machine, the style/tools toolbar, the layers +
annotations side panels, and sync wiring (WebSocket, outbox reconcile). It is the
maintenance bottleneck of the whole SPA — every editor task collides in this one file.

This is a **mechanical extraction** task: move code, don't change behavior. It comes
after T05/T06/T07 because those rewrite parts of what would be extracted.

## Changes

Extract into modules under `web/studio/src/editor/` (or `pages/song-editor/`), keeping
`SongEditor.tsx` as the composition root (~300 lines target):

1. `usePdfDocument.ts` — PDF.js loading, per-page rasterization, zoom + DPR handling,
   re-rasterize-on-zoom-settle logic.
2. `useDryOverlay.ts` — the committed-objects overlay canvases + `paintOverlay` (calls
   `@troubastack/ink renderObjects`; this stays the only dry path — invariant I8).
3. `useWetCanvas.ts` (or `WetCanvas.tsx`) — the edit canvas, pointer handlers, the
   gesture state machine (`gestureRef` union), the T06 fast freehand path.
4. `useSongSync.ts` — WebSocket lifecycle, outbox/echo reconciliation, rejection
   rollback (invariants I2/I6 semantics — move verbatim).
5. `Toolbar.tsx` — the T05 toolbar (tools, style controls, layer picker).
6. `SidePanels.tsx` — layers + annotation-list panels.
7. Shared editor state that several pieces need (active tool, selection, active layer)
   goes in one plain context or a lifted-state object — pick the simplest thing that
   avoids prop-drilling ten levels; do **not** introduce a state-management library.

Rules for the move:
- Zero behavior change. No renamed user-visible strings, no changed `data-testid`s.
- Move code in reviewable steps (one commit per extracted module) so the diff is
  auditable as pure relocation.
- Where a module boundary forces an interface, keep it a plain function/props interface;
  no new abstractions beyond what the seam needs.

## Acceptance criteria

- `SongEditor.tsx` ≤ ~400 lines; no extracted module > ~600 lines.
- `make e2e` green **without editing any spec** (this is the no-behavior-change proof).
- TS typecheck green; `make demo` manual smoke: draw/select/move/resize/delete, layer
  switch, zoom, file tabs, two-session realtime echo all work as before.

## Out of scope

- Any feature or visual change; any sync protocol change.
- Unit tests for the extracted hooks (welcome, but not required here).

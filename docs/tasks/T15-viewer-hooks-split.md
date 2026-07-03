# T15 — Split Viewer.tsx into pdf/overlay/sync hooks (T10 part 2)

**Priority:** after T10 · **Size:** M · **Area:** `web/studio/src/pages/song-editor`

## Context

T10 (commit 1, `01d98d7`) extracted the song editor into `pages/song-editor/`:
`SongEditor.tsx` is now a 95-line composition root, and Toolbar / WetCanvas /
SidePanels / MyFilesEditor / SongDetails / helpers are all ≤ ~400 lines. But two
files still exceed the T10 "no module > ~600" target:

- **`Viewer.tsx` ≈ 1,260 lines** — the orchestrator: PDF.js load/raster/zoom+DPR,
  the dry overlay (`paintOverlay` → `@troubastack/ink`, I8), the file strip, and
  realtime sync (WebSocket outbox/echo reconcile, reject rollback).
- **`WetCanvas.tsx` ≈ 690 lines** — marginally over.

This was deliberately deferred: it is the sync-sensitive part. `doc`
(`{layers, objects}`) is the shared spine — the `SyncClient` `onState`/`onReject`
callbacks drive it, the PDF layer-default logic reads it, the overlay paints it,
and the editing handlers mutate it optimistically — so the hooks below must
thread that shared state carefully, and a subtle regression here is exactly what
the no-flicker (`pdf-render-count`) and echo/rollback invariants guard.

## Changes

Extract from `Viewer.tsx`, keeping it as the orchestrator (~400–500 lines):

1. `usePdfDocument.ts` — PDF.js load, per-page rasterization, zoom/DPR, fit-scale
   math (`pageScale`, `scale`, `stepZoom`, `onZoomSelect`), `pdfRenderCount`. It
   owns `pdfDocRef` / `pageCanvasRefs` / `pageSizesRef` / `scrollRef` and returns
   the values + refs the JSX binds. Preserve the "no re-raster on annotation
   edit" behavior (the effect deps must stay `[selectedFile, status, scale,
   numPages, zoomMode]`, NOT objects/visibility).
2. `useDryOverlay.ts` — `paintOverlay` + `overlayRefs`; repaints committed objects
   via `renderObjects` (stays the ONLY dry path — I8) without re-rastering.
3. `useSongSync.ts` — the `SyncClient` lifecycle, `onState`→`doc`, `onReject`
   rollback + notice, and the optimistic mutation senders used by
   commitDraw/Move/Resize. Returns `{ doc, connStatus, rejectNotice, send… }`.

## Acceptance criteria

- `Viewer.tsx` ≤ ~600 (ideally ~450); `WetCanvas.tsx` trimmed ≤ ~600 if cheap.
- Zero behavior change: `make e2e` green **without editing specs** (run to
  completion on an unloaded machine — the T10 local run timed out on load at
  47/56 with 0 failures; CI is the reliable gate).
- Two-session realtime echo + no-flicker (`pdf-render-count` unchanged on edit)
  verified manually via `make demo`.
- TS typecheck green.

## Out of scope

- Any behavior/feature change; the shared-state design (plain hooks + a lifted
  state object, no state-management library) from T10's rules still applies.

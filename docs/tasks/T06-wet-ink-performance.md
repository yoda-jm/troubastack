# T06 — Low-latency wet-ink drawing path

**Priority:** 6 · **Size:** M · **Area:** `web/studio/src/pages/SongEditor.tsx`, `web/ink`

## Context

Freehand stylus latency is the make-or-break requirement for this product on tablets
(see `docs/design/03-rendering-and-ink.md` — "the web-ink spike" is called the first
build step, and a native overlay is only justified if the *optimized* web path fails).
The current wet path is unoptimized and would fail that test for the wrong reason:

- Every `pointermove` runs a React state update, then `repaint()` clears the wet canvas
  and re-renders the **entire** in-progress stroke via `renderObjects` → perfect-freehand
  `getStroke` over the whole point array (see the gesture handling and `repaint` in
  `SongEditor.tsx`, roughly lines 2237–2460). Cost grows with stroke length.
- The design doc's named mitigations are absent: no `getCoalescedEvents()`, no
  `pointerrawupdate`, no `desynchronized: true` canvas context (grep confirms none
  appear anywhere in `web/`).

Goal: make the in-browser wet path as fast as the platform allows, per the design doc.
Constraints from the architecture: committed rendering must keep going through
`@troubastack/ink` `renderObjects` (invariant I8 — one authoritative dry renderer), and
all stored geometry stays normalized `[0,1]` (I3). Only the *transient wet preview* may
use a faster private path.

## Changes

1. **Take the wet stroke out of React.** During an active freehand gesture, accumulate
   points in a mutable ref and draw imperatively; no `setState` per move. Commit still
   flows through the existing state/sync path on pointer-up (unchanged).
2. **Coalesced + high-frequency input.** In the pointer handlers, use
   `e.getCoalescedEvents()` (fall back to `[e]` when unsupported) so fast styluses don't
   drop samples. Optionally register `pointerrawupdate` on the edit canvas when
   available (feature-detect; keep the `pointermove` fallback).
3. **Desynchronized canvas.** Request the wet canvas 2D context with
   `{ desynchronized: true }` (harmless no-op where unsupported).
4. **Incremental rendering.** Recompute + refill only what changed: keep the stroke
   rendered up to the last "stable" segment on the canvas (or in an offscreen cache) and
   redraw only the tail since the last few points, instead of clear-all +
   full-`getStroke` each frame. One workable scheme: every N points, stamp the current
   outline into an offscreen canvas; per frame, blit the cache + draw only the recent
   tail with `getStroke` on a small slice. On pointer-up, do one final authoritative
   `renderObjects` pass so the committed preview is pixel-identical to the dry render
   (this final pass keeps I8 honest — a tiny visual settle at pen-up is acceptable per
   design doc §"wet→dry handoff").
5. **Frame pacing.** Coalesce draws with `requestAnimationFrame` — accumulate points
   between frames, render once per frame.
6. Non-freehand tools (line/rect/ellipse preview) may keep the simple full-repaint path;
   they render O(1) geometry.
7. Add a temporary dev-only instrumentation hook (e.g. behind
   `localStorage.inkPerf === "1"`): log points/sec received and average
   pointer-event→paint delta, so the tablet spike has numbers.

## Acceptance criteria

- Manual: `make demo`, draw long freehand strokes (several seconds, whole-page
  scribbles). With the perf hook on, per-frame render time stays roughly **constant** as
  the stroke grows (was: linear growth). No visual difference in the committed result.
- The committed object's stored points are unchanged in format (normalized, with
  pressure when present) — verify by drawing, reloading, and confirming the stroke
  re-renders identically.
- Two-browser realtime check still works (open the song in two sessions; strokes appear
  on the peer on commit).
- `make e2e` green (the editor specs draw via synthetic pointer events; if a spec
  depended on per-move state updates, adapt the spec, not the optimization).
- TS typecheck green.

## Out of scope

- The native (Kotlin) overlay — explicitly later, and only after this path is measured
  on the real tablet.
- Changing `web/ink`'s public API or the dry-render path.
- Zoom/viewport changes.

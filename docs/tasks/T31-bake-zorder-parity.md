# T31 — Bake ignores per-object z-order: baked overlays can stack differently than studio (I8 parity)

**Priority:** high (correctness — studio and Stage disagree) · **Size:** XS/S ·
**Area:** `web/bake` (render order) + a parity test · **Found:** 2026-07-10 arch audit.

## Context

T27 stage 2 gave objects a within-layer z-order: studio's dry render (and pick) sort
by `order → createdAt → uuid` (`compareObjectZ`). But the baked overlays are rendered
at bake time by `web/bake`, whose renderer still draws objects **in document/API
order** — `web/bake/src/render.ts:127` even documents it: *"Within a layer, objects
render in document order … matching studio's dry layer"* — a comment stage 2 silently
invalidated. Consequence: **a bring-to-front / send-to-back done in studio is IGNORED
in the baked output** — the performer's Stage shows the pre-reorder stacking. This is
exactly the I8 class (bake must render what studio shows). The REST annotation DTOs
already carry `order` and `createdAt` (added in the stage-2 landings), so the fix is
contained in `web/bake`.

## Changes

1. `web/bake/src/render.ts`: before rendering a layer's objects, sort them by
   `(order ?? 0)`, then `(createdAt ?? 0)`, then `uuid` — the exact `compareObjectZ`
   contract (studio's `helpers.ts`). Fix the stale comment. (Ink's `renderObjects`
   renders in array order by design — the caller owns ordering.)
2. **Parity test:** extend the bake test suite with two overlapping opaque rects on
   one layer where `order` INVERTS document order; assert the rendered overlay's
   overlap pixel is the high-`order` object's color (mirrors studio's
   `editor-zorder` pixel assertion). Without the sort this test must fail.
3. While there: assert equal-`order` objects fall back to `createdAt`/`uuid` so the
   bake tiebreak matches the client (one shared fixture is enough).

## Acceptance criteria

- The new parity test fails on pre-fix code and passes after; `npm test` in
  `web/bake` green; the B01 golden pixel-parity suite still green.
- A studio bring-to-front followed by a bake produces a `.tstage` whose overlay shows
  the reordered stacking (covered by the pixel test; no live-bake step required).

## Out of scope

- Kotlin/Stage changes (overlays arrive pre-rendered — order is baked in);
  cross-layer stacking (layer-major order is already correct).

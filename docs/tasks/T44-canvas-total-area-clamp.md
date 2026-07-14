# T44 — Total canvas-area budget clamp (stop mobile black pages)

**Priority:** HIGH (field: pages render black on Android mid-rehearsal — VLL confirmed
on-device 2026-07-14) · **Size:** S · **Area:** `web/studio` (raster scale math) + a
unit-shaped test. Follows T41 (DPR clamp, insufficient alone). **Ruling: approach (A).**

## Context

Definitive diagnosis (VLL on-device: zoom-in→black, zoom-out→recovers): the studio
renders + keeps a canvas for EVERY page at once (T27 single-transform-target), three
canvases each (raster + overlay + wet), re-rastered larger on zoom → cumulative
backing-store memory is unbounded on a phone → later/zoomed pages fail to allocate and
paint solid black. T41 capped DPR at 2; the SUM across pages still blows the budget.

**Concrete worst case (re-analysis 2026-07-14, from the code):** `MAX_ZOOM_SCALE = 5.0`
(`usePdfDocument.ts:35`). An A4 page (595×842 pt) at s=5, dpr=2 → a 5950×8420 canvas =
50 MP = **200 MB RGBA per canvas, ×3 canvases per page**. Even a moderate pinch to s≈2
is ~32 MB × 3 × pages. Two failure modes on Android Chrome, BOTH observed:
1. **Allocation failure / eviction under pressure** → black canvas. Zoom-out re-rasters
   smaller at settle → "recovers". This is the primary mechanism.
2. **Backing-store eviction of an ALREADY-RENDERED page** (2D context loss): Chrome
   discards canvas backings under memory pressure and the render effect only re-rasters
   on `[…, scale, zoomMode, renderNonce]` change — so a page evicted after earlier
   zooming stays black EVEN AT FIT until something re-triggers. This explains VLL's
   "page 2 black at fit". Prevention alone doesn't fully cover this (any other app can
   pressure memory); T44 needs the recovery hook too (change 4).
Also relevant: 5950×8420 exceeds the common 4096 px GPU max-texture dimension on both
axes — some drivers black-out or software-fallback such canvases regardless of total
memory (change 2 covers it).

## Changes (approach A — total-area budget, NOT virtualization)

1. A shared budget: `MAX_TOTAL_CANVAS_AREA_PX` (a mobile-safe ceiling on the SUM of all
   page canvas areas — tune against VLL's device; start conservative, e.g. ~24–32 MP of
   backing store total across pages × the 3 layers, i.e. per-page ≈ budget / (pages×3)).
2. Derive the raster scale so `pages × 3 × (pageW·pageH·scale²·dpr²) ≤ budget`, i.e.
   `scale ≤ sqrt(budget / (pages×3×pageArea×dpr²))`, then use
   `min(zoomScale, thatCap)` for the raster resolution. **Only the RASTER resolution is
   clamped — the CSS display size (layout) and the transform-zoom are unchanged**, so a
   page past the budget renders SOFTER, never black, and zero-shift/scrollIntoView are
   untouched. Route through the existing `rasterDpr()` site(s) so raster + overlay + wet
   stay pixel-aligned (T41 already unified them).
3. Make the derivation a PURE function (`rasterScaleForBudget(pageCount, pageArea, dpr,
   zoom)`) so it's unit-testable off-device.
4. **Context-loss recovery (cheap, required):** add `contextlost`/`contextrestored`
   listeners on the page canvases (Chrome supports these for 2D contexts) that bump the
   EXISTING `renderNonce` (the mechanism is already there — `usePdfDocument.ts:315`
   uses it for the settle nudge). An evicted page then repaints itself instead of
   staying black forever. This is the fix for the "black at fit" half of the symptom.
5. **Per-canvas dimension cap:** also clamp each canvas's raster so neither side exceeds
   ~4096 px (the common Android GPU max-texture floor) — fold it into the same pure
   function. The total-area budget usually implies this for multi-page docs but not for
   a 1-page score at s=5.
6. **Optional hardening (small, no invariant risk):** WetCanvas keeps its `cache` +
   `compose` scratch canvases (`WetCanvas.tsx:283,314`) at FULL canvas size forever once
   a freehand stroke happens on a page — 2 extra full-size buffers per drawn page.
   Release them (width=height=0) on gesture end.

## Acceptance criteria (split — honest about device dependence)

- **Unit-testable (CI):** `rasterScaleForBudget` keeps total derived canvas area ≤ the
  budget as page-count/zoom grow, caps each side ≤ the dimension limit, never returns
  ≤0, and equals the unclamped zoom scale when under budget (desktop/few-pages
  unaffected — no regression). Full editor suite + zeroshift spec green; `tsc -b` clean
  (proves layout/zero-shift untouched).
- **Recovery (CI-approximable):** a spec (or unit test on the handler) that a
  `contextrestored` dispatch on a page canvas triggers a re-raster (renderNonce bump).
- **On-device (VLL):** the black page is GONE on his Android at fit AND zoomed in — this
  is the real acceptance; headless cannot reproduce the failure. Report the confirmation.

## Out of scope

- Page virtualization / releasing off-screen canvases (that's T45, deferred — the proper
  fix if this clamp proves insufficient or a many-page score needs zoomed sharpness);
  any layout/zero-shift change; the PDF content path (poppler renders fine).

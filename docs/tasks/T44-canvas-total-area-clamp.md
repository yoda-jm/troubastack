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

## Acceptance criteria (split — honest about device dependence)

- **Unit-testable (CI):** `rasterScaleForBudget` keeps total derived canvas area ≤ the
  budget as page-count/zoom grow, never returns ≤0, and equals the unclamped zoom scale
  when under budget (desktop/few-pages unaffected — no regression). Full editor suite +
  zeroshift spec green; `tsc -b` clean (proves layout/zero-shift untouched).
- **On-device (VLL):** the black page is GONE on his Android at fit AND zoomed in — this
  is the real acceptance; headless cannot reproduce the failure. Report the confirmation.

## Out of scope

- Page virtualization / releasing off-screen canvases (that's T45, deferred — the proper
  fix if this clamp proves insufficient or a many-page score needs zoomed sharpness);
  any layout/zero-shift change; the PDF content path (poppler renders fine).

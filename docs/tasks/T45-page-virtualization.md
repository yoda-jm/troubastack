# T45 — Page virtualization (canvas allocation near the viewport) — DEFERRED

**Priority:** DEFERRED (build only if T44's total-area clamp proves insufficient
on-device, or a genuinely many-page/orchestral score needs full sharpness at zoom) ·
**Size:** M/L · **Area:** `web/studio` (usePdfDocument render model + scroll/zoom).

## Context

The correct long-term fix for mobile canvas-memory (T41/T44): instead of allocating a
canvas for EVERY page, allocate/raster only pages near the viewport (IntersectionObserver)
and RELEASE off-screen ones (clear + zero-size), so total memory is bounded to ~2–3 pages
regardless of page count OR zoom — and zoomed pages stay SHARP (no total-area softening).
This is how pdf.js's own viewer works.

## Why deferred (not now)

It TOUCHES T27's single-transform-target render model + the scroll/zoom math + the
overlay/edit canvases, and must not regress the **zero-shift** invariant or the
`scrollIntoView`/list-jump specs — a real, staged change that deserves its own careful
review, not a rush under a field-bug clock. T44 (total-area clamp) stops the acute black
safely in the meantime; build T45 only when its sharpness cap actually bites.

## Sketch (when built, stage it)

- Observe page visibility; keep a window of N pages (current ± 1–2) rastered; release the
  rest (clear canvas, set width/height 0) and re-raster on re-entry.
- Preserve zero-shift: released pages keep their LAYOUT box (a placeholder at the page's
  display size) so scroll position + `scrollIntoView` are unchanged; only the backing
  store is freed.
- The overlay + wet canvases follow the same visibility window (they already share
  `rasterDpr()` + the page box).
- Re-verify EVERY editor invariant spec (zeroshift, noflicker, wheelzoom one-raster,
  touch pinch one-raster, pick/hit-test) — this is the high-risk part.

## Acceptance criteria (when built)

- Memory bounded regardless of page count (a 22-page concert allocates ~3 pages' canvases);
  zoomed pages stay sharp; on-device black-page gone at any zoom; ALL editor invariant
  specs green; zero-shift/scrollIntoView unregressed.

## Out of scope

- Anything until it's un-deferred by the architect on evidence T44 is insufficient.

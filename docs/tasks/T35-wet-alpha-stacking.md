# T35 — Slow freehand at reduced opacity shows dark bands (wet alpha-stacking)

**Priority:** normal-high (field UX, VLL 2026-07-11) · **Size:** S ·
**Area:** `web/studio` WetCanvas (wet compositing) + a capture-time point filter ·
**Research:** requested by VLL ("document the method that solves this without
breaking the invariants") — the analysis below IS the deliverable's first half.

## The artifact, precisely

Drawing freehand SLOWLY at opacity < 1 shows darker stacked bands while the
stroke is wet. Root cause chain (all verified in code):

1. The T06 incremental wet path (`WetCanvas.drawWetFrame`) bakes stable stroke
   segments into a cache canvas as the stroke grows. Each segment renders via
   `renderObjects` → `globalAlpha = opacity`. Consecutive segments deliberately
   overlap by `WET_OVERLAP` points (to hide joins) — **where they overlap, alpha
   accumulates** (α + α(1−α): 50%+50% → 75%). The live tail then draws OVER the
   blitted cache with a third coat at the seam.
2. Slow drawing = dense input points = more segment bakes per screen-distance =
   more seams; `simulatePressure` also widens slow strokes, enlarging overlaps.
3. The DRY render is immune: `drawFreehand` builds ONE closed perfect-freehand
   outline and fills ONCE (single fill = one coat per pixel, even where the
   outline self-overlaps). The artifact therefore vanishes on pointer-up — it is
   a WET-ONLY compositing bug, not a geometry problem.

## Why NOT Bézier/smoothing (VLL's worry, answered)

Curve smoothing is neither necessary nor sufficient: the stacking comes from
multi-pass alpha compositing, not from segment geometry — a perfectly smooth
curve rendered in overlapping passes at α still stacks. (For the record: since
bake renders through the SAME ink code, an ink-internal smoothing change would
NOT break I8 parity per se; the real risks would be wet-tail vs final-stroke
divergence and pick-vs-paint mismatch. Moot — we don't need it. perfect-freehand
already smooths via its `smoothing`/`streamline` params.)

## The fix (two independent, invariant-safe changes)

1. **Uniform-alpha wet compositing (the actual fix).** Apply the precedent ink
   ALREADY ships for fill+stroke shapes (`paintShape`'s "Box" path: render
   OPAQUE offscreen → blit ONCE at the object's opacity — "no double-blended
   rim"):
   - Bake cache segments and the live tail **at opacity 1** (strip opacity in
     the style handed to `buildWet` for the wet path; keep color/width).
   - Compose per frame: clear a compose surface → draw the opaque cache →
     draw the opaque tail → **blit the composite to the wet canvas ONCE with
     `globalAlpha = opacity`** (and the object's blend, if any).
   - Implementation freedom: the "compose surface" can be the existing cache
     canvas + tail drawn into a second small offscreen, or one full compose
     canvas — executor's call; the requirement is exactly ONE alpha application
     per frame for the whole stroke. Perf stays bounded (the per-frame additions
     are one drawImage and the same short tail).
   - I8 unaffected: this changes only the LIVE preview's compositing; the
     committed object, the dry render, and the bake are untouched.
2. **Capture-time min-distance filter (the diet — VLL's "simplification"
   instinct, done at the data level).** In the pointermove capture, drop points
   closer than ε (page-relative, e.g. ~0.15% of page width — tune visually) to
   the last KEPT point; always keep the final point. Effects: slow strokes stop
   storing hundreds of near-duplicate points (payload + store size), fewer cache
   bakes, and `streamline` loses nothing visible. **Invariant-perfect:** the
   simplified points ARE the object — wet, dry, bake, and hit-testing all see
   identical data. Do NOT do commit-time-only simplification (that would make
   the final dry stroke differ from the wet preview the user just watched);
   filter at capture so wet and committed geometry are the same points.
   (Optional follow-up, separate decision: Ramer–Douglas–Peucker at capture end
   for further reduction — file separately if wanted; ε-spacing alone should
   fix the density pathology.)

## Committed reproducer (red-first)

A wet-frame pixel test: arm freehand at opacity 0.5, dispatch a dense synthetic
stroke (many close pointermoves — do NOT lift), then `getImageData` on the edit
canvas MID-STROKE: sample a pixel in a segment-join region vs a single-coat
region of the same stroke. Pre-fix: the join sample is measurably darker
(alpha-stacked); post-fix: uniform within tolerance. Second assertion: after
pointer-up, the committed render is uniform in BOTH worlds (guards that the fix
didn't touch dry). Mind the T34 lesson: shim `setPointerCapture` for synthetic
pointers.

## Acceptance criteria

- The reproducer fails pre-fix at the join sample and passes post-fix; the
  full editor suite stays green (wet-path perf spec `editor.spec`/inkPerf
  numbers must not regress meaningfully — cite before/after from the committed
  `[inkPerf]` console line).
- The capture filter: a slow dense synthetic stroke stores ≤ some sane fraction
  of the raw event count (assert points.length bound in the reproducer), and
  the committed stroke's pixels match the wet preview (same points).
- `tsc -b studio` clean; pixels reviewed at the gate (slow stroke at 50%,
  before/after).

## Out of scope

- Bézier/spline smoothing (ruled out above); changing perfect-freehand params;
  RDP simplification (separate decision); the dry/bake renderers (untouched by
  design); multi-OBJECT overlap darkening (two separate strokes overlapping is
  real-ink behavior, by design).

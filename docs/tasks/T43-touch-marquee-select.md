# T43 — One-finger marquee-select in Select mode on touch

**Priority:** normal (VLL 2026-07-13, studio in an Android browser) · **Size:** S ·
**Area:** `web/studio` WetCanvas (touch grammar) + e2e. **Ruling:** T27 author, option (b) scoped.

## Decision (RULED)

In **Select mode ONLY**, a one-finger drag on EMPTY space draws a marquee (rubber-band
select), matching the desktop mouse grammar — instead of panning. This does NOT reverse
T27's touch principle: **two fingers ALWAYS navigate** (pinch/pan-zoom) is unchanged, and
that already makes one-finger-pan in Select mode redundant, so no navigation is lost.

Unchanged: one finger ON an annotation MOVES it; two fingers pan/zoom; the DRAW tools'
one-finger pen/finger split (T27 stage 4); the T34 stuck-nav heal (two-finger path).

## Changes

- WetCanvas `onPointerDown` (~`:116-118`): the `tool === "select"` branch that currently
  sets `doPan = !pick.object && !inMulti` (one-finger pan on empty) instead BEGINS a
  marquee gesture on empty space for a touch pointer — the SAME marquee the mouse path
  starts (reuse `g.mode === "marquee"`, the `normalizeRect`/`intersectsRect` selection on
  pointer-up). A one-finger drag on an object still starts a move; two-finger still cancels
  to nav. Mouse behavior is untouched.

## Acceptance criteria

- e2e (touch): in Select mode, a one-finger drag across empty space over two objects
  SELECTS both (marquee), does NOT scroll the page; a one-finger drag on an object MOVES
  it (no regression); a two-finger pinch still zooms (one raster — the T27 stage-4 spec
  stays green). Red-first where the assertion is new. Synthetic-pointer capture shim per
  T34. Full editor + touch suites green; `tsc -b studio` clean.

## Out of scope

- Changing the DRAW-tool touch grammar or the two-finger nav; a marquee on mouse
  (already exists); any desktop behavior change.

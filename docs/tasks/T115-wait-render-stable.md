# T115 — `waitRenderStable`: replace fixed settle-sleeps in the exact-count raster tests

**Priority:** after the deflake sweep (`a35adee`) · **Size:** S · **Area:** `web/studio/e2e`

## Context

The deflake sweep converted 9 of 48 `waitForTimeout` sleeps to real awaits and **kept 39**, documented
by category. The largest single kept cluster was **category A — exact-count quiescence (10 sleeps)**: the
editor tests that assert a wheel/pinch zoom re-rasters *exactly once*
(`renderCount - before === pageCount`, never once-per-tick). Those slept a fixed amount to let
rasterization settle before reading the `pdf-render-count` probe.

A fixed settle-sleep here is a real (if latent) flake: too short under CI load → the count is read
mid-raster and the exact-count assertion fails spuriously. But it cannot be converted carelessly — a poll
that returns **mid-climb** would let a per-tick regression read `pageCount` on its way to `N·pageCount`
and pass, silently gutting ten assertions at once. This task adds one shared primitive that provably
cannot return early, and converts the 7 of those 10 where a render is actually expected.

## Scope — 7 of the 10, not 10

Three of the "A" sleeps are **negative assertions in disguise** and stay as sleeps (you cannot poll for
nothing happening): `editor-wheelzoom` L118 (plain wheel → no zoom/raster), L164 (overlay edit → no
re-raster), `editor-noflicker` L101 (the no-flicker guard: a move must not re-raster).

Converted (7): `editor-wheelzoom` ×4 (two settle-baselines + two burst waits), `editor-touch` ×2 (settle
+ pinch), `editor-noflicker` ×1 (settle baseline).

## The primitive — `web/studio/e2e/render-helpers.ts`

`waitRenderStable(page, since, { holdMs=200, samples=3 })` — wait until rasterization has quiesced after
an action expected to render, then return the settled count. Two guards make an early return impossible:

1. **A pass has landed** — `renderCount > since`; never returns at the pre-action baseline (which would
   race an unstarted raster: debounce + raster both still pending).
2. **It then holds steady** — `samples` **consecutive** equal reads spaced `holdMs` apart, i.e.
   `(samples-1)·holdMs = 400ms` of confirmed steadiness. (The original spec's poll held for only one
   window; this requires N consecutive equal samples so the code matches the "400ms" description.)

The caller keeps its own `=== pageCount` assertion; the helper only replaces the blind sleep.

## HOLD sizing — measured, not guessed

`WHEEL_SETTLE_MS = 120` (`usePdfDocument.ts:41`) is the debounce; but the count increments when a pass
**lands**, so HOLD must clear debounce + raster completion. Instrumented the gap between consecutive
`pdf-render-count` increments (in-page MutationObserver, timestamped) under the heaviest zoom in these
tests:

- 8-tick Ctrl+wheel burst from 100%: **2 increments, 37ms apart** (one per page of the 2-page fixture).
- `selectOption(300%)` (4096-px-side clamp) and fit-page: **both pages in a single batched React commit,
  0 intra-pass gap.**

Worst observed intra-pass gap **37ms**. `holdMs = 200` is >5× that and >1.6× the 120ms debounce; the
400ms steady window cannot land between a regression's extra passes. Reported, not assumed.

## Teeth-check — recorded pass→mutate→RED→revert→pass

Per exact-count test (`editor-wheelzoom`, `editor-touch`):

- **Inject a genuine spaced 2nd pass** (a second small zoom after the first settles → true delta
  `2·pageCount`). Result: **RED — Expected 2, Received 4.** `waitRenderStable` returned the *fully
  settled* count (4), not an early partial (2/3) → proves no early return **and** that `=== pageCount`
  reddens on extra rasters. Reverted → green.
- **Revert the impl to per-tick raster** (`commitWheelZoom()` on each tick instead of the debounced
  settle): **did NOT reproduce** — `ctrlWheelBurst` dispatches all ticks synchronously and
  `commitWheelZoom` rasters via `setZoomMode`, which React batches, so per-tick collapses to one raster.
  This is a pre-existing property of the synchronous-dispatch test, not a teeth loss from the conversion;
  recorded honestly rather than presented as a RED. The spaced-2nd-pass mutation above is the valid,
  sufficient teeth-check.

## Acceptance criteria

- The 7 sleeps replaced by `waitRenderStable`; the 3 negatives left as sleeps.
- Full `make e2e` green; `--repeat-each=4` on the 3 converted files green (flake-surface evidence).
- The two exact-count tests redden under the spaced-2nd-pass mutation and pass on revert (above).
- e2e-only: no `src/`, no Go.

## Out of scope

- The 3 negative-assertion sleeps in these files, and categories B/C/D/E from the sweep — a sleep is the
  correct primitive for "nothing happened" and for elapsed-time properties.

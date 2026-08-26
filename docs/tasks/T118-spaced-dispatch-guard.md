# T118 — Spaced-dispatch guard for the "not once per tick" half of the wheel exactly-once claim

**Priority:** after T115 · **Size:** XS · **Area:** `web/studio/e2e`

## Context

`editor-wheelzoom`'s exactly-once test asserts a Ctrl+wheel burst *"re-rasters exactly once, **not once
per tick**"*. During T115 we found — and Fable independently confirmed on `origin/main` before the T115
conversion — that the **second half of that claim is unguarded**: `ctrlWheelBurst` dispatches all ticks
**synchronously** in one task, so React batches the per-tick `setZoomMode` calls and a per-tick raster
regression collapses to a single raster. Applying that regression (`commitWheelZoom()` per tick instead
of the debounced settle) leaves the existing test **green** — it can't tell the two apart.

This adds one small guard whose input *can* distinguish them. Fable declined to queue it as part of
T115 ("its own guard, not a rider"); VLL asked for it.

## Change

One new test in `editor-wheelzoom.spec.ts` + a local `ctrlWheelSpaced` helper. The helper schedules N
Ctrl+wheel ticks in **separate tasks** via in-page `setTimeout(i·gap)` — timing driven by the **page
clock** (deterministic), not per-`evaluate` CDP round-trips (which vary with load). Ticks are **60ms
apart**, inside the 120ms `WHEEL_SETTLE_MS` window, so:

- **Correct (debounced) impl:** each tick resets the settle timer → exactly ONE committed pass →
  `delta === pageCount`. Green.
- **Per-tick regression:** each tick, in its own task, rasters separately → `delta > pageCount`. Red.

No fixed settle sleep is added: the caller's `waitRenderStable` (T115) blocks until a raster lands,
which is strictly after the last scheduled tick. Net `waitForTimeout` count unchanged (32).

## Why gap = 60ms (empirically tuned)

The raster count increments only when a pass **completes uncancelled** (`usePdfDocument.ts:342`), and a
new zoom cancels the in-flight raster — so the teeth need `raster-time < gap < debounce`. Swept
gaps on both impls (deterministic page-clock timing):

| gap | correct impl delta | per-tick regression delta |
|----:|:------------------:|:-------------------------:|
| 50ms | 2 (=pageCount) | 6 |
| 70ms | 2 | 8 |
| 90ms | 2 | 8 |
| 110ms | 2 | 9 |

Every gap 50–110ms distinguishes them. **60ms** is chosen for the correct-impl safety margin: it's 2×
under the 120ms debounce, so `setTimeout` drift under CI load can't push adjacent ticks past the debounce
and false-red the correct impl — while still well past the ~37ms per-page raster time, so the regression
reddens.

## Teeth-check — recorded

- **Correct impl:** `--repeat-each=3` → 3 passed, then `--repeat-each=2` after removing the helper's
  redundant settle sleep → 2 passed. No drift flakiness.
- **Per-tick regression** (`commitWheelZoom()` per tick): **RED — Expected 2, Received 4/6.** The
  existing synchronous exactly-once test stays GREEN under the identical mutation; this one reddens.
  That contrast is the whole point.

## Acceptance criteria

- New test green in full `make e2e`; reddens under the per-tick mutation (above).
- No net `waitForTimeout` added; e2e-only, no `src/`.

## Out of scope

- Any change to the existing exactly-once test or to `waitRenderStable`; the app raster/debounce path.

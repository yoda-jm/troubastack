# T13 — e2e: RO-vs-RW footprint shifts ~27px in CI headless

**Priority:** after T02 (unblocks re-hard-gating e2e) · **Size:** S/M · **Area:** `web/studio`

## Context

CI (T02) runs the Playwright suite. 55/56 pass; the one failure is
`web/studio/e2e/editor-rorw-shift.spec.ts:122` — *"focusing a read-only layer
does NOT shift the layout (RO vs RW footprint identical)"*. It asserts:

```
expect(ro.toolbarH).toBe(rw.toolbarH);   // passes
expect(ro.pageTop).toBe(rw.pageTop);     // FAILS in CI: 1403.625 vs 1376.84375
```

- **Green locally** (headed/local Chromium, 56/56), **fails only in CI headless**.
- The delta is ~26.8px — far too large to be sub-pixel rounding, so the exact
  `toBe` is not the root cause; something between the toolbar and the page
  raster genuinely changes height between the RO and RW states in CI.
- `toolbarH` is identical, so it is NOT the toolbar itself — look at what sits
  below it (style/layer bar, "Edit this layer" banner, reserved-space element
  introduced by 772be41 "keep toolbar/viewer footprint stable for RO vs RW").
- Prime suspect: a web font / icon asset that loads locally but not in CI,
  changing a line-box or control height in one state only. Confirm what font/
  asset the editor chrome depends on and whether it is bundled vs system.

Because this is a real behavior/environment issue and out of T02's scope, the
single spec is quarantined at the spec level — `test.skip(!!process.env.CI, …)`
in `editor-rorw-shift.spec.ts` — so the rest of the e2e suite still HARD-GATES
in CI (the job is NOT `continue-on-error`). It runs normally locally.

## Changes

1. Reproduce the CI failure locally (e.g. run the spec against headless Chromium
   with web fonts disabled, or replicate the CI font environment) to confirm the
   cause of the ~27px shift.
2. Fix the root cause so the RO and RW states have an identical footprint
   regardless of font availability (the invariant 772be41 was meant to hold).
   If the true footprint legitimately differs, fix the layout, not the test.
3. Remove the `test.skip(!!process.env.CI, …)` quarantine from
   `editor-rorw-shift.spec.ts` so the spec runs in CI again.

## Acceptance criteria

- `make e2e` is 56/56 in CI headless with the quarantine removed (the e2e job
  already hard-gates; only this spec is skipped in CI today).
- `editor-rorw-shift.spec.ts` still asserts an identical RO/RW footprint (not
  loosened to hide the shift).

## Out of scope

- Broader editor-chrome redesign (see T05).

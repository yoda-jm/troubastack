# A13 — Stage: volume keys must turn by spread in two-up (A12 defect)

**Priority:** bug fix, first of the reading-ergonomics batch · **Size:** XS/S ·
**Area:** `app/shared` (stage) + `app/androidApp` (MainActivity wiring)

## Context

Confirmed defect (arch-verified 2026-07-07, raised in
`docs/handoff/proposals/stage-reading-ergonomics.md` §1): every navigation input —
keys/pedals (A09), taps, swipes, pager buttons — routes through the spread-aware
`turnNext`/`turnPrev` (`StageScreen.kt:149`), EXCEPT Android volume keys, which
`MainActivity.kt:149` wires straight to `vm.next()/vm.previous()` (turn-by-1). In
landscape two-up the first volume press is a visual no-op (`spreadFor(1)==0` — same
spread), so volume needs two presses per spread while a pedal needs one.

**Design decisions (resolved):**
1. The registration moves to where `twoUp` is known: StageScreen publishes its
   `turnNext`/`turnPrev` via a **commonMain CompositionLocal**
   (`LocalVolumeTurnRegistrar: (((PageTurn) -> Unit)?) -> Unit`, default no-op) in a
   `DisposableEffect` keyed on the turn lambdas; androidApp provides it wrapping the
   existing `activity.stageVolumeTurn` field. Remove the `App()`-level
   `vm.next/previous` volume wiring. No new I15 seam (a CompositionLocal is Compose
   plumbing, not an expect/actual class); `PageTurn` is already commonMain; iOS never
   provides the local → no-op (it has no volume-key turn).
2. Keep A09's keep-screen-on/interception behavior untouched — this only changes what
   the intercepted press calls.

## Changes

1. commonMain: the CompositionLocal + StageScreen registration (`DisposableEffect`
   keyed on `twoUp`/`pageCount` republished when they change; unregister on dispose).
2. androidApp: provide the local around the app content; delete the direct wiring at
   `MainActivity.kt:149`.
3. Test: commonTest for the registrar contract if practical; otherwise the spread math
   is already covered — assert by a targeted unit test that the registered lambda in
   two-up advances `current` by 2 (drive a fake registrar).

## Acceptance criteria

- In landscape two-up, ONE volume press advances ONE spread (parity with pedals); in
  portrait/FIT_WIDTH, one press = one page, exactly as today.
- Registration is cleaned up when Stage closes (volume keys revert to normal outside
  Stage — the A09 dispose behavior still holds).
- `:shared:check`, `:androidApp:assembleDebug`, iOS klibs green. Emulator evidence:
  two-up screenshot pair (before/after one volume press) or a note that it was driven
  by `adb shell input keyevent KEYCODE_VOLUME_DOWN` with pager label 1–2 → 3–4.

## Out of scope

- iOS volume behavior; remapping other keys; any VM API change.

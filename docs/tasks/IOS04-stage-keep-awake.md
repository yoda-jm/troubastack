# IOS04 — iOS Stage host: keep the screen awake during a performance

**Priority:** iOS-track, anytime (unblocked) · **Size:** XS · **Area:** `app/shared` (iosMain)

## Context

Filed from the IOS03 prep-plan finding: Android's `StageHost` sets
`FLAG_KEEP_SCREEN_ON`, but the iOS entrypoint (`MainViewController.kt`, iosMain) never
touches `UIApplication.sharedApplication.idleTimerDisabled` — so an iPad on a music
stand sleeps mid-song. This is a Stage-contract parity gap (I13's performance
resilience is the whole point of Stage), and it is buildable/verifiable **today**,
unlike the rest of IOS03.

## Changes

1. In the iOS entrypoint, set `UIApplication.sharedApplication.idleTimerDisabled = true`
   while a Stage is open, and restore `false` when leaving Stage (back to Concerts) —
   mirror the scoping Android uses (flag held only while performing, not app-wide).
   The natural seam: where `selectedDir` transitions in `App()` (MainViewController.kt).
2. No new expect/actual seams (I15): this is entrypoint code in iosMain, same as the
   marker/AUTOPEN glue.

## Acceptance criteria

- `./gradlew :shared:compileKotlinIosArm64 :shared:compileKotlinIosSimulatorArm64`
  green on Linux (the change is iosMain-only; behavior can't be asserted headlessly).
- Code review confirms the flag is scoped to Stage (set on open, cleared on exit) —
  not set app-wide at launch.
- Android completely unaffected (`:shared:check` green; no commonMain change needed —
  if one is unavoidable, keep it a no-op for Android).

## Out of scope

- Everything else in the IOS03 runbook (signing, TestFlight, device QA — still blocked).
- Guided Access, brightness, battery (device-QA checklist items, not code).

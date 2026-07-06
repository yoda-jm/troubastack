# A09 — Stage: hardware page turns (Bluetooth pedals / volume keys)

**Priority:** A-track, unblocked · **Size:** S · **Area:** `app/shared` (stage), entrypoints

## Context

A stand-mounted tablet is performed with both hands on an instrument. The standard rig
is a Bluetooth page-turner pedal (AirTurn, PageFlip, …) — which presents as a keyboard
sending PageUp/PageDown (or arrow) key events. Stage currently only turns pages by tap;
it ignores hardware keys entirely. Volume keys are the poor-man's pedal on phones.

**Design decisions (resolved):**
1. Key map, fixed (no settings UI in v1): `PageDown` / `ArrowRight` / `ArrowDown` /
   `Space` / `VolumeDown` → next page; `PageUp` / `ArrowLeft` / `ArrowUp` /
   `VolumeUp` → previous page. Same clamped navigation the on-screen pager uses (no
   wraparound; silently no-op at the ends).
2. Volume-key capture **only while Stage is open** (Android: intercept in the Stage
   host only — leaving Stage restores normal volume behavior). This mirrors the
   keep-awake scoping (IOS04).
3. Shared where possible: the key→action mapping is a pure common function
   (`stageKeyAction(keyCode): PageTurn?`) with unit tests; the event capture itself is
   platform glue in the entrypoints (Compose `onKeyEvent`/`onPreviewKeyEvent` covers
   external keyboards on both platforms; Android volume keys need the Activity
   `onKeyDown` override in `androidApp` — entrypoint code, not a new seam, I15).

## Changes

1. Common: the mapping function + wire it into `StageScreen`'s existing
   next/previous-page actions via a focusable modifier (`onPreviewKeyEvent`).
2. `androidApp` MainActivity: `onKeyDown` for VOLUME_UP/DOWN forwarded to the Stage
   action ONLY when a Stage is open (return false otherwise so volume works normally).
3. iOS: external-keyboard events arrive through Compose's key handling in the shared
   code — verify by code review + klib compile (no simulator proof needed; pedals are
   keyboards).
4. commonTest: the mapping matrix (all mapped keys both directions, unmapped → null).

## Acceptance criteria

- Emulator evidence: `adb shell input keyevent KEYCODE_PAGE_DOWN` (and
  `KEYCODE_VOLUME_DOWN`) advances the Stage page; volume keys act normally outside
  Stage (screenshot pair or recording).
- Mapping unit tests green; `:shared:check` + iOS klib compiles + `assembleDebug` green.
- No wraparound; keys are inert on the Concerts list.

## Out of scope

- Configurable key mapping; MIDI pedals; long-press behaviors; iOS volume-button
  capture (not sanctioned by iOS APIs — external keyboards only there).

# A15 — Stage: song-jump navigation drawer

**Priority:** reading-ergonomics batch, with/after A13 · **Size:** S ·
**Area:** `app/shared` (stage)

## Context

Proposal §4 (legacy Concert Mode had a side drawer): today's "Songs" dropdown
(`StageScreen.kt:241`, `vm.goToSong`) already jumps anywhere, but a dropdown is
cramped for a mid-set encore decision. Promote it to a proper drawer.

**Design decisions (resolved):**
1. `ModalNavigationDrawer` (or `ModalDrawerSheet`) listing the concert's songs in
   order: title + the A08-style meta line (key/tempo if present), **current song
   highlighted**, big touch targets (stage-friendly). Opened from the existing Songs
   affordance; tap → `goToSong` + close. Scrim closes it.
2. Read-only (I12) — display + navigation only, no model change, no bake change.
3. Works in every mode (single, two-up, and A14 scroll when it lands — it just calls
   the same jump).

## Changes

1. `StageScreen`: replace the dropdown menu with the drawer (keep the trigger where it
   is); current-song derivation from the page → song mapping already used by A08.
2. commonTest: current-song-for-page derivation (pure), if not already covered by A08
   tests.

## Acceptance criteria

- Drawer lists all demo songs with the current one highlighted; tapping another song
  lands on its first page (spread-aligned in two-up, per A12's song-jump rule);
  back/scrim closes without navigation.
- `:shared:check`, `:androidApp:assembleDebug`, iOS klibs green; emulator screenshot
  (drawer open, mid-concert, highlight visible).

## Out of scope

- Encore/bench songs outside the setlist order (that's T23 — core-side); search;
  reordering from the drawer; per-song thumbnails.

# A14 — Stage: continuous-scroll reading mode

**Priority:** reading-ergonomics batch, after A13/A15 · **Size:** M ·
**Area:** `app/shared` (stage)

## Context

Legacy-app feature VLL wants back (proposal §3): a continuous vertical column of all
pages ("Scroll"), alongside today's discrete FIT_PAGE turns and single-page FIT_WIDTH.

**Design decisions (resolved, arch 2026-07-07):**
1. **A third fit/reading mode** on the existing toggle: Fit page · Fit width ·
   **Scroll** (a `LazyColumn` of all concert pages at fit-width). Cycle order:
   page → width → scroll → page.
2. **Scroll wins over two-up:** continuous scroll is a single column; in scroll mode
   the A12 landscape two-up branch is simply not taken (mutually exclusive by
   construction — no toggle interplay).
3. **A09/A13 turns in scroll mode:** pedal/key/volume "next" animates to the top of
   the next page (`animateScrollToItem`), "prev" to the previous page top. The pager
   label shows the topmost visible page.
4. **Persistence: global** Stage preference, exactly the A10 pattern (Storage KV via
   entrypoint DI, no new seam). NOT per-file (the legacy `fileId_memberId` keying is
   more model than we need; revisit only if VLL asks).
5. A08's metadata strip renders inline above each song's first page in the column
   (it scrolls with content — no floating overlay logic). A11's count-in chip
   unaffected. Night mode (A10) applies per-page exactly as today (same `PageView`).
6. Song jump (`goToSong`) scrolls to that song's first page top.
7. Decode pressure: the existing LRU async cache serves `LazyColumn` items; keep the
   cache size as-is and let laziness do the work (visible + prefetch neighbors).

## Changes

1. `StageScreen`: the scroll-mode branch (LazyColumn of `PageView`s + inline metadata
   strips); mode in the ViewModel state + persisted like A10's color mode; turn
   handlers switch to scroll-by-page in this mode; pager label from first visible item.
2. commonTest: mode cycle + persistence round-trip; "next from page N lands on N+1
   top" logic (pure part); label derivation.

## Acceptance criteria

- Scroll mode scrolls continuously through the whole demo concert; page turns
  (buttons, keys, volume) move exactly one page; song jump lands correctly; mode
  survives app restart; portrait/landscape both single-column in scroll mode.
- Existing FIT_PAGE/two-up behavior byte-identical when not in scroll mode.
- `:shared:check`, `:androidApp:assembleDebug`, iOS klibs green; emulator screenshot
  (mid-scroll showing a page boundary + the strip inline).

## Out of scope

- Per-file/per-song mode memory; horizontal continuous scroll; half-page turns;
  scroll-position persistence.

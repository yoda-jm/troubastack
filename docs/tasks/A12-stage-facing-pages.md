# A12 — Stage: facing pages (two-up) in landscape

**Priority:** A-track · **Size:** S/M · **Area:** `app/shared` (stage)

## Context

On a 11–13" tablet in landscape, every serious sheet-music app shows **two facing
pages** — one-page-in-landscape wastes half the glass and doubles page turns. Stage is
single-page always (USER-JOURNEY gap #6).

**Design decisions (resolved):**
1. **Automatic, not a mode:** when the Stage viewport is landscape (w > h) AND the
   current fit is `FIT_PAGE`, render two adjacent pages side by side (pages 2k/2k+1 of
   the concert — spread alignment by global page index, keeping it dead simple; no
   book-parity logic in v1). Portrait or `FIT_WIDTH` ⇒ single page, exactly as today.
2. Page turns move by **2** in two-up (pager label "3–4/22"); the last odd page shows
   alone, centered. Song jumps land on the page-pair containing the song's first page.
3. Both pages composite their overlays exactly as today (same `PageView` per side —
   reuse, don't fork); decode both via the existing async cache (the LRU's 12 entries
   comfortably hold two spreads).
4. A08's metadata strip renders when EITHER visible page is a song's first page
   (anchored to that side's top). A11's count-in unaffected.

## Changes

1. `StageScreen`: a two-up layout branch (Row of two `PageView`s) + pager math
   (`spreadFor(page)`, turn-by-2, label) as pure tested functions; state keeps a
   single "current page" as the source of truth (the left page of the spread).
2. commonTest: spread math matrix (odd/even totals, last-page-alone, song-jump
   landing, turn clamping).

## Acceptance criteria

- Emulator (the Pixel_Tablet_Portrait AVD rotated, or a landscape capture): landscape
  FIT_PAGE shows pages 1–2 of the demo side by side with overlays; portrait unchanged
  vs. current screenshots; turn advances by spread; page 21 (Open Road, second-to-last
  of 22) pairs with 22. Screenshot evidence.
- Spread-math tests green; `:shared:check` + iOS klibs + `assembleDebug` green.

## Out of scope

- Book parity (cover-alone/odd-even spreads); half-page turns; per-song spread reset;
  manual two-up toggle in portrait.

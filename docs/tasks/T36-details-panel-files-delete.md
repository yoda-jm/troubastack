# T36 — Files management + Delete song reachable in the editor (the rest of the clipped-Details gap)

**Priority:** HIGH (live blocker — VLL 2026-07-12: "we cannot add a pdf or a
typing text file, we cannot delete a song") · **Size:** S/M · **Area:**
`web/studio` (Viewer Details panel + SongEditor cleanup) + guard e2e.

## Context

The T27 full-bleed editor clips SongEditor's legacy `<Details>` section
off-screen. The metadata half was fixed 2026-07-11 (`5a457a3`) by wiring
`<Metadata>` into the Viewer's Details panel. Still unreachable to humans:
`<Files>` (shared-pool PDF/image **upload**, **＋ New text chart**,
rename/reorder/delete — `SongDetails.tsx:281/:273`) and `<DeleteSong>`
(admin, `:512`). The only reachable file UI (`MyFilesEditor`) is per-member
*selection* — it cannot add, create, or delete. So composing a new song's
content inside the editor is impossible. Same
Playwright-reachable/human-unreachable class as the metadata bug.

## Design (RULED — don't improvise)

1. **Placement: the Details panel.** The pill is labeled "Song details & files";
   you edit the song where you are. (Rejected: a band-page song-settings surface
   — fragments the mental model; restoring page scroll under the full-bleed
   editor — re-opens the zero-shift layout problem T27 closed.)
2. **Panel structure, top to bottom:** `<Metadata>` (as landed) → **Files**
   (the existing `<Files>` component: upload, ＋ New text chart, rename /
   reorder / delete file) → **My files** (`<MyFilesEditor>`, as landed) →
   **Danger zone**: `<DeleteSong>` LAST and least prominent (its existing
   confirm stays; admin-only visibility as today). On successful deletion,
   navigate to the band page (the song no longer exists).
3. **The panel must scroll:** it now holds four sections — give it
   `max-height` + `overflow-y: auto` so grown content stays reachable at any
   viewport (including <600px, where it's a sheet). Reachability is THE bug
   class here — gate it (below).
4. **Kill the dead copy:** delete the clipped `<Details>` section (and its
   now-duplicate children) from `SongEditor.tsx` entirely. A clipped copy that
   specs can silently target is how this class survives — remove the substrate.
   Spec mechanics that referenced the clipped copies get the open-the-panel
   mechanic (assertion freeze applies: `expect()` lines stay).

## Guard e2e (red-first, the metadata precedent)

Scoped to the panel (`details-panel` testid), so no other copy can false-pass:
- upload a PDF via the panel → file appears in the parts bar;
- ＋ New text chart via the panel → chart editor opens (T19 flow);
- delete-song via the panel (confirm) → lands on the band page, song gone;
- reachability probe on the LAST section (Danger zone): `elementFromPoint` at
  the delete button's center resolves to the button after scrolling the PANEL
  (not the page) — kills the clip/occlusion class for the panel's tail.
Prove all red on pre-fix main (the actions are unreachable), then green.

## Acceptance criteria

- Red-first proof; full editor + flows suites green (mechanic edits listed);
  `tsc -b studio` clean; pixels at the gate (panel with all four sections, both
  themes, desktop + 390px).

## Out of scope

- T37 (lyrics import) — separate task; any new file-management features; the
  rejected placements above.

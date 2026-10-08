# T187 — Fresh Studio screenshots for the project page

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-10-08: the GitHub Pages site is
being redesigned (Fable is building it) and should show *"something closer to the current status"*. Its
screenshots predate tags, search, line styles, the note viewer and the current editor chrome.

Sibling: **A80** (the Stage screenshots, mobile lane).

## Hard rules

- **Synthetic demo data only**: the seeded cast (Marie / Leo / Sasha, *The Troubadours*, the orchestra) and
  their songs (The Open Road, House of the Rising Sun, Amazing Grace, Greensleeves, the two classical pieces).
  **No real band, member, concert or song may appear anywhere in a frame**, not in a list, a tab, a toast or a
  browser title. These go on a public website.
- Run an **isolated** seeded server (a scratch port and data dir, `go run ./cmd/seed`). **Never :8080.**
- Clean frames: no dev tooltips, no error banners, no cursor, no half-finished dialogs (unless the dialog is
  the subject).

## Format

- Desktop **1440×900 at deviceScaleFactor 2**; phone **412×915 at DPR 2** where noted. PNG.
- **Light theme**, plus a dark-theme twin where noted, because the page will show the theme switch.
- Files go to `docs/screenshots/site-<id>.png` (dark twin `site-<id>-dark.png`). Commit them, then post the
  list at the gate with a one-line description each. **Fable reviews every frame before it ships** (publish
  class: the site is world-readable).

## The shots

| id | What it shows | Notes |
|---|---|---|
| `s1-editor` | The editor on **The Open Road**: a few annotations of different kinds, at least one **arrow with a styled end** (T177/T179), a highlight, a text mark; the layer drawer open on the right with two members' layers. | desktop + **dark** twin. The hero shot: make it beautiful, not busy. |
| `s2-search` | The band song list with **tag pills** on the rows (C1), the search box holding a **chip** plus a typed word, and the **suggestion list open** with counts. | desktop |
| `s3-tags` | The **Tags panel** (T182) with the **merge confirmation** open (a real merge, e.g. fold a variant into `folk`). | desktop |
| `s4-note-viewer` | The **rehearsal-note viewer** (T186) over The Open Road, with a handwritten-looking note (draw one with a stylus or a mouse and upload it through the API as the e2e does). The info line shows the file name. | desktop |
| `s5-chart` | The **text-chart editor**: chord-over-lyric source on one side, the typeset page on the other. | desktop |
| `s6-setlist` | A **setlist** with per-member cues and the **intermission** row, mid-drag if you can catch it cleanly. | desktop |
| `s7-phone` | The editor on a **phone** (annotating The Open Road). | phone |

## Not in this task

Any UI change. If a screen looks wrong in a frame, file it; do not fix it in this task.

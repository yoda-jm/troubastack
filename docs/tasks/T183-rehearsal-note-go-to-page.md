# T183 — Rehearsal notes list: "Go to" the note's page

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-09-30: *"in studio in
rehearsal notes details, maybe a 'go to' to the note page could be nice, especially if there is a lot of
pages"*.

## 1. What exists

- `RehearsalNotesChip` (`song-editor/RehearsalNotes.tsx`): the **⋯** button opens a popover with one row per
  note (*page N · from rev … · taken as … · drawn/sent …*) and a **Done, remove** button. Nothing on the row
  takes you to the page, so on a long chart you scroll and count.
- The viewer renders every page in one scroll column. `Viewer.tsx` already has `scrollObjectIntoView`,
  which centres page N's `.pdf-page` wrapper by scrolling the column itself (it also works in the embedded
  webview). This task only needs the page part of it.
- The underlay is placed with `noteForPage(rehearsalNotes, i)`, where `i` is the page index **of the open
  file**. The note's `pageInSong` is *"0-based, in the bake it was drawn on"* (`app.go`).

## 2. ⟨D1⟩ Each row gets "Go to page N"

A button on each popover row, beside *Done, remove*. Clicking it:

1. **turns the underlay on** if it is off, because going to a page to see a note you have hidden is
   pointless;
2. scrolls the viewer so that page is centred. **Extract a `scrollPageIntoView(page)`** and have
   `scrollObjectIntoView` call it. Do not write a second copy of the scroll arithmetic;
3. closes the popover, which would otherwise sit on top of the page you asked to see.

The label is **"Go to page N"** (N 1-based, as the row already shows it). It is a new string, authorised
here.

## 3. ⟨D2⟩ When the page is not there, say so

If `pageInSong` is outside the open file's pages (`≥ numPages`, or `> 0` on a single image), the underlay
cannot be showing that note either. It is drawn on no page today. The button is then **disabled**, with a
title that states the fact: *"This file has no page N"*.

Do **not** guess a page, and do not switch files. The row cannot know which file a bake page came from:
the note carries only `pageInSong`. If VLL runs into this, it becomes its own task.

## 4. Not in this task

- Next/previous-note navigation, a page badge in a margin, or a highlight flash on arrival. Add them only if
  "Go to" turns out not to be enough.
- Any change to what *Done, remove* does, or to the chip.

## 5. Acceptance

- A PDF with **at least 6 pages** and a note on the **last** page. **The fixture must actually be long
  enough that the note's page starts outside the viewport**; assert that before clicking, or a test on a short
  chart passes without scrolling at all. After *Go to page N*, that page's `.pdf-page` is in the viewport and
  its `rehearsal-underlay` is visible.
- With the underlay off, *Go to* turns it on.
- The popover is closed afterwards.
- A note whose page is beyond the open file's page count: the button is disabled, with the title text above.
- `scrollObjectIntoView` still works (the existing tests stay green) and calls the extracted helper.

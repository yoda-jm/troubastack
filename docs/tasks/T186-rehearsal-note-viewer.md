# T186 — Look at a rehearsal note: a viewer that shows it on the page it was drawn on

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-10-07, relayed by web-core:
*"a popup viewer of the rehearsal note — useful in general, but important for non-matching."*

## 1. The gap

A note is a stroke layer, about 99.9 % transparent. On its own it is marks floating on nothing. Today it can
only be seen as an underlay on a page of the open file (T170), which fails in two cases:
- **Unresolved** (T185 ⟨D2⟩ "absent": the file changed, the rev was purged, or the renderer changed). There is
  no page to underlay it on, so the member cannot see what they drew at all. That is the case VLL calls
  important.
- **Resolved onto another file.** Seeing it is a detour: switch file, then scroll (T185 "Go to").

## 2. ⟨D1⟩ Show the note on the page it was drawn on, not on today's page

**The server still holds that page.** Every bake rev keeps its page rasters (`bakes/<concert>/<rev>/blobs/…`),
and the note carries the exact `rasterHash` of the page it was drawn over. So the viewer composites:

1. **background: the bake page whose `rasterHash` equals the note's**, looked up in the note's own rev, else
   in any rev of the same concert that has that hash (the T185 ⟨2⟩ walk). It is the same image whatever the
   rev;
2. **over it, the note PNG.**

This is the page **as it was when the musician wrote on it**, which is the context the note refers to, even
when the chart has changed since. For an unresolved note it is the only context that exists.

Add one endpoint, for example `GET …/rehearsal-notes/{page}/background`. It serves that raster to the note's
owner only, with the same authorisation as the note itself. The base raster carries no overlays, so no
member's private marks can leak through it. **If no rev still holds the hash, return 404**, and the viewer
shows the note on plain paper with the sentence in §3.

## 3. ⟨D2⟩ The viewer

- **Opens from** a **View** button on each row of the rehearsal-notes popover (T183), next to "Go to page N".
  Clicking the row's description also opens it. It is the obvious action on a note.
- **One note at a time, with ‹ › across the song's notes** (and ← → keys). A song rarely has many, and paging
  is cheaper than going back to the list.
- **The image:** fit to the dialog, page on white whatever the theme (it is paper). Note and page are drawn
  together at the same size.
- **The line above the image**, the same facts as the row, plus where it lives now:
  - resolved: *"bass · page 1 · from rev 12 · taken as … · sent 2 Oct"*;
  - unresolved, background found: *"Drawn on a page that is no longer in this song's files, shown as it was
    at rev 12."*;
  - no background: *"The page this note was drawn on is no longer available. Showing the strokes alone."*

  **Say it plainly; never imply a page number** for an unresolved note.
- **Actions in the viewer:** **Go to** when resolved (closes the viewer, then T185's file switch). **Done,
  remove** exactly as on the row, with the same effect and the same confirmation if the row has one. **Close**,
  also on Escape.
- A real dialog: `role="dialog"`, focus trapped, focus returned to the row on close.

## 4. Not in this task

- Zoom or pan inside the viewer. Fit is enough to read handwriting on a chart page. Add it if VLL asks.
- Showing other members' notes. Notes stay owner-only (T170).
- Any change on the tablet.

## 5. Acceptance

- The **unresolved case first**, because it is why VLL asked. Use a note whose rev no longer resolves (T185's
  failing-file fixture or a rev whose file changed) **but whose rev still holds the page**. The viewer shows
  the bake page plus the strokes, and the "no longer in this song's files" line. **Assert the background
  request returns the raster whose sha256 equals the note's `rasterHash`.** That is the property, not "an
  image loaded".
- A purged-rev note: background 404, strokes on paper, and the "no longer available" line.
- A resolved note: the file-named line, and Go to switches file.
- ‹ › walk the song's notes in `pageInSong` order. Escape closes and returns focus to the row.
- The background endpoint refuses a member who is not the note's owner. Assert it.
- **On :8080:** VLL's bass-tab note opens in the viewer over the bass page, as it was at rev 12. Report it
  without the song title.

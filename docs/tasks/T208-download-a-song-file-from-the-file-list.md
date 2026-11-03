# T208 — Download a song file from the file list, with or without its annotations

Owner: **web-core**. Spec: Fable, 2026-11-03, per VLL ("can I download a pdf of a file inside a song (annotated or
not), I haven't found it, could be nice to have this feature at the file level"). Size: S.

## What exists today, and why VLL didn't find it

Both halves already exist, but neither sits where a person looks for "download this file":

- **With annotations:** FILEPDF (`d31bc1dd`). `GET /api/bands/{b}/songs/{s}/files/{f}/pdf?layers=…` prints one
  file as an A4 PDF with the chosen layers drawn on it. Mandatory layers always print. The only button is
  `FilePdfButton` (`web/studio/src/pages/song-editor/FilePdfButton.tsx`). To reach it you must:
  1. open the file in the viewer;
  2. open the right-hand panel from the top bar;
  3. stay on its **Layers** tab.

  It is offered for `application/pdf` files only, which covers uploaded PDFs and text charts.
- **Without annotations:** the file name in each row of the song's file list (`SongDetails.tsx`, `file-download`) is
  a link to `GET /api/files/{id}?rev=…`. It opens the file **inline in a new tab**. Nothing says it is a download.

The row's **⋯ File actions** menu (`RowMenu`, `file-menu`) is where a person looks. It offers Rename, Move up,
Move down and Delete, and no download.

## The feature

Add one download entry to every file row's ⋯ menu.

**Placement:**
- The entry goes first in the menu, above Rename.
- **PDF and text-chart rows:** the entry is **"Download PDF…"**. This reuses the product word FILEPDF already uses;
  the ellipsis says that a dialog opens.
- **Image rows:** the entry is **"Download image"**, which downloads the file as stored, with no dialog. FILEPDF
  cannot print an image file today, so there is no annotated option for images; that is out of scope.

**The dialog** (title **"Download PDF"**, built like `PdfDialog`/`BakeDialog`) offers two choices:

1. **"With annotations"** (selected by default).
   - Below it, a checklist of **this file's** layers. This is the same set `visibleLayerIdsForFile` draws from:
     `l.fileId === fileId`. Song-level layers are not drawn over a file, so they are not listed.
   - The ticks start at what the viewer shows when it opens this file: `defaultVisibility(layers, myUserId)`
     (`useSongSync.ts`).
   - A mandatory layer is ticked, cannot be unticked, and is marked "always printed".
   - If the file has no layers of its own, this choice is disabled with the line "This file has no annotations.",
     and choice 2 is selected.
2. **"Without annotations"**: the file exactly as stored (an uploaded PDF untouched; a text chart as rendered).
   - This is the file the name link already serves. It exposes nothing new.
   - A crop set in Studio (T198) is **not** applied here. Choice 1 does apply it, as the tablet does.
   - The dialog says so in one line under this choice: "The file as uploaded: no annotations, no crop."

**The button and errors:**
- The dialog's button is **"Download"**. While FILEPDF composes, it shows **"Preparing PDF…"** and is disabled, as
  `FilePdfButton` does.
- A failure shows in the dialog through `ErrorBanner`, with the server's message, and the dialog stays open.
- On success the browser saves the file and the dialog closes.

**Both choices are real downloads,** never a new tab:
- With annotations: the name the server already sends (`<song title> - <file name>.pdf`, from `Content-Disposition`).
- Without annotations: the stored name with its extension, as `downloadFilename` gives it today.

**The request.** "With annotations" ALWAYS sends `layers` explicitly: the ticked ids, and an empty value when none
are ticked, which the server reads as "no optional layers". It must never omit the parameter. The server's
absent-parameter default is a different rule from the client's:
- the server, `fileLayerVisible` → `LayerVisible` with role `""`, drops a shared layer that carries a role tag;
- the client, `defaultVisibility`, shows it.

So omitting `layers` would silently print less than the dialog showed. Use the one `api.filePdf` call and the one
layer rule that `FilePdfButton` already uses. Do not write a second copy.

**Unchanged:**
- The viewer's Layers-tab "Download PDF" stays as it is.
- The file-name link keeps opening the file in a tab.

## Acceptance

1. **DOM test** (`web/studio/test/`). It must use a file whose layers include:
   - a shared layer **with a role tag** — the discriminating vector: `defaultVisibility` ticks it, the server default
     drops it;
   - a conductor layer;
   - the viewer's own personal layer;
   - another member's personal layer;
   - a mandatory layer;
   - a song-level layer with no `fileId`.

   Assert that:
   - the checklist lists exactly this file's layers, ticked as `defaultVisibility` says, with the mandatory one
     locked;
   - Download sends `layers=` with exactly the ticked ids, the role-tagged one included;
   - unticking everything sends `layers=` with an empty value, never no parameter.

   **Sabotage:** make the dialog omit `layers` when the ticks equal the defaults. The role-tagged assertion must go
   red. Report the red.
2. **Image row:** "Download image" is the entry, with no dialog. A file with no layers: "With annotations" is
   disabled, with its line.
3. **e2e** (`web/studio/e2e/`). Mock the FILEPDF endpoint with `page.route`, as `file-pdf.spec.ts` does, because
   the e2e core has no poppler. Then:
   - ⋯ → Download PDF… → With annotations → Download yields a `download` event with the server's file name, and the
     routed request carries the expected `layers`;
   - ⋯ → Download PDF… → Without annotations → Download yields a `download` event whose bytes equal
     `GET /api/files/{id}` for the current revision, and opens no new page.
4. **Docs:** in `docs/USER-JOURNEY.md`, Phase 1's "Validated today" bullet on the multi-file pool (which ends "PDF
   viewing in Studio."), add that any file downloads from its ⋯ menu, with your annotations or as uploaded.
5. **A PR run green on every job.** No app code changes, so the Windows rule does not apply. The run must still be
   green.

**Present** with: the DOM-test sabotage red, the e2e passing, the PR run's `/jobs`, and a screenshot of the menu and
the dialog in Studio's light and dark themes.

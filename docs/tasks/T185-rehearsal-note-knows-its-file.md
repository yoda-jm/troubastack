# T185 — A rehearsal note knows which file and page it was drawn on

**Lane:** web-core · **Status:** specced, not started · **Origin:** VLL, 2026-10-06: *"there is one annotation
page 3 and there is only 2 pages, I suspect an error when numbering on the tablet"*.

T183 ⟨D2⟩ named this case and deferred it: *"Which file a bake page came from … If VLL runs into this, it
becomes its own task."* He has run into it.

## 1. What happened, measured

The only rehearsal note on :8080 is on a song with **two files**: a lyrics chart (2 pages) and a bass tab
(1 page). VLL's own file selection is `[lyrics, bass]`, so his Stage sequence for that song is **lyrics p1,
lyrics p2, bass p1**. The note has `pageInSong = 2` and `rasterHash dba1d0a0…`, and that hash is **exactly**
pool page 2 of the concert's rev-12 bake. I opened that raster: it is the bass tab.

**The tablet is right.** `StageModel` sets `pageInSong` to the position in the viewer's **resolved sequence**
(`resolvePageSequence(song, identity)`), and "page 3" of his version is the bass page.

**Studio is what is wrong.** It places a note by `pageInSong` **on whichever file is open**
(`noteForPage(rehearsalNotes, i)`). The lyrics chart has no page 3, and neither has the 1-page bass tab, so
the note is drawn **nowhere** and T183's button reads *"This file has no page 3"*. The cause is not a bad
number. **No one records which file a bake page came from**: the bake builds `pagesByFile` and discards it,
and the bundle's pages carry only `pageRasterRef`, `rasterHash` and `contentBottomPermille`.

Any song with more than one file in a member's selection can produce this. It needs no bug on the tablet.

## 2. ⟨D1⟩ The bake records each page's source, server-side

At bake time, the baker knows every pool page's `(fileId, filePage)`: that is `pagesByFile`. **Persist it
next to the bundle**, for example as a sidecar beside `bundle.json` in `bakes/<concert>/<rev>/`, keyed by
`songId` and pool index, carrying `fileId`, `filePage` (0-based) and `rasterHash`.

**Server-side only. Do not add it to the bundle the tablet downloads.** The tablet does not need it, and
keeping it out means no contract change on Stage. The mechanism is yours. The property is: **for any bake rev
the server still holds, it can answer "which file and page produced the raster with this hash in this song".**

## 3. ⟨D2⟩ A note resolves to its file and page

A note already carries `concertId`, `concertRev`, `songId` and `rasterHash`. Resolve those through ⟨D1⟩ to
`fileId` + `filePage` and expose both on the note, for example as optional `fileId` and `filePage` fields.
Whether to resolve at upload and store, or at list time, is your call. Resolving at list time also covers
notes uploaded before this task.

- **Match on `rasterHash` within that song and rev, not on the index.** The hash is what identifies the
  drawn page. The index is a position in a sequence that differs per member.
- **Unresolvable** (the rev's bake is gone, no source record, or no hash match): leave both fields empty.
  Never guess.
- **Existing bakes have no source record** (VLL's note is on rev 12, baked before this task). The current
  note must still resolve. Re-deriving the map for an old rev, for example by re-rendering its files, is your
  design. If that proves unreasonable, say so at the gate rather than ship it unresolved.

## 4. ⟨D3⟩ Studio places, labels and goes to the right page

- **Underlay:** a resolved note is drawn on **page `filePage` of file `fileId`**, and only when that file is
  open. An unresolved note keeps today's rule (`pageInSong` on the open file), which is right whenever the
  song has a single file.
- **Popover row:** a resolved note says which file: **"bass · page 1"** (the file's name as Studio shows it
  elsewhere), not "page 3". The unresolved label stays "page N".
- **"Go to"** (T183): for a resolved note on **another** file, it **switches to that file**, then centres the
  page and turns the underlay on. T183 forbade switching only because the target was unknown. It is known now.
- **T183's "This file has no page N"** remains only for unresolved notes.

## 5. Not in this task

- Any change to the tablet or to the bundle format.
- Changing the note's unique key (owner, song, `pageInSong`).
- What happens to a note when the member changes their selection after drawing: the hash still identifies
  the page, so ⟨D2⟩ covers it.

## 6. Acceptance

- **VLL's case, as a fixture:** a song with a 2-page file and a 1-page file, a member selection that orders
  them `[2-page, 1-page]`, a bake, and a note on sequence position 2 with that page's hash. It resolves to the
  **1-page file, `filePage` 0**. **Assert the fixture is discriminating:** sequence index 2 must **not** equal
  `filePage`, or a broken resolver that just copies the index would pass.
- A **reversed** selection (`[1-page, 2-page]`), with a note on sequence position 0 that has the 1-page file's
  hash, resolves to the same file and page. That is the same page reached through a different index.
- Unresolvable (unknown rev, or a hash not in the song): fields empty, and the existing behaviour holds.
- Studio: a resolved note is drawn only on its own file and page, the row names the file, and "Go to" switches
  file. Pin it with an e2e on a two-file song.
- **On :8080 after deploy:** VLL's existing note shows on the bass tab and names it. Report it, **without
  writing the song title into the repo**.

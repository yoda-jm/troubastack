# A70 — Rehearsal notes on Stage: one bitmap per page, pencil and eraser, nothing else

**Lane:** mobile (Part A). Part B is core + studio + app, to be filed as its own T-task.
**Size:** L (Part A). **Filed:** 2026-09-10 by the architect from VLL's request; **revised the same day with
VLL's answers to twelve questions** (§1.2) — every decision below that quotes him is his, the rest are the
architect's and say so.

## Status: DISPATCHED to mobile, 2026-09-10 (VLL: *"ok dispatch it, mobile takes it after N10 and T149 land"*)

Takeable by the **mobile lane only after the in-flight N10 (swipe lock + finger-follow) and T149
(title clip) edits have landed on main** — same `StageScreen.kt` regions (§9). The I12 scope clause of
§3.1 is landed in `docs/ARCHITECTURE.md` in the same push as this dispatch, so the lane implements against
the amended invariant, not around it. Part A only; Part B (§7) is not dispatched and gets its own T-task.

## 1. What VLL asked for

### 1.1 The request, clause by clause

*"I know we said stage should just be a stage, but during rehearsal I need to take notes (no internet), my
idea is a single layer, pure bitmap, just freehand with an eraser also, and then we can backport this
layer in studio later to integrate the note in normal layers, it should not be long live (live only in a
page of a concert, be reported like that) so there is no confusion."*

And the clarification that fixes what "backport" means: *"the bitmap is especially done in order to be
sure it is not mixed for an annotation, the fact that you cannot directly import it as a layer that you
can do something is precisely for that (for example you print it between the pdf and the other layers and
then you recopy manually in annotation layers the notes you took)."*

| clause | what it fixes here |
|---|---|
| *"stage should just be a stage"* — acknowledged departure | I12's scope phrase gains one clause; its safety half is untouched and the presenter still has no annotation model (§3.1) |
| *"during rehearsal … (no internet)"* | everything in `stage/` is offline; the only network step is the explicit send in Part B, outside `stage/` |
| *"a single layer"* | one note per page; on Stage it is one extra layer row, present only when something was drawn (§3.8) |
| *"pure bitmap"* | the note IS a transparent bitmap; no strokes, no objects, no sidecar (§3.2) |
| *"just freehand with an eraser"* | pencil and eraser, 4 colours, 3 widths, opaque; no undo, no clear, no opacity, no highlighter (§3.6) |
| *"not mixed for an annotation … cannot directly import it as a layer"* | in Studio the bitmap is a **reference underlay between the PDF and the layers**, never an object; the musician recopies by hand (§7) |
| *"live only in a page of a concert, be reported like that"* | keyed by (song, page raster); reported on the page, in the layer list, in a Notes tab and on Home (§5) |
| *"so there is no confusion"* | never silently re-placed on a page that changed (§3.4); on Stage the note draws on top because *"you naturally draw on top of what you see"* |

### 1.2 VLL's answers (2026-09-10), recorded so nobody re-decides them

1. **Opacity:** *"no opacity control, you draw opaque."*
2. **Palette:** *"4 colors, 3 width (one really line where we can write, 1 medium when you can circle
   informations, 1 wide enough to redact stuffs)."*
3. **Page turns in the editor:** *"I don't see how we can page turn with the editor in, all press and
   movements will be drawing or erasing, except if we add another tool 'move 2D'."* → no touch turning in
   note mode; a move tool is a follow-up, not v1.
4. **Reading modes:** *"page + width, scroll is for the moment out of scope."*
5. **Input:** *"I have a stylus but it is passive, so it probably just emulates a finger."* → yes; there
   is no pen/finger distinction to build on.
6. **Across bakes:** *"I would like it to stay if the pdf page is the same (like song + file + page? or
   just the file hash?). If it is lost we can maybe keep it (just seeing it associated with the name of
   the song might be enough)."* → keep by (song, page raster hash); keep orphans, labelled by song (§3.4).
7. **Lifetime:** *"no time limit, it can probably stay between bakes but maybe warn after a few bakes:
   this annotation only exists on this device, have you backported it? can we delete it? yes / no."*
8. **Where notes are managed:** *"in stage native part a third tab with the bitmap layers: band + song +
   page where we can choose 'see' 'delete' and send to studio. In the home page a hint: '3 layers not
   sent to studio'. Layers depend on the performers they belong to, who send them (manage conflict when
   you are connected with an account that is not who you perform and took notes)."* Then, on being
   told the Stage section has no tabs: *"it is studio with tabs, so it is a second tab in stage, do the
   same kind of tab as in studio (of course with the right color) … first tab being the 'bakes'."*
9. **Studio side:** *"yes done remove. … for sure you know if it was already 'uploaded', the action on
   the tablet decides if it was sent, not the existence server side; if there is already a bitmap for
   this song/page it asks to overwrite."*
10. **Visibility:** *"this is indeed an extra layer, activated if there is something (no bitmap layer if
    there was nothing done) but it can be disabled."*
11. **Stacking on Stage:** *"On stage I think it should sit on top, or else it is strange to draw … for
    studio it is different as it is a reference."* And on covering baked marks: *"you can redact stuffs
    in the bitmap, it is sad but physically you wrote on top of something so at least it is natural."*
12. **Undo/clear:** *"no undo, no clear, to undo you use eraser, to clear page you remove it from the
    'notes' tabs in Stage."*
13. **The bar:** *"I have the feeling that the bottom feels more natural, but no strong opinion."* →
    bottom (§3.7).
14. **Entering the mode** (after the architect's critique that an accidental entry mid-set is the one
    real stage risk): *"we should have a confirmation when entering note mode — 'entering note mode
    yes/no' or something like that."*
15. **The warning** (after the architect proposed dropping the nag): *"the number of bakes is what I
    thought, a warning is bothering enough — in the tab name 'Notes ⚠' if at least one note is old, with
    the warning in orange or red."*

### 1.3 Why this is the design and not a stopgap (VLL, 2026-09-10)

The architect asked whether the server was reachable at rehearsal, because if it were, a one-tap
"annotate this page in Studio" would produce real annotations and need no second store. VLL: *"no, the
server is really not reachable because it is at a remote location, and the problem is rehearsing in
Studio is hard, and I don't want to import all the features into Stage — it would be too painful."*

So the three alternatives are each closed on his facts: Studio at rehearsal (no server); offline Studio
(rehearsing in Studio is hard regardless of connectivity); Studio's tools inside Stage (refused, and it is
exactly what I12 protects against). Rehearsal notes as pixels on Stage is the deliberate answer, not a
placeholder for one of those. Anyone proposing to "do it properly" later starts from this paragraph.

### 1.4 The limits are the message (VLL, 2026-09-10)

*"My problem with notes is that they cannot apply if I change the lyrics of a song, and then they are not
attached anymore. So I want to encourage people to recopy them: warning, not applying when changing the
hash, scoped to a tablet (cannot share them)…"*

So the three things that make a note second-class are **deliberate, and stay deliberate**:

- **it does not follow a changed page** — the hash key orphans it the moment the lyrics change (§3.4);
- **it cannot be shared** — it lives on one tablet, under one Stage identity, and never enters a bundle;
- **it nags** — `Notes ⚠` after a few bakes (§3.4).

Every one of these is an argument for recopying the note into a real annotation while it still means
something. A future task that "fixes" one of them — re-placing across a reflow, syncing notes between
devices, silencing the warning — is removing the reason the note is a bitmap. That task needs this
paragraph overruled first, by VLL.

## 2. What exists today (verified 2026-09-10 in the mobile checkout: `origin/main` at `49c9bd9f` plus the lane's uncommitted N10/T149 edits to `StageScreen.kt`, so `StageScreen.kt` numbers will shift when those land)

- **Pages are raster only.** `PageImages.pageRasterRef` is the PDF raster, `overlays[]` one transparent
  image per layer per page (`proto/troubastack/v1/bundle.proto`, mirrored in `BundleModel.kt:21-40`).
  No vector reaches the device. The presenter is a compositor + pager (I12).
- **An overlay registers with its page only because it is drawn by a sibling `Image` with the identical
  modifier and `contentScale`** (`PageView` and `ScrollPage` in `StageScreen.kt`). There is no
  page-to-screen matrix and no pan or zoom. That is the registration mechanism to reuse.
- **Ink colour on dark grounds is a per-pixel rule, not the page matrix** (A64): overlays go through
  `transformOverlayPixel(argb, scheme)` (`AnnotationColor.kt:255`) once per (overlay, size, scheme) and are
  drawn with `colorFilter = null`. Achromatic ink inverts with the paper; chromatic ink keeps its hue and
  only its lightness is remapped for contrast. A note is ink and follows the same rule — which is what
  makes a red note stay red in NIGHT and a black stroke invert with the paper, in every scheme.
- **`rasterHash` is a per-page content hash** (`PageImages.rasterHash`, sha256 of the raster bytes) already
  used by `StageViewModel.remapCurrent` to keep the viewport across a re-bake. There is **no page id**
  anywhere; a page is its index in `BakedSong.pages[]`. VLL's *"file hash"* is exactly this value, per page.
- **The T145 lesson.** A mark keyed by page index was silently re-pointed by a reflow. A bitmap has no
  text run to anchor to, so its only honest key is the raster hash, and the honest behaviour on a mismatch
  is to say so.
- **Bundle install is destructive.** `BundleImporter` replaces `bundlesDir/<concertId>` on every update, so
  notes live beside it, not in it.
- **The gesture stack** (`Performing`): drawer gesture off unless open; `stageTaps` (every tap toggles the
  chrome); `pointerInputSwipe` (page turns); `PageView`'s `verticalScroll` in FIT_WIDTH; scroll mode's
  pager and columns. The working tree's N10 `swipeLocked` is the precedent: one boolean read at every
  drag owner.
- **Layer visibility is per song** (`StageState.visibleBySong`, `visibleFor(songId)` unions mandatory),
  toggled in `LayersDialog`; `PersonalTag` is the existing "only your view" badge.
- **Persistence today is small strings through the Storage seam**, written by the host on callbacks
  (`onPositionChange` → `stage.pos.$concertId`, `MainActivity.kt:714`). The Stage identity is
  `identity.$concertId` (a roster member id, the *Who are you?* picker). The image decoder is plain DI
  (`ImageDecoder`, `StageScreen.kt:142-144`). Pixel-level expect/actuals outside the three seams exist
  (`OverlayTransform.kt`) — precedent under I15.
- **The Stage section of the app has no tabs**: it is a **Concerts** list grouped by band (T143 accordion)
  with `‹ Home`, `Edit`, `Import`, and `Connect`/`Sign out` in its header (`MainActivity.kt` ~`:800`);
  rows carry title + rev + date and a ⋮ with `BundleAction { Freeze, Unfreeze, Pin, Unpin, Delete }`
  (`distribution/BundleRow.kt`), labelled by a hand-maintained `when` in `MainActivity`. Home is two
  tiles, Stage and Studio (`home/HomeScreen.kt`). **The Studio section does have tabs**:
  `StudioBrowseScreen.kt:82-95`, a Material3 `TabRow` whose `SecondaryIndicator` and selected text take
  the Studio accent (the default indicator is `colorScheme.primary`, which the comment there rejects),
  tabs *Concerts* and *Bands*. VLL's ruling: the Stage section gets **the same component with the Stage
  accent** (`LocalBrandAccents.current.stage`), first tab **Bakes** (today's concerts list, unchanged),
  second tab **Notes** (§5.3).
- **Tests:** `commonTest` (pure model/VM over a synthetic `ConcertBundle`), `androidUnitTest` (JVM;
  source-guard greps, shared vectors). No Compose UI tests, no instrumented tests: pixel claims are an
  emulator/tablet pass with the artefact pulled and measured.
- **Studio has no image object, no eraser, no underlay concept.** `web/ink` renders the dry layer over
  the PDF canvas; the app POSTs only identity and bake-kick calls (`HttpTransport.kt`); song-file upload
  accepts `image/*` but a PNG uploaded there becomes a chart page.

## 3. Decisions

### 3.1 What actually changes in I12 (architect; corrected by the gate's first read, `41f5b48f`)

The first draft said "this amends I12" and paraphrased I12 as *no writes*. **I12 does not say that.** Its
rule is: *the presenter is a pure image compositor + pager; at performance time it depends on nothing
server-side and contains no annotation-model or access-control logic.* "No writes" lives in the file
headers (`StageModel.kt:1-4`) and the README's A-track paragraph, not in the constitution.

So a local per-page bitmap does not touch "nothing server-side" (offline by construction) and does not add
an annotation model or access-control logic (it is pixels). It contradicts only **"pure image compositor +
pager"**, which is a statement of **scope**, not a safety property. The narrower question VLL is being
asked is: *may the presenter accept pen input into its own local scratch surface?* — and his answers in
§1.2 say yes; the gate records the go.

**The amendment is to that phrase, not a clause naming this feature** (a carve-out that names one feature
dates at once): I12's rule becomes *"the presenter composites and pages flattened images, and may capture
local pixels that never reach a bundle; at performance time it depends on nothing server-side and contains
no annotation-model or access-control logic"* — the safety half verbatim. The architect lands it with the
GO.

`stage/` draws into memory and hands bitmaps to a host-implemented port; the port does the file I/O. The
headers of `StageModel.kt` / `StageViewModel.kt` change from *"NO writes"* to *"no writes except through
the rehearsal-notes port (A70)"*, and **that port is the only I/O `stage/` may reference — guarded, not
promised** (§4.6).

### 3.2 Pure bitmap, no stroke sidecar (VLL's *"pure bitmap"*, architect's reasoning)

No polyline log. The eraser makes it inexact the moment it is used; two representations of one drawing
must share a lifetime and will drift; and Part B does not need it — the underlay is the bitmap.

### 3.3 One note per page; the page is `(songId, rasterHash)` (VLL #6)

- `songId` is in the key because two songs can share a byte-identical raster (an intermission poster).
  Page index is **recorded** for labels and for Part B's placement, **never** used to find a note.
- Not `concertRev`: the same page in rev 9 and 10 has the same hash when it did not change, and VLL wants
  the note to *"stay if the pdf page is the same"*.
- **Location:** `<filesDir>/notes/<concertId>/` — beside `bundles/`. New `Storage.notesDir()` on the
  existing seam (IOS01's precedent of extending it).
- **Files:** `index.json` + one PNG per note named by the first 16 hex of `sha256(songId + "\n" +
  rasterHash)`. Entry:
  `{ songId, rasterHash, file, pageInSong, songTitle, bandId, bandName, concertRev, takenAs, width,
  height, updatedAt, sentAt?, bakesSinceTouched }`. `takenAs` is the Stage identity at save time (#8).
- **Deletion:** the Notes tab's Delete (#12: that IS "clear page"); T143's bundle Delete removes
  `notes/<concertId>/`. A save of an all-transparent bitmap deletes the note (#10: *"no bitmap layer if
  there was nothing done"*).
- **When `index.json` and the PNGs disagree** (raised at the gate, `41f5b48f`): the **index is the truth
  about what exists** — a PNG's name is an irreversible hash, so a file without an entry is unaddressable
  garbage and is deleted on load; an entry without a file is dropped from the index on load (there is
  nothing to show). Write order makes the first case the only one a crash can produce: PNG first, index
  last, both `tmp` + rename. A test covers each direction (§6.1).

### 3.4 Across a bake update: keep by hash, keep orphans, count bakes (VLL #6, #7)

On `applyUpdate` and on import of a newer bundle:

1. flush the note being drawn, leave note mode;
2. a note whose `(songId, rasterHash)` still exists is simply still there;
3. a note whose page is gone is **orphaned**: kept, labelled with its song title and last-known page and
   rev (*"just seeing it associated with the name of the song might be enough"*), viewable over plain
   paper, deletable, sendable;
4. `bakesSinceTouched` increments for every note not sent since its last save; at **`NAG_BAKES = 3`** a
   note is **old**. When at least one note is old, the Stage section's tab reads **`Notes ⚠`** with the
   glyph in the warning colour (orange, red once any note is past `2 × NAG_BAKES`), and the old note's
   row carries VLL's words: *"This note only exists on this device. Have you sent it to Studio? Delete
   it?"* with **Yes / No**. That tab label is the whole warning (#15: *"a warning is bothering enough"*);
   nothing on Home, nothing inside the performing surface;
5. the T143 update notice gains *"· N pages of notes are from the previous bake"* when N > 0.

**Never re-place a note on a page whose raster changed.**

### 3.5 Resolution and registration (architect)

- Fixed canonical width `NOTE_W = 1600`, height `round(NOTE_W * rasterH / rasterW)` from the **decoded**
  raster. ~13 MB ARGB per page; only the page(s) on screen in note mode hold a mutable bitmap.
- Drawn as one more sibling `Image` with the raster's modifier and `contentScale`, **above every baked
  overlay** (#11).
- Touch → note pixels is one pure function over the `Fit` (FIT_PAGE) or `FillWidth` (FIT_WIDTH)
  rectangle; `null` outside the page; a stroke leaving the page is cut at the edge.

### 3.6 Tools (VLL #1, #2, #12)

- **Pencil** and **eraser**, two buttons. Opaque always.
- **Three widths**, in note px: `FINE = 3` (writing), `MEDIUM = 9` (circling), `WIDE = 40` (redacting) —
  the executor tunes on the tablet and reports the numbers. The eraser uses the selected width ×3, never
  below `MEDIUM`.
- **Four colours** (VLL, 2026-09-10): **black `#111111`**, **red `#E53935`**, **blue `#1E63D6`**,
  **green `#2E8B3A`**. The architect first proposed paper white as a correction fluid; VLL rejected it —
  *"redacting in paper color means you don't really see it (to avoid)"* — a redaction must stay visible
  as a redaction, so it is done with a wide black stroke. All four are either achromatic (black inverts
  with the paper) or chromatic (hue kept, lightness remapped) under A64's rule, so each reads the same
  way in every scheme.
- Last tool, width and colour are remembered per device (`stage.note.tool/width/colour`).
- **No undo, no clear, no opacity, no highlighter, no shapes, no text.**
- **Wet/dry in miniature:** while the finger is down the stroke is a `Path` drawn by a Compose `Canvas` in
  screen space; at pen-up it is committed into the bitmap in one draw. The eraser applies to the bitmap on
  every move (its preview cannot be painted over). Latency is whatever Compose gives; a passive stylus is
  a finger (#5), so there is no palm rejection to build — the first pointer draws, extra pointers are
  ignored.
- **Colour on dark grounds:** outside note mode the note goes through `transformOverlayBitmap` like any
  overlay (scheme-augmented cache key carrying `updatedAt`). Inside note mode the committed bitmap is
  kept as a **display copy** transformed incrementally (the stroke's bounding box only) at each commit,
  and the wet stroke is drawn in `transformOverlayPixel(colour, scheme)`. The stored PNG is always the
  neutral colours.

### 3.7 Note mode (VLL #3, #4, #13)

- **Available in FIT_PAGE (single and two-up) and FIT_WIDTH.** Scroll mode: the entry item is disabled
  with *"Notes: switch to page mode"*.
- **Entered from a pencil item in the top bar next to ⚙**, labelled *"Notes"* (the `:672-675` ruling on
  mystery-dot FABs stands), **behind a confirmation** (#14): *"Enter note mode? Touch will draw, not turn
  pages."* — **Yes / No**, default focus on No. This is the one dialog the feature has, and it exists
  because an accidental entry mid-set is the one way this feature can hurt a show: in note mode a swipe
  no longer turns the page. `stageHoldsKeyFocus` already handles a dialog's focus (A50); the pedal keeps
  working through it. Session-only: off on every entry to Stage.
- **The note bar sits at the bottom** and replaces the `‹ ›` FABs while in note mode:
  `[Pencil] [Eraser] · ○ ○ ○ (widths) · ■ ■ ■ ■ (colours) · [Exit]`. The chrome does not auto-hide in
  note mode (Exit must be reachable); the top bar stays with the title and `Notes · this page only`.
- **All touch is drawing or erasing** (#3): `stageTaps`, `pointerInputSwipe`, FIT_WIDTH's
  `verticalScroll` and the drawer gesture are not attached. Consequence in FIT_WIDTH, stated plainly:
  only the visible part of the page is drawable; to reach the rest, Exit, scroll, re-enter. The **move
  tool** that fixes this is the named follow-up.
- **Hardware keys and the pedal still turn pages** (a pedal is not on the glass), and note mode stays on
  across such a turn; the page under the finger is always the page under the touch. Two-up: each
  `PageView` owns its note and its pointer input; a stroke never crosses the gutter.
- **Exit:** the Exit button, leaving Stage, `applyUpdate`. Every exit flushes (§4.5).

### 3.8 The note as a layer row (VLL #10)

`LayersDialog` lists **`Rehearsal notes · this device`** for the current song **only when the song has at
least one note**, default on, toggleable per song like any layer, stored in `visibleBySong` under the
reserved id `~notes` (no bake layer id starts with `~`; a source guard pins that the loader rejects one).
Entering note mode forces it on for that song.

## 4. Exact changes — Part A (mobile lane)

### 4.1 `app/shared/src/commonMain/.../stage/notes/` (new package)

- `NoteKey(songId, rasterHash)`, `NoteEntry` (§3.3), `NoteIndex` — pure: `attach(bundle)` partitions
  live/orphaned and returns per-song counts; `forPage`; `bumpBakes(exceptSent)`.
- `NoteGeometry` — pure: `noteSize(rasterW, rasterH)`, `touchToNote(box, note, contentScale, offset): Offset?`.
- `NoteFlushPolicy` — pure, injectable clock (T147 pattern): `dirty(now)`, `due(now)` at
  `IDLE_FLUSH_MS = 2000`, `force()`.
- `NoteTools` — the constants: widths, the four colours, `NOTE_W`, `NAG_BAKES`, the reserved `~notes` id.
- `RehearsalNotes` **port** (DI like `ImageDecoder`, not a seam): `index(concertId)`,
  `load(concertId, key): ImageBitmap?`, `save(concertId, key, entry, bitmap)`, `delete`, `deleteAll`,
  `markSent(concertId, key, at)`. `save` encodes a **copy** off the main thread, writes `tmp` then renames,
  rewrites `index.json` last, and deletes instead when no pixel has alpha > 0.

### 4.2 `StageModel.kt` / `StageViewModel.kt`

- `StageState` gains `noteMode`, `noteTool`, `noteWidth`, `noteColour`, `noteRevision` (bumped per
  commit), `noteCounts: Map<songId, Int>`, `orphanedNotes: Int`.
- `StageViewModel`: `requestNoteMode(): Boolean` (refused in scroll mode with a reason; otherwise only
  sets `noteModePending`), `confirmNoteMode()` (enters; forces `~notes` visible for the song),
  `cancelNoteMode()`, `exitNoteMode()`, `setNoteTool/Width/Colour`, `noteVisible(songId)`,
  `notesTabWarning(): None | Orange | Red`. None of them ever moves the page. `applyUpdate` calls `exitNoteMode()` first, re-attaches the index, bumps
  `bakesSinceTouched`, and carries the orphan count into the T143 notice.
- Headers updated per §3.1.

### 4.3 `StageScreen.kt`

- `PageView`: note sibling `Image` above the overlays; wet-stroke `Canvas`; the drawing `pointerInput`
  only when `state.noteMode`, inside the page box. `stageTaps`, `pointerInputSwipe`, `verticalScroll`
  gated off in note mode (extend the N10 condition). Auto-hide suspended in note mode.
- Top bar: the *Notes* pencil item beside ⚙. Bottom: the note bar replaces the FAB row in note mode.
- A small persistent badge *"✎ notes"* top-left of a page that has a note, in every mode, chrome hidden
  or not.
- `LayersDialog`: the `~notes` row (§3.8).
- Stored notes decode through `decodeOverlayCached` with a port-backed decode (LRU, pinning and the A64
  transform come for free).

### 4.4 Host wiring (`androidApp`, `iosMain`)

- `AndroidRehearsalNotes(notesDir)`: `asAndroidBitmap().compress(PNG)`, `BitmapFactory`, `getPixels` alpha
  scan. iOS: Skia `encodeToData` / `Image.makeFromEncoded`. Beside the image decoders.
- `Storage.notesDir()` on both actuals.
- `MainActivity`: persists tool/width/colour like `stage.clockVisible`; Stage host `onStop` flushes;
  T143's Delete calls `deleteAll(concertId)`; the Stage identity is passed in as `takenAs`.

### 4.5 Flush points (each with a test)

Pen-up + `IDLE_FLUSH_MS` idle · turning the page by key/pedal · `exitNoteMode` · `applyUpdate` · leaving
Stage · host `onStop`. A flush with no `dirty` is a no-op.

### 4.6 The write-port source guard (required by the gate, `41f5b48f` — part of this task, not a follow-up)

A70 adds the **first** write port to `stage/`, which widens exactly the surface I12's residual worries
about (*"no automated check forbids a server dependency creeping into the presenter"*). So, in
`androidUnitTest`, in the shape of `NoRawChromeSurfaceTest`: every file under `stage/` may import the
`RehearsalNotes` port and **nothing else that does I/O** — no `java.io`, `java.nio.file`, `okio`,
`java.net`, `io.ktor`, `android.*` file or network APIs. **Teeth-check at the gate:** add a forbidden
import to one `stage/` file, watch the guard go red, revert, and put the receipt in the entry.

## 5. Reporting — "be reported like that" (VLL #8, #10, #12)

1. **On the page:** the *"✎ notes"* badge; in note mode the top bar says *"Notes · this page only"*.
2. **In the layer list:** the `Rehearsal notes · this device` row, only when there is something.
3. **The Notes tab** — the Stage section becomes two tabs, **Bakes** | **Notes**, using the Studio
   section's `TabRow` pattern with the Stage accent (§2). *Bakes* is today's list untouched. *Notes* is
   grouped **band → song → page**, one row per note:
   *"page P of Q · rev R · taken as <member> · <date>"*, *"page changed in rev R'"* for orphans,
   *"sent <date>"* or *"not sent"*, the §3.4 nag when due; actions **See** (the PNG over plain
   `schemePaper`, or over the page raster when the page is still live), **Delete**, **Send to Studio**
   (Part B; disabled with *"sign in to send"* when offline or signed out).
4. **The tab label is the warning**: `Notes ⚠` in orange or red when a note is old (§3.4). **No Home
   hint** — VLL: one warning is bothering enough.
5. **On the library row:** the T143 subtitle gains *"· notes on N pages"* — in both intents; it is
   information, not a control.

## 6. Acceptance — RED FIRST, every row

Pure suites: `cd app && ./gradlew :shared:check`. Pixel rows: emulator or tablet, the PNG pulled with
`adb exec-out 'run-as com.troubastack.app cat files/notes/<id>/<file>'` and measured with PIL. Each test
is **seen failing** against the naive implementation named in its row, and the gate entry carries the
sabotage receipt (a break that silently fails to apply has bitten twice this month).

### 6.1 Pure (`commonTest` unless noted)

| assertion | fails against |
|---|---|
| `touchToNote` (Fit): portrait page in a landscape box maps the four page corners to the four bitmap corners and a letterbox touch to `null`; two-up halves likewise; (FillWidth): a touch below the visible band maps to the right note row | `offset * (noteW / boxW)` (ignores centring) |
| `noteSize` keeps the decoded aspect within 1 px for 2× and 4× downsampled rasters | nominal bundle size |
| `NoteIndex.attach`: same `(songId, rasterHash)` → live; same hash under another song → **not** matched; missing hash → orphaned, kept, never re-placed by index. Teeth: a fixture whose page 1 and 2 swapped rasters must orphan both, never swap them | keying by `(songId, pageInSong)` |
| `bumpBakes`: increments unsent notes, leaves sent ones; the nag predicate is true at 3 and false at 2 | a global counter |
| A64 for the palette: each of the four colours through `transformOverlayPixel` in NIGHT and AMBER keeps its hue within 10° (red, blue, green) or inverts (black), and every one clears 4.5:1 against the scheme's paper | drawing through the page matrix |
| `NoteFlushPolicy`: 1999 ms → not due, 2000 ms → due, `force()` → due, no-dirty → no-op | a flush per stroke |
| `StageViewModel`: `requestNoteMode` refused in scroll mode with a reason, otherwise sets `noteModePending` (the dialog) and **nothing else**; `confirmNoteMode` enters, `cancelNoteMode` does not; neither ever changes `state.current`; entering forces `~notes` visible; `applyUpdate` leaves note mode, bumps bakes, reports orphans; the `~notes` row is absent for a song with no note | entering on the first call (no confirmation) |
| Tab warning predicate: false with no old note; `⚠` orange at one note at `NAG_BAKES`; red at one past `2 × NAG_BAKES`; sent notes never count | counting sent notes |
| Loader rejects a bake layer id starting with `~` (`androidUnitTest`, torture fixture) | — |
| `RehearsalNotes` contract (`androidUnitTest`, fake bitmap): save → row; all-transparent save → row and file gone; `deleteAll` empties; `markSent` sets `sentAt` and the nag stops | — |
| Reconciliation on load: a PNG with no entry is deleted; an entry with no PNG is dropped; a matching pair is untouched (§3.3) | trusting either side alone |

### 6.2 Pixels (emulator or tablet — a gate)

- Draw, exit, kill the app, relaunch: the stroke is on the same words (crop + diff).
- Draw a line, erase across it: crossing pixels alpha 0, the rest intact (PIL numbers in the entry).
- A wide black stroke over a lyric line in NORMAL hides it and reads as a redaction; in NIGHT the same
  stroke is light and still hides it (A64 inversion); a red stroke is red in both, a green one green.
- The note draws **above** a baked overlay (a red note across a cue glyph covers it).
- Two-up: a stroke from the left page stops at the gutter; the right page's PNG is untouched.
- FIT_WIDTH: a stroke lands on the visible band at the right note rows after the page was scrolled
  before entering note mode.
- Re-bake with one song changed: unchanged pages keep their notes; the changed page's note is listed as
  orphaned with rev and song; the update notice names the count; after three more bakes the nag shows.
- Tapping *Notes* shows the confirmation; *No* leaves everything as it was and a swipe still turns the
  page; *Yes* enters and a swipe draws.
- The Notes tab: See, Delete; after three re-bakes with an unsent note the tab reads `Notes ⚠` in orange.

### 6.3 Source guards (`androidUnitTest`)

- The note `Image` uses the raster `Image`'s `contentScale` expression and sits after the overlays in the
  same box (state the property; the executor picks the grep).
- Every drag owner disarmed by `swipeLocked` is also disarmed by `noteMode`; `stageTaps` and FIT_WIDTH's
  `verticalScroll` too.
- The write-port guard of §4.6, with its sabotage receipt.

## 7. Part B — the underlay in Studio (VLL #8, #9) — **filed as T170** (`T170-rehearsal-notes-underlay-in-studio.md`, dispatched to web-core 2026-09-11); T170 is authoritative where the two differ

**The bitmap is never an annotation.** In Studio it is printed **between the PDF and the layers**, at
full opacity, as a reference the musician recopies from by hand, then removes. Nothing about it enters
the annotation model, the object types, the ink renderer or the bake.

1. **Send** — from the Notes tab (Part A), signed in. `PUT
   /api/bands/{b}/songs/{s}/rehearsal-notes/{pageInSong}` (multipart: the PNG +
   `{ rasterHash, concertId, concertRev, takenAs, width, height, capturedAt }`). One note per
   **(owner, song, pageInSong)** server-side; the PUT returns **409** when one exists, the tablet asks
   *"A note already exists in Studio for this song/page. Overwrite?"* and retries with `?overwrite=1` (#9).
   On 2xx the tablet sets `sentAt` — **the tablet's action decides "sent", not the server's state** (#9).
   `Content-Length` derives from the bytes written (T141).
2. **Identity conflict** (#8): a note carries `takenAs` (the Stage identity). Sending is always as the
   signed-in user, who becomes the owner. When the signed-in member ≠ `takenAs`, the tablet asks
   *"Taken as X · you are signed in as Y. Send as Y?"* — Send / Cancel. No silent re-attribution.
3. **Core** stores the PNG in the blob store and a `RehearsalNote` row `{ id, bandId, songId, ownerUserId,
   pageInSong, rasterHash, concertId, concertRev, takenAs, blobHash, width, height, capturedAt,
   uploadedAt }`; `GET …/rehearsal-notes` (owner's list), `GET …/rehearsal-notes/{page}` (bytes),
   `DELETE …/rehearsal-notes/{page}`. **Owner-only**: nobody else lists or fetches them. Each row also
   reports whether the song's **current** bake still has a page with that `rasterHash`.
4. **Studio**: when the signed-in user has notes for the open song, a chip *"Rehearsal notes (N)"* in the
   editor toggles the **underlay** — each page's PNG drawn under the dry layer, over the PDF, at the page
   index it was taken on, tagged *"page changed since"* when the hash no longer matches (the user judges;
   nothing is re-placed). Per page: **Done, remove** → `DELETE` (#9). Removal in Studio never reaches the
   tablet; the tablet's copy lives until deleted there or with its bake.
5. **Nothing else.** No object type, no proto change, no bake change, no renderer change.

## 8. Out of scope (v1)

Scroll mode · a **move 2D** tool (the named follow-up that makes FIT_WIDTH comfortable and could allow
touch turns in note mode) · undo, clear, opacity, highlighter, shapes, text · pressure · any sync from
`stage/` · anything in a bundle or the bake · Part B itself.

## 9. Sequencing and conflicts

- Starts only after the in-flight N10 swipe-lock/finger-follow and T149 edits land — same
  `StageScreen.kt` regions.
- Touches `shared` → compile iOS before landing (`./gradlew :shared:compileKotlinIosSimulatorArm64`).
- The I12 sentence (§3.1), the README's A-track paragraph (*"never writes"*) and `USER-JOURNEY.md`'s
  presenter bullet are the architect's edits, landed with the GO.

---

## ⟨D5⟩ 2026-09-12 — The eraser: clear the SWEPT PATH, and show it. §3.6 is not reversed.

VLL on the tablet: *"it does not erase all my path"* and *"the shadow of the path of the current stroke
eraser could be nice"*.

### R1 — erase the segments, not the samples

The pencil connects its touch samples into a polyline; the eraser clears a circular dab **at each sample**.
A fast finger samples sparsely, so the dabs do not overlap and the swept path is left combed. Erase the
**connected segments** between consecutive samples — a round-capped, round-joined line in Clear blend at the
eraser width — exactly the geometry the pencil already uses. This is a straight asymmetry bug: two tools
consuming the same pointer stream, one of which treats it as a path and the other as a set of points.

The eraser width is already `selected × 3, never below MEDIUM` (§3.6), so nothing about sizing changes.

### R2 — the "shadow" does NOT reverse §3.6, and reading it that way would build the wrong thing

§3.6 says: *"The eraser applies to the bitmap on every move (its preview cannot be painted over)."* That is a
statement about **deferral**, and it is still true — you cannot preview a *removal* by drawing something on
top, because the preview would have to show absence. The eraser must keep committing on every move.

**What VLL asked for is not a deferred commit; it is a trail.** A translucent indicator of the path the
finger has swept during the current stroke, drawn **above** the note, while the erasure continues to happen
immediately underneath. The two are orthogonal: one is *when the pixels change*, the other is *what the hand
is told it did*. Nothing in §3.6 forbids the second.

So:
- **Erasure stays immediate.** No change to when pixels are cleared.
- **A live overlay** follows the same polyline R1 erases, in screen space, translucent, at the eraser width —
  the `Canvas`-over-the-page mechanism the pencil's wet stroke already uses.
- **It disappears at pen-up**, with no commit of its own. It is chrome, never ink; it must never reach the
  bitmap, the PNG or T170's underlay.
- **Colour:** it must read over both light and dark paper and over ink of all four palette colours. A neutral
  translucent grey with a visible outline is the safe shape; the executor measures it on the tablet in two
  schemes rather than picking a hex here.

R1 makes R2 coherent rather than merely possible: once the eraser owns a real polyline, the shadow has an
exact thing to draw, and the two cannot disagree about what was cleared.

**Sizing:** small, one task, mobile. R1 is a bug fix and could land alone; R2 without R1 would draw a
continuous trail over a combed erasure, which would make the defect *more* visible, so land them together or
R1 first.

---

## ⟨D6⟩ 2026-09-12 — Note rendering: two transforms of one rule, a Clear that re-transforms nothing, and a state the user cannot reach

Four items from VLL's live testing. Two are reframed from how they were routed, and the reframing is the
part that changes what gets built.

### R1 — the wet→dry resettle is a TWO-IMPLEMENTATIONS problem; settle it by test, not by eye

The same authored ink goes through **two different transforms**: `transformOverlayPixel(colour)` while the
finger is down, `transformOverlayBitmap(pixels)` after the commit. That is one rule with two implementations
— the shape that has rotted three times in this repo already — and the reported symptom (red bright during
the drag, darker a few ms after lift) is exactly what divergence looks like.

**Do not tune either one until this is answered**, and it is answerable without the tablet:

> For **every palette colour × every scheme**: build a solid 1-colour bitmap, run `transformOverlayBitmap`,
> and compare the result pixel to `transformOverlayPixel(colour)` for the same scheme. They must be equal.

If they differ, that is the bug and the fix is to make one derive from the other so a third caller cannot
reintroduce it. If they agree, the cause is candidate (b) — geometry, screen-space wet path vs
commit-then-rescale — and *then* you measure the offset on device, in pixels, before proposing anything.
Either way the goal is WYSIWYG and the evidence comes first (this is the feel-bug rule: instrument, then fix).

### R2 — eraser lag: a Clear never needs a re-transform

Re-transforming the whole bitmap on every erase move, on the UI thread, is the cost. Your own observation is
the rule and it generalises past this bug, so state it that way in the code:

> **A Clear produces transparent pixels, and transparency has no colour to transform.** So an erase must
> update the display copy directly over the swept segment's bounds and never re-key or re-transform the
> bitmap.

Bound the work to the segment's bounding box, the same geometry ⟨D5⟩ R1 already clears. A full re-transform
stays correct for a *pencil* commit (which does add colour) — this is not licence to skip it there.

### R3 — the eraser shadow is already specced; do not re-rule it

⟨D5⟩ R2. It is **not** a reversal of §3.6: §3.6 forbids *deferring the commit*, and the shadow is chrome
drawn above while the erasure stays immediate. Build it from ⟨D5⟩, land it with or after ⟨D5⟩ R1, never before.

### R4 — "no clear" yields, because the system defines a state the hand cannot reach

§3.6 says *"no undo, no clear"*. That stands as a scope rule against building a tool suite in the presenter —
but it now collides with §3.3, which says **an all-transparent save deletes the note and drops the chip**.
VLL erased what he could see and 21 opaque pixels survived: the system defines "empty" as meaningful and
gives no way to reach it. That is not an ergonomics complaint, it is an **unreachable defined state**, and
the scope rule has to yield to it.

**Add a single explicit action in note mode — "Erase this note"** — with a confirmation, since it is
destructive and there is no undo. It deletes the note by the §3.3 path (file and index entry gone, chip gone)
rather than by painting transparent pixels, so it cannot leave specks behind by construction.

Still **no undo, no shapes, no text, no highlighter**: one destructive action reaching a state the model
already defines is not the thin end of a tool suite.

**Sizing:** R1 is a test first and possibly a one-line fix; R2 is a bounded redraw; R4 is a button and a
dialog. Mobile, after ⟨D5⟩.

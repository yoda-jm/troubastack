# A70 — Rehearsal notes on Stage: one bitmap per page, pen and eraser, nothing else

**Lane:** mobile (Part A). Part B is core + studio + bake + app, to be filed as its own T-task.
**Size:** L (Part A). **Filed:** 2026-09-10, by the architect, from VLL's request.

## ⛔ Status: WORK IN PROGRESS — a spec draft, NOT takeable

**Do not start this.** It is not in any lane's queue and no gate entry dispatches it. It is committed so
the design is reviewable and so the number is reserved, nothing more. VLL asked for it to be written
thoroughly and to be marked exactly this way: *"commit it but say it is work in progress and should not be
taken."* It becomes takeable only when a Fable entry in `docs/handoff/reviews.md` dispatches it, after
VLL has read the decisions in §3 and either accepted or overruled them.

Two things this draft deliberately leaves to that reading:

- **It amends I12.** The presenter has been "no writes" since A04, and the file headers say so
  (`StageModel.kt:1-4`: *"No annotation model, no access-control, NO network, NO writes"*). §3.1 states
  the narrowest exception that does the job and the sentence that goes into `docs/ARCHITECTURE.md`.
  That sentence is the architect's to land, not the lane's — and only once VLL confirms the carve-out.
- **Part B needs a new object type in the annotation model** (`image`, §7). That is a proto change and a
  renderer change; it is sketched here so Part A's on-disk format provably serves it, but it is decided
  at its own gate.

## 1. What VLL asked for, clause by clause

*"I know we said stage should just be a stage, but during rehearsal I need to take notes (no internet), my
idea is a single layer, pure bitmap, just freehand with an eraser also, and then we can backport this
layer in studio later to integrate the note in normal layers, it should not be long live (live only in a
page of a concert, be reported like that) so there is no confusion."*

| clause | what it fixes in this spec |
|---|---|
| *"stage should just be a stage"* — acknowledged as a departure | I12 carve-out is explicit and minimal (§3.1); the presenter still has no annotation model |
| *"during rehearsal … (no internet)"* | Stage side is fully offline; nothing in `stage/` touches the network; the only network step is Part B and it lives outside `stage/` |
| *"a single layer"* | exactly one note per page; no layer list, no ordering, no per-layer visibility |
| *"pure bitmap"* | the note IS a transparent bitmap; no stroke model, no objects, no vector sidecar (§3.2 explains why the tempting stroke log is rejected) |
| *"just freehand with an eraser also"* | two tools, pen and eraser, one colour, one width each; no undo, no shapes, no text |
| *"backport this layer in studio later"* | Part B: the bitmap travels to core and becomes an object on a normal layer, on the page it was drawn on (§7) |
| *"integrate the note in normal layers"* | the backported note is a real annotation object on a real layer, so it bakes, syncs and deletes like any other |
| *"should not be long lived (live only in a page of a concert …)"* | the note's identity is (concert, song, page raster); it does not follow the song across concerts and it never becomes a layer of the song by itself |
| *"… be reported like that"* | wherever the note shows, the UI says what it is: a badge on the page, a count on the library row, a sheet that lists pages and orphans (§5) |
| *"so there is no confusion"* | one fixed pencil colour distinct from any baked overlay, a persistent badge, and never silently re-placed on a page that changed (§3.4) |

"Not long lived" is read as **scope**, not as a timer: the note lives exactly as long as the page it was
drawn on is the page on the device. Nothing expires on a date. A note dies with the bake it belongs to,
with an explicit discard, or with the page's raster changing under it — and in that last case it is
reported as orphaned rather than deleted (§3.4). If VLL meant a time limit as well, that is one sentence
to add at the gate.

## 2. What exists today (verified 2026-09-10 in the mobile checkout: `origin/main` at `49c9bd9f` plus the lane's uncommitted N10/T149 edits to `StageScreen.kt`, so `StageScreen.kt` numbers will shift when those land)

- **Pages are raster only.** `PageImages.pageRasterRef` is the PDF raster, `overlays[]` one transparent
  image per layer per page (`proto/troubastack/v1/bundle.proto`, mirrored in `BundleModel.kt:21-40`).
  No vector reaches the device. The presenter is a compositor + pager (I12).
- **An overlay registers with its page only because it is drawn by a sibling `Image` with the identical
  modifier and `contentScale`** — `PageView` (`StageScreen.kt`, the raster then the overlays with the same
  `ContentScale.Fit`) and `ScrollPage` (same, `FillWidth`). There is no page-to-screen matrix anywhere, and
  no pan or zoom at all. That is the registration mechanism this feature must reuse, not reinvent.
- **Ink colour on dark grounds is a per-pixel rule, not the page matrix** (A64): overlays go through
  `transformOverlayPixel(argb, scheme)` (`AnnotationColor.kt:255`) once per (overlay, size, scheme) and are
  drawn with `colorFilter = null`. Achromatic ink (Lab chroma < 20) inverts with the paper; chromatic ink
  keeps its hue. A note bitmap is ink, so it follows the same rule.
- **`rasterHash` is a per-page content hash** (`PageImages.rasterHash`, sha256 of the raster bytes,
  `core/internal/bake/bundle.go`) and the app already uses it to keep the viewport across a re-bake
  (`StageViewModel.remapCurrent`: hash → (songId, pageInSong) → nearest → clamp). There is **no page id**
  anywhere in the system; a page is its index in `BakedSong.pages[]`.
- **The T145 lesson.** A mark stored against `(page index, x, y)` of one render was silently re-pointed
  by a reflow; nine hours of bakes on one side of a step change, ten on the other, every mark plausible
  and wrong. Anything keyed by page index alone repeats that. A bitmap covering the whole page has no
  text run to anchor to, so it cannot use T145's `SourceAnchor`; the only honest key it has is the raster
  hash — and the honest behaviour on a mismatch is to *say so*, never to guess.
- **Bundle install is destructive.** `BundleImporter` replaces `bundlesDir/<concertId>` wholesale on
  every update. Anything stored inside it is lost on the next auto-update — so notes live beside it, not
  in it.
- **The gesture stack** (`Performing`): drawer gesture disabled unless open; `stageTaps` (every tap
  toggles the chrome, N3); `pointerInputSwipe` for page/width turns; scroll mode's `HorizontalPager` and
  per-song `LazyColumn`; `PageView`'s own `verticalScroll` in FIT_WIDTH. **The working tree's N10
  `swipeLocked` is the exact precedent**: one boolean read at every place a drag is owned. Note mode is
  the same shape with one more owner (taps).
- **Persistence today is tiny strings through the Storage seam** (`Storage.getSecret/putSecret`,
  EncryptedSharedPreferences) written by the host, not by `stage/`: `onPositionChange` →
  `stage.pos.$concertId` (`MainActivity.kt:714`), chrono likewise. `stage/` itself writes nothing. The
  image decoder is plain DI (`ImageDecoder`, `StageScreen.kt:142-144`; `AndroidImageDecoder(root: File)`
  resolves refs against the bundle dir).
- **Colour-transform expect/actuals outside the three seams already exist** (`OverlayTransform.kt`,
  Android via `Bitmap.getPixels`, iOS via Skia) — precedent that a pixel-level platform function is not a
  fourth seam under I15.
- **The library surface** (T143): `BundleRow` (`app/shared/.../distribution/BundleRow.kt`) shows title +
  rev + date; `BundleAction { Freeze, Unfreeze, Pin, Unpin, Delete }` in the ⋮, with a hand-maintained
  `when` for labels in `MainActivity` (the enumeration the gate warned about on the re-bake item). The
  perform surface is `lean` and carries no management controls.
- **Tests:** `commonTest` (pure model / VM, synthetic `ConcertBundle`, no mocks), `androidUnitTest` (JVM;
  source-guard greps, shared vectors), **no Compose UI tests and no instrumented tests**. Pixel claims are
  therefore verified on the emulator or the tablet, with the artefact pulled and measured.
- **Studio has no image object and no eraser.** `ObjectType` stops at `icon`; removal is a `delete`
  tombstone; the ink registry adds a type in three steps (`web/studio/src/annotations/README.md`); the Go
  core treats types opaquely. The app POSTs only identity and bake-kick calls (`HttpTransport.kt`), never
  content. Uploads exist only for song files (`POST …/songs/{s}/files`, multipart, `image/*` accepted,
  32 MiB cap) — a PNG uploaded there would become a **chart page**, which is not what a note is.

## 3. Decisions (each named, with the reason; overrule at the gate, not in code)

### 3.1 The I12 carve-out, stated as narrowly as it can be

**Rule to add under I12:** *The presenter may hold one local, per-page, per-device bitmap of rehearsal
notes (A70). It is pixels, not objects: the presenter still contains no annotation model, no layer
model and no access-control logic, and it still performs with nothing server-side. The notes never
enter a bundle and are never read by the bake; they leave the device only through an explicit, separate
action outside `stage/` (Part B).*

Why this shape: the reason I12 exists is *stage reliability* — the smartness happened at bake time. A
bitmap the size of the page has no smartness in it. What would violate the spirit is a stroke model, a
sync client or a layer list inside `stage/`; none of those is added. `stage/` does the drawing into
memory and hands a bitmap to a port; the port (host-implemented, like `ImageDecoder`) does the file I/O.
The file headers of `StageModel.kt` / `StageViewModel.kt` change from *"NO writes"* to *"no writes except
the rehearsal-notes port (A70)"*, so the next reader is not lied to.

### 3.2 Pure bitmap, and no stroke sidecar

The tempting design is to also log the strokes as polylines at pen-up — it is nearly free at capture
time and would let Part B import editable `freehand` objects. **Rejected for v1**, for three reasons:
(1) VLL said *pure bitmap* and the eraser makes a stroke log inexact the moment it is used (a partially
erased stroke has no vector form); (2) two representations of one drawing is the paired-state trap this
repo has hit twice — they must share a lifetime and they will drift; (3) Part B works from the bitmap
alone (§7). If VLL later wants editable imports, a sidecar can be added as its own decision, with the
bitmap still the truth.

### 3.3 One note per page; the page is `(concertId, songId, rasterHash)`

- **Key:** `(songId, rasterHash)` inside the concert's notes directory. `songId` is in the key because two
  songs can share a byte-identical raster (an intermission poster, a blank page) and a note on one must
  not appear on the other. Page index is **recorded** (for display and for Part B) but is **never** used
  to find a note.
- **Why not `concertRev`:** the same page in rev 9 and rev 10 has the same raster hash when it did not
  change, and a musician's notes should survive the bake that only touched another song. The hash
  expresses "the page under my pen is the same page"; the rev does not.
- **Storage location:** `<filesDir>/notes/<concertId>/` — beside `bundles/`, never inside a bundle
  directory (install replaces it). A new `Storage.notesDir()` on the existing seam (Android: `filesDir/notes`;
  iOS: the sibling of `bundlesDir()`), following IOS01's precedent of extending the seam rather than
  adding one.
- **Files:** `index.json` (the truth about what exists) + one PNG per note, named by the first 16 hex of
  `sha256(songId + "\n" + rasterHash)` so no sanitising of ids is needed. `index.json` entries:
  `{ songId, rasterHash, file, pageInSong, songTitle, concertRev, width, height, updatedAt }` — `pageInSong`,
  `songTitle` and `concertRev` are *as of the last save*, for labels and for Part B's placement; they are
  not keys.
- **Deletion:** T143's Delete on a bundle removes `notes/<concertId>/` with it — the note's life is the
  bake's life. Discarding one note removes its file and entry. A save of a fully transparent bitmap
  **deletes** the note (an empty note is not a note; the badge disappears).

### 3.4 Across a bake update: carry by hash, orphan the rest, and say so

On `applyUpdate` (auto-update at rehearsal is exactly when this fires):

1. flush the note being drawn (§4.5), leave note mode;
2. every note whose `(songId, rasterHash)` still exists in the new bundle is simply still there — no
   copy, no move, it was never keyed by index;
3. every note whose page is gone is **orphaned**: kept on disk, listed in the sheet (§5.3) with its
   last-known song, page and rev, viewable over plain paper, discardable, and sent by Part B with its rev
   so Studio can still place it;
4. the T143 update notice gains a suffix when the orphan count is non-zero: *"… · 2 pages of notes are
   from the previous bake"*. Never a dialog, never steals the page.

**Never re-place a note on a page whose raster changed.** That is the T145 bug in bitmap form, and a
bitmap has no anchor to re-project from. Orphaning loudly is the correct behaviour, not a limitation.

### 3.5 Resolution and registration

- The note bitmap has a **fixed canonical width `NOTE_W = 1600`** and height
  `round(NOTE_W * rasterH / rasterW)` from the **decoded** raster's dimensions (the decoder downsamples by a
  power of two; using the decoded aspect keeps the two `Fit` rectangles within a pixel of each other).
  1600 px across a page is comfortably above the tablet's fitted page width in two-up and below it in
  single-page portrait; notes are pencil, not engraving. A 1600×2100 ARGB bitmap is ~13 MB; only the
  page(s) on screen in note mode hold a mutable one.
- It is **drawn as one more sibling `Image` with the same modifier and `contentScale` as the raster**, so
  registration is the same mechanism overlays use. A source-guard test pins this (§6.3).
- **Touch → note pixels** is one pure function: given the page box `(wPx, hPx)`, the note `(w, h)` and a
  touch offset, compute the `Fit` rectangle (scale = min(wPx/w, hPx/h), centred) and map into note
  pixels; `null` when the touch is outside the page. Tested with vectors (§6.1). A stroke that leaves the
  page is **cut at the edge**, not clamped to it.

### 3.6 Tools, colour, width

- **Pen:** one colour, `PENCIL = #3A3A3A`, fully opaque, round caps and joins, width `PEN_W = 4` note px.
  Achromatic on purpose: A64's rule 1 inverts it with the paper in NIGHT/AMBER exactly like printed text,
  so a note reads as handwriting in every scheme, and it is visibly not any baked overlay colour (the
  palette's ink is `#111827`; cue and note colours are chromatic).
- **Eraser:** `BlendMode.Clear` (destination-out) with a round brush, `ERASER_W = 40` note px. The eraser
  affects **only the note bitmap** — it cannot touch the page raster or a baked overlay, which is the
  reason it needs no confirmation.
- **No pressure, no width choice, no colour choice, no undo.** "Clear page" exists (two taps: the button,
  then an inline *Really clear?* in the same bar — never a dialog, dialogs steal key focus, A50). Undo on a
  pure bitmap means one full-page copy per stroke; deferred, and the eraser is the undo a pencil has.
- **Live stroke vs committed pixels** (design/03's wet/dry split, in miniature): while the pen is down,
  the current stroke is drawn as a `Path` by a Compose `Canvas` in screen space over the page; at pen-up it
  is committed into the bitmap in one draw and the `Image` invalidates once. The eraser, whose preview
  cannot be painted over, applies to the bitmap on every move. Input-to-photon latency is whatever
  Compose pointer input gives; if it feels laggy on the tablet that is A07's territory, not this task's.
- **Colour on dark grounds:** outside note mode the note goes through `transformOverlayBitmap` like any
  overlay, cached under a scheme-augmented key that also carries `updatedAt`. Inside note mode the live
  bitmap is drawn through the **page `ColorFilter`** (per-frame per-pixel transform is too slow for a
  mutable bitmap). These two paths must agree for `PENCIL` — a pure test asserts
  `transformOverlayPixel(PENCIL, scheme) == pageMatrix(scheme) · PENCIL` for all four schemes (§6.1); if a
  scheme's legibility clamp moves the grey, pick a `PENCIL` for which the two coincide rather than
  special-casing.

### 3.7 Note mode: where it is allowed, how it is entered, what it disarms

- **Available in FIT_PAGE only** (single and two-up). FIT_WIDTH scrolls inside the page and scroll mode
  scrolls a column; "one finger draws, one finger scrolls" is the stylus/finger routing question that
  A07's tablet session has not answered, and guessing it produces a note that scrolls the page or a
  scroll that draws. In those modes the Notes chip is disabled with the reason *"Notes: switch to page
  mode"*. Follow-up, not v1.
- **Entered from a labelled chip "Notes" in the top bar** (the chrome auto-hides, so it never sits over the
  music; the ruling at `StageScreen.kt:672-675` that a bare glyph FAB "reads as a mystery dot" is why it
  is labelled). No server gate: rehearsal is offline. Not persisted: **note mode is session-only and off
  on every entry to Stage**, in the spirit of I13's transient auto-update.
- **In note mode the top bar becomes the note bar:** `[Pen] [Eraser]` · *"Notes · this page only"* ·
  `[Clear page] [Done]`. The chrome **does not auto-hide** while in note mode (Done must be reachable).
  The bottom `‹ ›` FABs, hardware keys and the pedal keep turning pages — a pedal is not a finger — and
  note mode **stays on across a turn**; the page under the pen is always the page under the touch.
- **Disarmed while in note mode:** `pointerInputSwipe` (not attached, as `swipeLocked` does), `stageTaps`
  (a tap is a dot, not a chrome toggle), and the drawer gesture (already off). Two-up: each `PageView`
  owns its own note and its own pointer input; a stroke never crosses the gutter.
- **Stylus and palm:** the pen-seen idiom from `WetCanvas.tsx`: once a `PointerType.Stylus` event has been
  seen in this Stage session, finger touches do not draw (they do nothing in note mode). Without a stylus,
  one finger draws. Multi-touch: the first pointer draws, extra pointers are ignored.
- **Exit:** Done, leaving Stage, or `applyUpdate`. Every exit flushes (§4.5).

### 3.8 Visibility

A global persisted preference **"Show rehearsal notes"** (Settings sheet, beside "Show clock", default
on) so a performer can hide every note for a show without discarding anything. Independent of note mode:
entering note mode forces the current page's note visible.

## 4. Exact changes — Part A (mobile lane)

### 4.1 `app/shared/src/commonMain/.../stage/notes/` (new package, pure where it can be)

- `NoteKey(songId, rasterHash)`; `NoteEntry` (the `index.json` row, §3.3); `NoteIndex` — pure:
  `attach(bundle)` partitions entries into *live* (page exists in this bundle) and *orphaned*, counts per
  song, `forPage(songId, rasterHash)`. Serialised with the kotlinx-serialization setup `BundleModel.kt`
  already uses (`ignoreUnknownKeys`, defaults for every field).
- `NoteGeometry` — pure: `noteSize(rasterW, rasterH)` and `touchToNote(box, note, offset): Offset?` (§3.5).
- `NoteFlushPolicy` — pure state machine with an injectable clock (the T147 pattern): `dirty(now)`,
  `due(now)` (true `IDLE_FLUSH_MS = 2000` after the last stroke), `force()`. No sleeping in tests.
- `RehearsalNotes` **port** (a `fun interface`-style DI object, like `ImageDecoder`, **not** a seam):
  `index(concertId)`, `load(concertId, key): ImageBitmap?`, `save(concertId, key, entry, bitmap)`,
  `delete(concertId, key)`, `deleteAll(concertId)`. `save` encodes **a copy** (never the live bitmap) off
  the main thread, writes `tmp` then renames (A05's atomic pattern), rewrites `index.json` last, and
  **deletes** instead when the bitmap has no pixel with alpha > 0.
- `PENCIL`, `PEN_W`, `ERASER_W`, `NOTE_W`, `IDLE_FLUSH_MS` are named constants in this package.

### 4.2 `StageModel.kt` / `StageViewModel.kt`

- `StageState` gains `noteMode: Boolean = false`, `noteTool: NoteTool = PEN`, `notesVisible: Boolean = true`,
  `noteRevision: Int` (bumped on commit so the `Image` recomposes), `noteCounts: Map<songId, Int>` and
  `orphanedNotes: Int` (from `NoteIndex.attach`, for the badge/labels).
- `StageViewModel`: `enterNoteMode(): Boolean` (refused with a reason outside FIT_PAGE; never moves the
  page), `exitNoteMode()`, `setNoteTool`, `clearPageNote`, `setNotesVisible`. `applyUpdate` calls
  `exitNoteMode()` **first**, then re-attaches the index and carries the orphan count into the T143 notice.
- Header comments updated per §3.1.

### 4.3 `StageScreen.kt`

- `PageView` gets the note sibling `Image` (same modifier + `contentScale` as the raster), the wet-stroke
  `Canvas`, and — only when `state.noteMode` — the drawing `pointerInput` placed **inside** the page box so
  hit-testing is per page. `stageTaps` and `pointerInputSwipe` are not attached in note mode (extend the
  N10 condition). Auto-hide is suspended in note mode (`overlayOpen`-style gate).
- The note bar replaces the top bar's contents in note mode; the `Notes` chip sits between the title and
  the metronome capsule; a small persistent **badge** *"✎ notes"* in the page's top-left corner whenever
  the page has a note, in every mode, chrome hidden or not (it is the "be reported like that").
- Settings sheet: the "Show rehearsal notes" switch (§3.8).
- Decoding a stored note for display goes through `decodeOverlayCached` with a `RehearsalNotes`-backed
  decode, so it inherits the LRU, the pinning and the A64 transform; the cache key carries `updatedAt`
  and the scheme.

### 4.4 Host wiring (`androidApp`, `iosMain`)

- `AndroidRehearsalNotes(notesDir)` implements the port: `asAndroidBitmap().compress(PNG)` for save,
  `BitmapFactory` for load, `getPixels` alpha scan for the empty check. iOS: Skia `encodeToData` /
  `Image.makeFromEncoded`. Both live where `AndroidImageDecoder` / `IosImageDecoder` live.
- `Storage.notesDir()` added to the seam (both actuals).
- `MainActivity`: persists `stage.notesVisible` like `stage.clockVisible`; **Stage host `onStop` flushes**
  (a note lost because the tablet slept is the T147 failure in a worse form); T143's Delete calls
  `deleteAll(concertId)`.

### 4.5 Flush points (all of them, and a test for each)

Pen-up after `IDLE_FLUSH_MS` idle · leaving the page (turn) · `exitNoteMode` · `applyUpdate` · leaving Stage ·
host `onStop`. A flush of an unchanged bitmap is a no-op (the policy's `dirty` flag, not a pixel compare).

### 4.6 The library surface (§5)

## 5. Reporting — "be reported like that"

1. **On the page:** the *"✎ notes"* badge (§4.3). In note mode the bar says *"Notes · this page only"*.
2. **On the library row** (`BundleRow`): the T143 subtitle gains *"· notes on N pages"* when N > 0, and
   *"· M from an earlier bake"* when orphans exist. Perform intent (`lean`) shows the same subtitle — it is
   information, not a control.
3. **In the ⋮:** `BundleAction.Notes` → a **Rehearsal notes sheet**: one row per note — song number in
   the running order, *"page P of Q"*, date, *"from rev R (page changed)"* for orphans — each with a
   thumbnail and **Discard**; **Discard all** at the bottom; an orphan row opens the PNG over plain paper
   (`schemePaper`) full-screen since its raster is gone. Part B adds **Send to Studio** to this sheet.
   The `when` in `MainActivity` learns the new member; make it exhaustive if it is not.

## 6. Acceptance — RED FIRST, every row

Run from `app/`: `./gradlew :shared:check` for the pure suites; the pixel rows are an emulator/tablet
pass with the artefact pulled by `adb exec-out 'run-as com.troubastack.app cat files/notes/<id>/<file>'`
and measured with PIL. A test must be **seen failing** against the naive implementation named in its row.

### 6.1 Pure (`commonTest`)

| assertion | fails today / against |
|---|---|
| `touchToNote`: portrait page in a landscape box maps the page's four corners to the bitmap's four corners and a touch in the letterbox to `null`; two-up half boxes likewise; a 1-px-off aspect still maps corners within 1 px | a naive `offset * (noteW / boxW)` (ignores centring) |
| `noteSize` keeps the decoded aspect within 1 px for a 2× and a 4× downsampled raster | using the bundle's nominal size |
| `NoteIndex.attach`: a note whose `(songId, rasterHash)` is in the bundle is live; same hash under another song is **not** matched; a missing hash is orphaned, not dropped, not re-placed by page index | keying by `(songId, pageInSong)` — the teeth: a fixture whose page 1 swapped raster with page 2 must orphan both, never swap them |
| `transformOverlayPixel(PENCIL, s) == pageMatrix(s)·PENCIL` for all four schemes | a chromatic pencil |
| `NoteFlushPolicy`: dirty then idle 1999 ms → not due; 2000 ms → due; `force()` → due immediately; a flush with no dirty is a no-op | a tick counter / a flush-on-every-stroke |
| `StageViewModel`: `enterNoteMode` refused in FIT_WIDTH and scroll mode with a reason, accepted in FIT_PAGE, and **never changes `state.current`**; `applyUpdate` leaves note mode and reports the orphan count; `exitNoteMode` requests a flush | — |
| `RehearsalNotes` contract test (androidUnitTest, a fake bitmap): save → index has the row; save of an all-transparent bitmap → row and file gone; `deleteAll` empties the directory | — |

### 6.2 Pixels (emulator or tablet — this is a gate, not a nicety)

- Draw a stroke across a page, exit note mode, kill the app, relaunch: the stroke is there, on the same
  words (screenshot before/after, cropped and diffed).
- Draw a line, erase across it: in the pulled PNG the crossing pixels have alpha 0 and the rest of the
  line does not (PIL assertion, numbers in the gate entry).
- NIGHT: the note reads light on dark and the page raster's own text reads the same way; the badge is
  visible with the chrome hidden.
- Two-up: a stroke started on the left page ends at the gutter; the right page's PNG is untouched.
- Re-bake with one song changed: notes on unchanged pages are still there; the changed page's note is
  listed as orphaned with its rev, and the update notice names the count.

### 6.3 Source guards (`androidUnitTest`, the pattern of `NoRawChromeSurfaceTest`)

- The note `Image` in `PageView` uses the same `contentScale` expression as the raster `Image` (state the
  property; the executor picks the grep).
- Every drag owner disarmed by `swipeLocked` is also disarmed by `noteMode`, and `stageTaps` is.
- `stage/` imports nothing from `java.io` / `okio` / networking: the write stays behind the port.

### 6.4 Sabotage receipts

For each guard, the gate entry shows the sabotage that was applied (diff or grep), the red, and the green
after revert — a sabotage that silently fails to apply has bitten twice this month.

## 7. Part B — the backport (sketch to be filed as its own T-task; decisions that Part A's format must honour)

**Path:** tablet → core → Studio → a normal layer → the next bake.

1. **Transfer — explicit, from the library, outside `stage/`.** The Rehearsal notes sheet gains **Send to
   Studio** (needs a session and the network, like the re-bake kick). It POSTs every note of the concert,
   orphans included, to a new `POST /api/bands/{b}/songs/{s}/rehearsal-notes` (multipart: the PNG +
   `{ concertId, concertRev, pageInSong, rasterHash, width, height, capturedAt }`). Content-addressed by
   the PNG's sha256, so re-sending is idempotent. `Content-Length` derives from the bytes written (T141).
   Sending does **not** delete the note on the device; the row shows *"sent"*.
2. **Core** stores the bytes in the existing blob store and a `RehearsalNote` record per (song, sha256),
   served by `GET …/songs/{s}/rehearsal-notes` and `GET …/rehearsal-notes/{id}` (bytes). It also reports,
   per note, whether the song's **current** bake has a page with that `rasterHash` — the honest
   "the page has (not) changed since you drew" signal Studio needs.
3. **Studio** lists the song's notes in the editor (thumbnails, page, rev, date, changed/unchanged) with
   **Place on layer…** → creates an object `{ type: "image", points: [(0,0),(1,1)], page: pageInSong,
   text: <note id>, style: {opacity: 1} }` on the chosen layer (personal by default). The whole-page box
   is the placement because the bitmap *is* the page-sized overlay; the user then draws over it or deletes
   it like any object. **A note whose page changed is placed with a warning and a side-by-side preview**,
   never silently.
4. **Model:** `OBJECT_TYPE_IMAGE = 8` in `object.proto` (additive; `buf breaking` passes), the Go mirror
   regenerated, `web/ink` gains `drawImage` in the registry (image fetched by `text` id through a
   resolver the host provides: Studio via the GET, bake via bytes core passes in the batch request), so
   the editor and the bake render it through the one renderer (I8) and the parity test covers it.
5. **Anchoring:** an image object has no `SourceAnchor` (nothing to anchor a page-sized bitmap to);
   `Points` are authoritative, like a mark on an uploaded PDF. T145's reflow-orphan guard still applies
   at bake: if the page index no longer exists the bake fails loudly.
6. **After the backport** the note is a normal object on a normal layer: it bakes into that layer's
   overlay, it syncs, it deletes. The device-side note is then redundant; it stays until the bake it
   belongs to is deleted or the user discards it, per §3.3. Whether "sent and now baked" should
   auto-discard is a Part B decision for VLL.

Rejected for B: vectorising the bitmap (lossy, and the eraser has no vector form); a tracing underlay
without an object (cannot bake, so it never "integrates into normal layers"); uploading the PNG as a
song file (it would become a chart page).

## 8. Out of scope (v1)

Note mode in FIT_WIDTH and scroll mode (§3.7) · undo · colours, widths, pressure · text or shapes · a
per-layer or per-member note · any sync from `stage/` · anything in a bundle or the bake · Part B itself.

## 9. Sequencing and conflicts

- **Starts only after the in-flight N10 swipe-lock + finger-follow and T149 title-clip changes land** —
  they edit the same `StageScreen.kt` regions (the drag owners, `PageView`, `ScrollPage`), and two tasks
  in one file do not run in parallel here.
- Touches `shared` → compile the iOS targets before landing
  (`./gradlew :shared:compileKotlinIosSimulatorArm64`).
- The I12 sentence (§3.1), the README's A-track paragraph (*"never writes"*) and `USER-JOURNEY.md`'s
  presenter bullet are the architect's edits, landed with the GO, not before.

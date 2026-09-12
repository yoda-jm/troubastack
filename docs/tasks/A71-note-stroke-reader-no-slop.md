# A71 — The note pad reads the finger directly: no touch slop, the touchdown is the first point

**Lane:** mobile. **Size:** S. **Filed:** 2026-09-12 by the architect from VLL's report and his ruling.
**Parent:** A70 (rehearsal notes on Stage) — this is a follow-up to Part A's §3.6 wet/dry stroke.

## Status: DISPATCHED to mobile, 2026-09-12 (VLL: *"ok fill it for the mobile lane to fix"*)

## 1. The report, the diagnosis, and the ruling

VLL, 2026-09-12, on the tablet with a finger in note mode:

> *"it was difficult to draw correctly small stuffs (small segments, dots, ...) is this because of the
> hardware, of the human (me) or the software ? … I was expecting a near paper experience"*

**Diagnosis (architect, read from the Compose 1.9.0 source, not guessed):** the pad captures strokes with
Compose's `detectDragGestures`, a detector built for scrolling. It waits until the pointer has moved past the
platform **touch slop** (8 dp on Android, ~1.3–1.5 mm on the tablet) before it reports anything, and the
position it then hands to `onDragStart` is the *slop-trigger* change, not the touchdown (the public four-arg
overload maps `onDragStart = { _, slopTriggerChange, _ -> onDragStart(slopTriggerChange.position) }`). The
pad seeds the wet stroke from that point, so **the touchdown is discarded on every stroke.** Consequences:

- a stroke shorter than the slop never becomes a drag; it falls through to `detectTapGestures`, whose
  `onTap` receives the **finger-up** position → a dot at the END of the intended segment;
- a stroke a little longer than the slop draws with its first ~1.5 mm missing, starting late;
- a deliberate dot works (the tap path); long strokes look fine (a missing start is invisible on 3 cm).

Small marks broken, big marks fine — exactly the report. The hardware (a ~7 mm capacitive contact whose
centroid wanders) and the human (a finger hides its own target) add wobble and mis-placement, but they do
not make marks vanish or start late; that part is ours.

**VLL's ruling, which is the design:** *"since this is only used for note and for note we don't have any
gesture (you go out with the exit in the menu), I think it is safe to be as stupid and simple as possible."*
Verified in the code (§2): in note mode nothing else wants the touch, so there is nothing to disambiguate,
and disambiguation is the only reason a slop threshold exists.

## 2. What exists today (verified 2026-09-12 against `origin/main`; symbols and greps, no line numbers)

- **The two detectors**, both on the wet-stroke `Canvas` in `NoteLayer`:
  `git grep -n "detectTapGestures\|detectDragGestures" app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/NotePad.kt`.
  The tap detector marks a dot / erases a dab (A70 §3.7 *"all touch is drawing"*); the drag detector
  accumulates `wet` (screen-space `Offset`s) and commits on `onDragEnd` through `strokeInto` in NOTE space via
  `NoteGeometry.touchToNote`. The eraser applies live on every move (no wet preview).
- **A one-point stroke is already a dot:** `strokeInto` draws a filled circle when `pts.size == 1`
  (grep `A dot: a zero-length stroke` in `NotePad.kt`). So a down-and-up with no movement needs no separate
  tap path once the reader includes the touchdown.
- **Note mode owns the touch.** In `StageScreen.kt` every other consumer is removed when `state.noteMode`:
  the chrome tap (`stageTaps`), the turn-swipe, the fit-width `verticalScroll`, and the jump-mark taps —
  `git grep -n "noteMode) Modifier\|noteMode != true\|noteMode == true" app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/StageScreen.kt`.
  Scroll mode refuses entry (A70 §3.7); the drawer takes gestures only while open (`gesturesEnabled =
  drawerState.isOpen`); exit is the **Done** button in the note bar. The existing guard
  `every_swipeLocked_drag_owner_is_also_noteMode_gated` in `StageNotesGuardTest.kt` pins this.
- **Pointers:** A70 §3.6 — *"a passive stylus is a finger, so there is no palm rejection to build — the
  first pointer draws, extra pointers are [ignored]"*. VLL's stylus is passive (A70 §1.2 #5).
- **No Compose UI test harness in `app/`** (`git grep -l "runComposeUiTest\|createComposeRule" app/` is
  empty). The seam is proven on the device (§5.3), the reader is proven pure (§5.1).

## 3. Decisions

### D1 — No slop. The touchdown is the first point of the stroke.
The first sample the pad commits is the down position; every subsequent move of that pointer is appended
as it arrives. No minimum distance, no minimum point count, no gesture library between the finger and the
list of points.

### D2 — One reader for both tools; a tap is a one-point stroke.
`detectTapGestures` and `detectDragGestures` both go. Pencil: down-and-up with no move commits a one-point
stroke → `strokeInto`'s dot branch. Eraser: the dab applies at the down immediately, then on every move.

### D3 — First pointer wins (A70 §3.6, unchanged).
The stroke belongs to the pointer that went down first. A second pointer going down mid-stroke is ignored:
it does not start a stroke, its moves are not appended. The stroke ends when the FIRST pointer lifts, even
if the second is still down. With a passive stylus a resting palm is the realistic second pointer.

### D4 — Consume what you read.
The reader consumes every change it takes. Nothing competes today; this is insurance so a parent gesture
added later cannot steal a stroke mid-way. Cheap, and it keeps the guard in `StageNotesGuardTest` honest.

### D5 — The reader is a pure state machine; the Compose glue is thin.
Put the logic in `notes/StrokeReader.kt` (commonMain, no Compose imports): it is fed pointer events
`(pointerId, phase: Down|Move|Up, x, y)` and yields a completed stroke (its point list) on the owning
pointer's `Up`, plus the running wet points for the preview. `NoteLayer` becomes a loop that forwards
Compose's `PointerEvent` changes into it. This is what makes §5.1 possible without a UI harness. Mechanism
suggestion, not a requirement: `awaitEachGesture { awaitFirstDown(); while (…) awaitPointerEvent() }`.

### D6 — Nothing else changes.
No smoothing, no resampling, no pressure, no stylus/finger distinction, no palm rejection. The wet-preview
path (`Path` of `lineTo`s in screen px) and the commit (`strokeInto` in note px) stay as they are. If small
marks still feel wrong after this lands, that is a NEW report, measured, not a retune of this one.

## 4. Exact changes

- `app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/notes/StrokeReader.kt` — new, pure (D5).
- `app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/NotePad.kt` — replace the two
  `pointerInput` modifiers on the wet `Canvas` with one that drives `StrokeReader`; delete the imports of
  both detectors.
- `app/shared/src/commonTest/kotlin/com/troubastack/shared/stage/notes/StrokeReaderTest.kt` — §5.1.
- `app/shared/src/androidUnitTest/kotlin/com/troubastack/shared/stage/StageNotesGuardTest.kt` — §5.2.

## 5. Acceptance — RED FIRST, every row

### 5.1 Pure (`commonTest`, `StrokeReaderTest`)
Coordinates in whatever unit; the discriminating ones are SMALLER than any slop (use single-digit values):
1. `Down(P0) · Up` → one stroke `[P0]` (the dot).
2. `Down(P0) · Move(P1) · Up` with `|P1−P0|` tiny → one stroke `[P0, P1]`. **Not** `[P1]`, **not** empty.
   This is the row the old code fails: a sub-slop stroke became a dot at the up position.
3. `Down(P0) · Move(P1) · Move(P2) · Up` → `[P0, P1, P2]` — the first committed point IS the down (D1).
4. Second pointer: `Down(id=1,P0) · Down(id=2,Q0) · Move(id=2,Q1) · Move(id=1,P1) · Up(id=1)` → one stroke
   `[P0, P1]`; `Q*` appear nowhere; the stroke is complete at `Up(id=1)` while id=2 is still down (D3).
5. After the owning pointer lifts, a fresh `Down` starts a fresh stroke — the reader has no carried state
   (paired-state rule: the wet list and the owner id share one lifetime).
6. Eraser: the reader reports the down as an immediate point (the dab at touchdown), not only on the first
   move.

### 5.2 Source guard (`androidUnitTest`, `StageNotesGuardTest`)
`NotePad.kt` contains neither `detectDragGestures` nor `detectTapGestures`. **Positive control:** the same
test asserts the file it read contains `StrokeReader` — an empty offender list from the wrong file is not
evidence.

### 5.3 Device pass — a gate, VLL's own check (2026-09-12)
On the tablet, in note mode, with a finger:
- a **~2 mm stroke** renders as a short line, not as a dot at its end point;
- a **~1 cm line** starts under the point where the finger landed, not ~1.5 mm along;
- a **stab** (down-up) still makes a dot; the eraser dabs at touchdown;
- a second finger resting on the glass mid-stroke neither breaks the stroke nor draws.
Report it as seen, with a photo or screenshot of the 2 mm and 1 cm cases; a pure green is not this row.

## 6. Out of scope
Smoothing / bezier fitting, pressure, hover, stylus detection, palm rejection, and anything that changes
how a stroke is rasterised. The hardware/human share of "near paper" is real and is not this task.

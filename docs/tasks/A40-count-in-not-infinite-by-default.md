# A40 — The beat is a count-in by default; ∞ is opt-in

**Priority:** normal — small, but it changes what every player sees the first time · **Size:** XS ·
**Area:** `app/shared/.../stage` (`StageBeat.kt`). Lane: Mobile. **Studio: no change — see §3.**

VLL, 2026-08-23: *"the infinite counting should be disabled by default, the default behavior is just
2 bars, not infinite"*.

## 1. Where it stands, pinned

**The app has it backwards.** `StageBeat.kt:73`:

```kotlin
/** ∞ mode (studio parity — the ∞ loop toggle in the editor): ON → [toggle] keeps the beat running;
 *  OFF → it's an 8-beat count-in that self-stops. Defaults ON (VLL's preferred "keep it running").
 *  Affects the NEXT start; a run already going finishes as it started. Set it via the ∞ FAB. */
var continuous by mutableStateOf(true)
```

So today, tapping the metronome on the Stage starts a beat that **never stops** until you tap it
again. The comment cites an earlier steer of VLL's; he has now reversed it. The comment must change
with the value — a stale rationale in the source is how this drifts back.

Note the second half of that comment is also wrong now in spirit: it claims "studio parity", and the
studio defaults the other way (§3). After this change the claim becomes true.

**Nothing guards the default.** `BeatPhaseTest.kt` exercises `COUNT_IN_BEATS` and `CONTINUOUS_BEATS`
thoroughly, but no test asserts what `StageBeat` *starts* as. That absence is why a default could sit
wrong without a red test — so the fix has to add that assertion, not just flip the literal.

## 2. The change

1. `continuous` defaults to **`false`**. Tapping the metronome runs a count-in that self-stops.
2. Rewrite the KDoc: OFF is the default and is a count-in of **two bars**; ∞ is the opt-in
   keep-running mode, reached from the ∞ chip next to the metronome (`StageScreen.kt:1100-1102`).
3. **Do not persist it.** A player who wants ∞ turns it on for that session; opening the Stage again
   starts from the count-in. That matches the studio (`useBeat` is per-editor state) and keeps the
   Stage read-only (I12). If you think it should be sticky, raise it at the gate — do not add a
   preference key on your own initiative, because a persisted ∞ that survives a restart is exactly
   the "it just counts forever" complaint again, one setting deeper.

That is the whole task. It is deliberately two lines of code plus tests.

## 3. The studio is already correct — Web-Core has nothing to build

Recorded here so nobody "fixes" a compliant runtime, and so the gate can check it without re-deriving:

- `web/studio/src/useBeat.ts:69` — `useState(false)`: the ∞ toggle starts **off**.
- `Viewer.tsx:1070` — the play button calls `beat.start(beat.continuous)`, so a fresh editor starts a
  count-in.
- `beatPhase.ts:46` — `countInUnits(groups) = 2 * unitsPerBar(groups)`: **two bars, in every metre**
  (8 in 4/4, 6 in 3/4, 12 in 6/8, 14 in 3+4/8) since T86.
- `e2e/beat.spec.ts:238` asserts `beat-loop` is `aria-pressed="false"` on a fresh editor, and the test
  at `:208` asserts the count-in **stops itself**. The default is already guarded against regression.

Web-Core: no work. Do not open a task for this.

## 4. The count-in *length* on the app belongs to A35, not here

`COUNT_IN_BEATS = 8` (`CountIn.kt:25`) is two bars of a hard-coded 4/4, so it is right for 4/4 and
wrong for everything else — a 3/4 song counts eight beats instead of six. That is already **A35 §6**
("Count-in becomes 2 bars = `2 × unitsPerBar`"), which lands with the metre grid. Leave it alone in
A40: flipping a default and porting a metric grid are different reviews.

Sequencing: A40 is XS and touches `StageBeat.kt`; A35 touches `CountIn.kt` and the same controller.
Do **A40 first, standalone** — it rebases under A35 without a conflict worth the name. If you have
already started A35, still land A40 separately rather than folding it in; I want the default flip
reviewable on its own.

## 5. Acceptance criteria

- **The default is asserted directly**: a fresh `StageBeat()` has `continuous == false`. This is the
  regression guard the code lacks today; without it this task can silently undo itself.
- **The behaviour, not just the flag**: tapping the metronome on a fresh Stage starts a run of
  `COUNT_IN_BEATS`, and the controller reports **not running** once the count is over. Assert the
  self-stop by driving the phase past `COUNT_IN_BEATS * interval` — a flag check alone would pass
  even if `toggle` ignored the flag.
- ∞ ON still runs continuously, and the documented mid-run rule is unchanged: toggling ∞ affects the
  **next** start, not the run in flight. Keep that assertion if it exists; add it if it does not.
- The KDoc at `StageBeat.kt:73` no longer says "Defaults ON (VLL's preferred …)". A comment that
  contradicts the code is worse than no comment.
- `:shared:check` green. Remember `:shared:check` + the APK do **not** cover the iOS klib — build
  `:shared:compileKotlinIosSimulatorArm64` too if you touch anything shared.
- **Device check, one screenshot pair**: metronome tapped with ∞ off — the frame pulses, and a second
  shot after ~2 bars showing it dark again and the chip idle. Screenshots of the actual build, not of
  a previous run (this lane has re-used stale shots before).

## 6. Out of scope

- The metre-aware count-in length — **A35 §6**.
- Persisting ∞, and any new preference row in Parameters (A36's screen).
- An audible metronome. The Stage beat is silent and read-only (I12); nothing here changes that.
- The studio (§3).

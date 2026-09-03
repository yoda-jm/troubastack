# A67 — retire the 4/4 leftovers, and fix the comment that still claims an 8-beat count-in

**Lane:** mobile. **Size:** XS. **Status:** spec, not started. **After the gig.**
**Raised by:** VLL, 2026-09-03, asking whether the count-in counts **bars** rather than beats so the
metre and the bpm agree. **It does** — this task is only about the dead code that says otherwise.

## The state (verified, so nobody re-investigates)

The metre-aware path is live and correct: `meter` (proto field 12) reaches `BundleModel.kt:55`, becomes
a metric grid (`4/4→[1,1,1,1]`, `6/8→[3,3]`, additive `3+4/8→[3,4]`), and

```kotlin
fun countInUnits(groups: List<Int>): Int = 2 * unitsPerBar(groups)   // 4/4→8 · 3/4→6 · 6/8→12
```

is what production calls — `StageBeat.kt:107`. The downbeat is `tierOf(u, groups)`, not `% 4`.
`MeterGridTest` pins 3/4→6 and 6/8→12, and `web/studio/test/beat-phase.test.ts` pins the same on the
Studio side. An unset metre falls back to 4/4, so pre-T86 bundles beat unchanged.

## The leftovers

`CountIn.kt` still exports the A34-era 4/4 constants, now called **only from the old test file**:

| symbol | line | still used by |
|---|---|---|
| `BEATS_PER_BAR = 4` | `CountIn.kt:23` | `COUNT_IN_BEATS`, `isDownbeat` |
| `COUNT_IN_BEATS = 8` | `:26` | `BeatPhaseTest.kt` only |
| `isDownbeat(i) = i % BEATS_PER_BAR == 0` | `:162` | `BeatPhaseTest.kt:57` only |

**They are inert, and they have already done damage:** `StageBeat.kt:63` still describes *"an 8-beat
count-in"*, which is wrong for every metre except 4/4 and is exactly what would convince the next
reader that the `% 4` assumption still holds. A stale comment beside live code costs more than the dead
constants do.

## Work

1. **Fix `StageBeat.kt:63`** — say two bars in metric units, and point at `countInUnits`. Do this even
   if nothing else in this task is done.
2. **Remove `COUNT_IN_BEATS` and `isDownbeat`**, and rewrite the tests that used them against the grid
   (`countInUnits`, `tierOf`). `BeatPhaseTest` is the A34 contract test — keep what it proves about
   phase and drift, drop only the 4/4 assumptions.
3. **`BEATS_PER_BAR`**: if anything still needs a 4/4 default, it is `DEFAULT_GROUPS`, which already
   exists. Delete the constant rather than leaving a second way to say the same thing.

## Do not

- Do not change any timing behaviour. This is a naming-and-deletion task; the beat must sound and look
  identical afterwards. If a test's expected numbers change, something is wrong — stop and say so.
- Do not touch the `unitIntervalMs` tempo rule (T86 §4: irregular groups count units, `60000 / bpm`).

## Done when

- No production or test symbol assumes four beats to a bar.
- `StageBeat.kt`'s comments describe the metric count-in, not an 8-beat one.
- `:shared:testDebugUnitTest` green with the **same** count of assertions passing behaviourally —
  and a reviewer can see that no expected timing value moved.

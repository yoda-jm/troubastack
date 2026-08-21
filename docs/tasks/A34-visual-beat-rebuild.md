# A34 — Stage: make the visual beat actually visible (and optionally keep running)

**Priority:** high (VLL 2026-08-21, from the stage: *"the metronome beat … seems that it is not
working"*) · **Size:** M · **Area:** `app/shared/.../stage` (`CountIn.kt`, `StageScreen.kt`) +
commonTest. Replaces the A11 pulse; the studio sibling is **T85**.

## Why it looks broken — it very nearly is

A11 shipped a *count-in* (8 beats, self-stopping) and its acceptance asked for "8 visible pulses".
Reading `StageScreen.kt` `TempoChip`, that is not what it renders:

```kotlin
val active = beat >= 0                                   // TRUE for the whole run — never toggles
val dot = when { active && isDownbeat(beat) -> 12.dp     // downbeat
                 active -> 8.dp                          // off-beat
                 else -> 7.dp }                          // idle
```

1. **There is no per-beat event at all.** `active` is true from beat 0 to beat 7, so nothing turns on
   and off *per beat*. The dot changes size only when the bar changes.
2. **Off-beats are a 1 dp change** (8 dp vs the 7 dp idle). That is invisible. Effectively the
   performer sees **2 events in 8 beats** (the two downbeats) — which reads exactly as "not working".
3. The dot is always filled; A11 asked for "larger/filled vs outline", so the emphasis channel the
   spec intended was never implemented either.
4. **Timing accumulates drift**: `for (b …) { beat = b; delay(ms) }` adds the recomposition cost to
   every interval, and `60_000L / tempo` truncates (90 bpm → 666 ms, not 666.67).

**Why the tests didn't catch it** — and this is the part to fix structurally. `CountInTest` asserts
`countInIntervalMs` and `isDownbeat` only; the file says *"the animation itself is code-review +
screenshot"*. So the one property VLL cares about — **that a beat is visible** — was never expressed
as a testable claim. A pure interval function passing tells you nothing about whether anything blinks.

## How visual metronomes are actually built (what the field does)

- **Flash a large, high-contrast area — the frame/border — not a small dot.** Peripheral vision is
  driven by luminance and motion, not by a few dp of size. Commercial visual metronomes pulse the
  screen border or the whole meter precisely so it is catchable while you look at your instrument
  ([Korg visual metronome](https://www.audiotechnology.com/by-brand/korg/visual-metronome),
  [metronome-online](https://metronome-online.org/visual-metronome)).
- **Strong vs weak beats differ in *kind*, not degree** — fully lit on the downbeat, partially on the
  others, plus a distinct colour for beat 1 so you never lose your place.
- **Motion gives anticipation; a flash alone only reports the past.** The classic design
  ([US 4,649,794](https://image-ppubs.uspto.gov/dirsearch-public/print/downloadPdf/4649794)) moves an
  illuminated bar at constant rate and intensifies it *at* the beat, so the eye can predict the
  downbeat instead of reacting to it. For a running metronome this is the difference between usable
  and merely blinking.
- **Never schedule beat-by-beat.** Timers drift under load; the standard fix is to compute each
  beat's target time from a monotonic clock and schedule against it
  ([why timers drift](https://dev.to/kandz/why-javascript-timers-drift-building-a-high-precision-metronome-with-web-audio-api-c0a),
  [don't use setInterval](https://perfecttune.net/articles/why-your-metronome-should-not-use-setinterval.html)).

## Design (decided)

1. **A real transient per beat.** Each beat lights the indicator for
   `min(90 ms, interval × 0.35)` and then clears it *before* the next beat. On/off is the event; if
   the light never goes out, there is no beat.
2. **Emphasis by kind:** downbeat = filled + accent colour + full size; off-beat = outline/dim at a
   clearly smaller size. The two must be distinguishable at a glance in the dark, not by 4 dp.
3. **Peripheral channel: a thin edge pulse**, not a small chip dot alone. A ~3–4 dp bar along the
   page edge, lit on the beat. **Still no full-screen flash** — A11 was right about stage lighting,
   and an edge is what the field uses anyway.
4. **Anticipation (the part that makes it a metronome).** Between beats, sweep a subtle indicator
   along that edge at constant rate, intensified at the beat. Predictable beats are the whole point;
   a dot that blinks after the fact is a clock, not a metronome.
5. **Monotonic scheduling.** Compute beat *k*'s target as `start + k × intervalMs` (Double ms, no
   integer truncation) and drive the visual from `withFrameNanos`, so the phase is derived from the
   frame clock rather than accumulated `delay()` calls. No drift over a long run.
6. **Count-in vs continuous — both, explicitly.** Tap = the current 8-beat count-in (unchanged
   default). **Long-press = keep running** until tapped again or the page turns. VLL said
   "metronome", and A11 deliberately built only a count-in; make the continuous mode exist rather
   than leaving the word ambiguous. It stays silent and read-only either way.

## Acceptance criteria

- **The visible-beat claim becomes testable — this is the criterion that matters.** Extract a pure
  `beatPhase(elapsedMs, intervalMs, beats)` → `(beatIndex, lit: Boolean, emphasis)` in `CountIn.kt`,
  and unit-test the **sequence**: over one count-in at 120 bpm it yields exactly 8 lit→unlit
  transitions, each lit window ≤ interval × 0.35, downbeats emphasised at 0 and 4. A regression here
  must fail if someone again renders "always on".
- Off-beat and downbeat renderings differ in **fill/colour**, not only size; assert the emitted
  emphasis value, and include a screen recording in the handoff (the eyeball check is evidence *in
  addition to*, never *instead of*, the sequence test).
- No drift: with a stubbed clock, beat 200 at 120 bpm lands within ±5 ms of `start + 200 × 500 ms`.
  Interval maths is Double (`60_000.0 / tempo`); 90 bpm → 666.67 ms.
- Long-press runs continuously and self-stops on page turn or a second tap; tap still does exactly 8.
- Out-of-range tempo (outside 20..300) remains a no-op.
- `gradle` build + commonTest green; no new deps; nothing persisted (I12 read-only).

## Out of scope

- Audio click (deliberately silent — this is a stage).
- Meter/time-signature awareness beyond 4/4 (we still don't know the real meter; if we ever do, the
  emphasis rule generalises from `isDownbeat`).
- The studio version — **T85**.

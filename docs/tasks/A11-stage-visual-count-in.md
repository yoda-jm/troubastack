# A11 — Stage: optional visual count-in (a few silent beats at the song's tempo)

**Priority:** A-track, after A08 · **Size:** XS/S · **Area:** `app/shared` (stage)

## Context

A08 put ♩=N on the song's first page. The natural next inch: tap it and get a **silent,
visual count-in** — the drummer's "1, 2, 3, 4" for bands without one, or a tempo
reminder before the intro. Strictly optional, strictly visual (it's a stage; no audio,
ever), self-stopping.

**Design decisions (resolved):**
1. **Trigger:** the tempo chip in the A08 metadata strip becomes tappable when
   `tempo > 0` (add a subtle ♩ affordance). No tempo ⇒ no feature. No new chrome
   button.
2. **Behavior:** exactly **8 beats** (two bars of 4/4 — we don't know the meter, so
   don't pretend; 8 beats works as a count-in for practically everything) at
   `60_000/tempo` ms per beat, then stops itself. Tap again to restart; page turn or
   exit cancels.
3. **Visual:** a pulse that reads from the corner of the eye without covering music —
   a small filled circle in the strip that scales/flashes on each beat, beat 1 of each
   bar emphasized (larger/filled vs. outline). NO full-screen flash (stage lighting!).
4. **Read-only contract intact** (A04/I12): no state persisted, nothing written,
   pure UI.

## Changes

1. `stage/`: a `countInBeats(tempo): beatIntervalMs` + beat-index sequence as pure
   logic (unit-tested: interval math, 8-beat termination, tempo bounds — clamp to a
   sane 20..300 bpm, else ignore the tap); a Compose `LaunchedEffect`-driven pulse in
   the metadata strip, cancelled on dispose/page change.
2. commonTest for the pure parts; the animation is code-review + screenshot.

## Acceptance criteria

- On a song with tempo (demo: Black Hole Sun ♩=98, The Open Road ♩=92): tap the tempo
  chip → 8 visible pulses at the right rate (evidence: screen recording or a
  3-screenshot sequence), then the strip returns to normal. Songs without tempo show
  no affordance.
- Page-turn mid-count cancels cleanly; `:shared:check` + iOS klibs + `assembleDebug`
  green.

## Out of scope

- Audio click; configurable beat count/meter; persistent metronome (that's a different
  product); haptics (revisit on real-device feedback).

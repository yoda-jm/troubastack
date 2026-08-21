# T85 — Studio: the same visual beat, from the same contract as the app

**Priority:** normal — do **after A34**, and only if VLL still wants it once the app version is good
· **Size:** S/M · **Area:** `web/studio` (song editor / viewer chrome) + a shared timing contract.
VLL 2026-08-21: *"maybe I also want something similar in the studio."*

## Why this is a sibling, not a copy

A34 rebuilds the stage beat: a real per-beat transient, emphasis by kind, an edge pulse for
peripheral readability, motion for anticipation, and monotonic scheduling. Studio wants the same
*behaviour* on a different runtime (TypeScript/React vs Kotlin/Compose), which is exactly the shape
that produced a bug last time: **two implementations of one idea drift, and the drift is invisible
until someone compares them side by side.**

We already have the pattern for avoiding that — `glyphs.json` is generated once and consumed by TS
ink, the Go baker and the app, so "what a `cuica` looks like" has a single source of truth. Do the
same here for *what a beat is*.

## Design (decided)

1. **One contract, two renderers.** The beat *phase* function is the contract:
   `beatPhase(elapsedMs, intervalMs, beats) → { beatIndex, lit, emphasis }`. A34 defines it in
   `CountIn.kt`. T85 implements the identical function in TS **and pins both to a shared
   test-vector JSON** (`docs/contracts/beat-phase.vectors.json` or similar): a table of
   `(elapsedMs, intervalMs) → expected phase`, run by **both** the Kotlin commonTest and the studio
   test. That is the P205/T57 pattern — the vectors are the source of truth, so the two renderers
   cannot silently disagree about when a beat happens.
2. **Same visual language:** transient on/off per beat (never "always on"), downbeat distinguished by
   fill/colour rather than a few pixels of size, no full-screen flash.
3. **Where it lives in studio:** the editor is a desk tool, not a stage — the beat belongs near the
   song's tempo (the metadata strip), not overlaying the page. Same tap = count-in, long-press or a
   toggle = continuous.
4. **Timing:** drive from `requestAnimationFrame` with each beat's target computed as
   `start + k × intervalMs` — never `setInterval`/`setTimeout` per beat, which drifts under load
   ([why timers drift](https://dev.to/kandz/why-javascript-timers-drift-building-a-high-precision-metronome-with-web-audio-api-c0a)).
   If audio is ever added, schedule it on the Web Audio clock and derive the visual from the same
   timeline — but audio is out of scope here.

## Acceptance criteria

- The shared vector file exists and is executed by **both** suites; a deliberate off-by-one in the TS
  phase function fails the TS run against the same vectors that the Kotlin run passes (prove the
  vectors bind, red-first).
- Sequence test: at 120 bpm a count-in yields exactly 8 lit→unlit transitions with downbeats at 0
  and 4; lit window ≤ interval × 0.35.
- No drift: beat 200 lands within ±5 ms of `start + 200 × interval` against a stubbed clock.
- Testids for the control and the indicator; e2e covering tap → the indicator pulses and stops
  itself. Run the **dangling-testid sweep** and the **full** suite (isolated ports, T81).
- `tsc -b studio` clean.

## Out of scope

- Audio click in the studio (same reasoning as the stage: add it only on an explicit ask, and then
  from the audio clock).
- Tempo *editing* from the indicator, tap-tempo, or metronome settings UI.
- Any change to A34 once it has landed — if the contract needs to move, it moves in A34 first and
  the vectors are regenerated.

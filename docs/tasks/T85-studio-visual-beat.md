# T85 — Studio: the same visual beat, from the same contract as the app

**Priority:** high — **this goes FIRST**, before the Stage rebuild (A34) · **Size:** S/M · **Area:**
`web/studio` (song editor / viewer chrome) + the shared timing contract. VLL 2026-08-21: *"I would
like the studio version first to understand how it renders before committing to doing the change on
the concert mode."*

## Sequencing decision (VLL, 2026-08-21)

Studio leads and **owns the contract**; A34 ports the tuned result to the Stage. The reasoning is
sound: the studio is a desk tool where you can look at the thing, tweak it and look again, while the
stage is the high-stakes surface you don't want to iterate on. So the visual language gets settled
here, and A34 becomes a port rather than an experiment.

Two consequences, stated so they aren't discovered later:

- The `beatPhase` contract and its **test vectors are authored in this task**, in TS. A34 then
  implements the same function in Kotlin and runs the *same* vectors. (The original T85 draft had
  this the other way round; the inversion is the only change.)
- **Tuning does not transfer 1:1.** Desk (bright room, ~50 cm, mouse) and stage (dark, ~1 m, hands
  full) differ, so sizes, brightness and flash length will need a second pass on the device. What
  transfers is the *language* — transient per beat, emphasis by kind, edge placement, anticipation —
  not the numbers.

## Design confirmed by prototype

An interactive Stage emulation was built and reviewed with VLL:
<https://claude.ai/code/artifact/50e21132-b37f-46de-95cf-87f7a91d491d>

**VLL's verdict — the border pulse is the chosen direction:** *"I liked your border of the
presentation flashing/pulsing, feels like it is very visual and does not interfere with looking at
the page content."* So the edge rail is now a **decision, not an option**: the beat lives on the page
border, never over the content and never as a full-screen flash.

## Why this is a sibling, not a copy

A34 rebuilds the stage beat: a real per-beat transient, emphasis by kind, an edge pulse for
peripheral readability, motion for anticipation, and monotonic scheduling. Studio wants the same
*behaviour* on a different runtime (TypeScript/React vs Kotlin/Compose), which is exactly the shape
that produced a bug last time: **two implementations of one idea drift, and the drift is invisible
until someone compares them side by side.**

We already have the pattern for avoiding that — `glyphs.json` is generated once and consumed by TS
ink, the Go baker and the app, so "what a `cuica` looks like" has a single source of truth. Do the
same here for *what a beat is*.

## The visual, settled on the prototype (VLL, 2026-08-21)

Prototyped and reviewed together at
<https://claude.ai/code/artifact/50e21132-b37f-46de-95cf-87f7a91d491d>. These are decisions, not
suggestions — implement these values.

**Form — a rounded frame pulsing around the page.** The beat is a frame in the padding *around* the
sheet, on **all four sides**, with the same corner radius as the page container. It never overlaps
the music and it is never a full-screen flash. VLL: *"very visual and does not interfere with looking
at the page content."*

**Envelope — attack + decay, never a square wave.** The first cut flashed a hard 90 ms on/off and
read as a strobe (*"pulsating too quickly"*). The pulse is now full at the beat and eased away:

```
decay  = min(220 ms, interval × 0.75)      // clamped so it always clears before the next beat
env(t) = t >= 1 ? 0 : (1 - t)²             // t = msSinceBeat / decay
width  = BASE × (0.45 + 0.55 × env)        // BASE = 9 px  (dp in the app)
opacity= env × 0.92
glow   = 0 0 (width × 2.4)px rgba(colour, env × 0.55)
```

**Emphasis — hue at EQUAL width.** Every beat draws the same frame weight; the downbeat differs only
in colour, so the geometry never moves:

| beat | colour |
|---|---|
| downbeat (every 4th) | **amber `#ffb02e`** |
| off-beat | **aqua `#3ee0d4`** |

Two reasons this pairing is fixed rather than taste. Grey read as *switched off* rather than as a
beat — both beats need a real colour. And the pair is **warm against cool**, so the distinction rides
the blue-yellow axis and survives red-green colour deficiency (~1 in 12 men — likely someone in the
band). Do not substitute a red/green pair.

**Weight — BASE 9 px.** VLL asked for it wider than the first 6 px cut. Keep it a single constant so
it is one edit to re-tune; the app will almost certainly want a larger value again at stage distance.

**Sweep — not in v1.** A travelling marker gives anticipation, but it is constant peripheral motion
for a whole song; it was toggled off by default in the prototype and VLL did not pick it. Leave it
out. Revisit only on request, and then behind a preference.

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

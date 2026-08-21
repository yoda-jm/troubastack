/**
 * T85 — the visual-beat CONTRACT, shared with the app (A34).
 *
 * `beatPhase` answers one question — *when is a beat, and what kind* — as a pure
 * function of elapsed time. It is deliberately free of any rendering: the studio
 * (here) and the Stage (A34, Kotlin) each draw the beat their own way, but they must
 * agree on the timeline. So this function is pinned to a shared table of test vectors
 * (`docs/contracts/beat-phase.vectors.json`) that BOTH runtimes execute — the same
 * `glyphs.json` / view-resolution.vectors pattern. A34 implements the identical
 * function in Kotlin and runs the same vectors; if the two ever drift, a vector fails.
 *
 * What does NOT live here: the visual envelope (frame width, colour, opacity, glow).
 * Those are tuning, and tuning does not transfer 1:1 between a lit desk at 50 cm and a
 * dark stage at arm's length — see `beatFrame.ts` for the studio's numbers.
 */

export interface BeatPhase {
  /** Which beat we are in: `floor(elapsed / interval)`. Counts on past `beats`. */
  beatIndex: number;
  /** True during the on-portion of a beat that is still within the count — the
   *  discrete "a beat is happening" signal a simple on/off renderer (and the
   *  sequence test) keys on. Distinct from the visual fade, which lasts longer. */
  lit: boolean;
  /** True for the whole of a downbeat (every 4th beat) — the emphasis channel.
   *  Held across the beat's fade, not just the lit window, so the frame keeps the
   *  downbeat colour while it eases away. */
  emphasis: boolean;
}

/** Beats per bar — downbeats fall on beat 0, 4, 8, … */
export const BEATS_PER_BAR = 4;

/** A count-in is two bars. Tap = count-in; continuous mode passes `Infinity`. */
export const COUNT_IN_BEATS = BEATS_PER_BAR * 2;

/** ms between beats for a tempo in bpm. Kept as a float — the app's original bug was
 *  truncating `60000 / 90` to 666 instead of 666.67, which drifts a bar every ~40 s. */
export function intervalMs(bpm: number): number {
  return 60000 / bpm;
}

/** How long the frame takes to fade back to nothing after a beat. Clamped so it always
 *  clears before the next beat even at slow tempos. */
export function decayMs(interval: number): number {
  return Math.min(220, interval * 0.75);
}

/** The `lit` window: the on-portion of a beat. Capped at 30% of the interval so the
 *  discrete signal is a transient at every tempo (the sequence test asserts ≤ 35%). */
export function litWindowMs(interval: number): number {
  return Math.min(decayMs(interval), interval * 0.3);
}

/**
 * The phase at `elapsedMs` for a count of `beats` at `intervalMs` spacing. Pure; reads
 * the clock only through its argument, so the caller (a rAF loop, or a stubbed clock in
 * a test) owns time. `beats = Infinity` is continuous mode.
 */
export function beatPhase(elapsedMs: number, intervalMs: number, beats: number): BeatPhase {
  const beatIndex = Math.floor(elapsedMs / intervalMs);
  const active = elapsedMs >= 0 && beatIndex < beats;
  const msSinceBeat = elapsedMs - beatIndex * intervalMs;
  const lit = active && msSinceBeat < litWindowMs(intervalMs);
  const emphasis = active && beatIndex % BEATS_PER_BAR === 0;
  return { beatIndex, lit, emphasis };
}

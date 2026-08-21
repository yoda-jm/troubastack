/**
 * T85 — the studio's VISUAL tuning for the beat frame.
 *
 * These are the numbers Fable and VLL settled on the prototype
 * (https://claude.ai/code/artifact/50e21132-b37f-46de-95cf-87f7a91d491d). They live here,
 * NOT in the shared `beatPhase` contract, because tuning does not transfer 1:1 to the
 * stage (dark room, arm's length, hands full) — A34 will re-tune while keeping the same
 * *language*: a transient per beat, emphasis by hue at equal width, an edge frame, an
 * attack+decay envelope (never a square wave).
 */
import { BEATS_PER_BAR, decayMs } from "./beatPhase";

/** Frame weight at the peak of a beat. One constant so re-tuning is one edit; the app
 *  will want a larger value again at stage distance. VLL asked for wider than the first 6 px. */
export const BEAT_BASE_PX = 9;

/** Emphasis is by HUE at equal width, so the geometry never moves. The pair is warm
 *  against cool on purpose: the distinction rides the blue–yellow axis and survives
 *  red-green colour deficiency (~1 in 12 men). Do not swap in a red/green pair. */
export const DOWNBEAT_COLOR = "#ffb02e"; // amber — every 4th beat (the downbeat)
export const OFFBEAT_COLOR = "#3ee0d4"; // aqua — every other beat (grey reads as "off")

export interface BeatFrameStyle {
  /** px */
  borderWidth: number;
  borderColor: string;
  opacity: number;
  boxShadow: string;
}

function withAlpha(hex: string, alpha: number): string {
  const a = Math.max(0, Math.min(255, Math.round(alpha * 255)));
  return hex + a.toString(16).padStart(2, "0");
}

/**
 * The frame style at `elapsedMs`, or `null` when the frame should be invisible (before
 * start, between pulses, or after the count-in has finished). Envelope is `(1 - t)²` over
 * a decay clamped to clear before the next beat — a hard on/off read as a strobe
 * ("pulsating too quickly"), so the pulse is full at the beat and eased away.
 */
export function beatFrameStyle(
  elapsedMs: number,
  intervalMs: number,
  beats: number,
): BeatFrameStyle | null {
  const beatIndex = Math.floor(elapsedMs / intervalMs);
  const active = elapsedMs >= 0 && beatIndex < beats;
  if (!active) return null;

  const decay = decayMs(intervalMs);
  const msSinceBeat = elapsedMs - beatIndex * intervalMs;
  if (msSinceBeat >= decay) return null; // between pulses — dark

  const env = (1 - msSinceBeat / decay) ** 2;
  const width = BEAT_BASE_PX * (0.45 + 0.55 * env);
  const color = beatIndex % BEATS_PER_BAR === 0 ? DOWNBEAT_COLOR : OFFBEAT_COLOR;
  return {
    borderWidth: width,
    borderColor: withAlpha(color, env * 0.92),
    opacity: 1,
    boxShadow: `0 0 ${(width * 2.4).toFixed(2)}px ${withAlpha(color, env * 0.55)}`,
  };
}

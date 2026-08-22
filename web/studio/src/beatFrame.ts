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
import { beatPhase, decayMs, DEFAULT_GROUPS, TIER2_MUTE_MS } from "./beatPhase";

/** Frame weight at the peak of a beat. One constant so re-tuning is one edit; the app
 *  will want a larger value again at stage distance. VLL asked for wider than the first 6 px. */
export const BEAT_BASE_PX = 9;

/** Colour per TIER (T86): emphasis is by HUE at equal width, so the geometry never moves.
 *  bar=amber, felt-pulse=aqua — warm-vs-cool, so the distinction rides the blue–yellow axis and
 *  survives red-green colour deficiency (do not swap in a red/green pair). Free subdivisions are a
 *  receding grey — fine as the THIRD rank (VLL rejected grey as an off-BEAT, where it read as "off"). */
export const TIER_COLORS = ["#ffb02e", "#3ee0d4", "#6b7a90"] as const; // bar · felt pulse · subdivision

export interface BeatFrameStyle {
  /** px */
  borderWidth: number;
  borderColor: string;
  opacity: number;
  boxShadow: string;
}

// T88: the frame's geometry (Edges, frameBox) moved to ./layout so the icon palette and the e2e
// unit tests can share it. This module keeps the beat's VISUAL tuning only.

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
  groups: readonly number[] = DEFAULT_GROUPS,
): BeatFrameStyle | null {
  const beatIndex = Math.floor(elapsedMs / intervalMs);
  const active = elapsedMs >= 0 && beatIndex < beats;
  if (!active) return null;

  const { tier } = beatPhase(elapsedMs, intervalMs, beats, groups);
  // A free subdivision that would strobe is left dark — the grid still ticks, it just doesn't light.
  if (tier === 2 && intervalMs < TIER2_MUTE_MS) return null;

  const decay = decayMs(intervalMs);
  const msSinceBeat = elapsedMs - beatIndex * intervalMs;
  if (msSinceBeat >= decay) return null; // between units — dark

  const env = (1 - msSinceBeat / decay) ** 2;
  const width = BEAT_BASE_PX * (0.45 + 0.55 * env);
  const color = TIER_COLORS[tier];
  // Tier 2 recedes: ~half the opacity and NO glow — hue alone is too weak a rank signal at speed,
  // and a third *width* would break the equal-width rule.
  const peakAlpha = tier === 2 ? 0.45 : 0.92;
  return {
    borderWidth: width,
    borderColor: withAlpha(color, env * peakAlpha),
    opacity: 1,
    boxShadow: tier === 2 ? "none" : `0 0 ${(width * 2.4).toFixed(2)}px ${withAlpha(color, env * 0.55)}`,
  };
}

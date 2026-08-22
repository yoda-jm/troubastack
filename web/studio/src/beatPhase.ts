/**
 * T85/T86 — the visual-beat CONTRACT, shared with the app (A35).
 *
 * `beatPhase` answers *when is a beat, and what kind* as a pure function of elapsed time,
 * free of any rendering — the studio (here) and the Stage (Kotlin) draw it differently but
 * must agree on the timeline. It is pinned to a shared vector table
 * (`docs/contracts/beat-phase.vectors.json`) that BOTH runtimes run.
 *
 * T86 makes the beat follow the song's METRE. A metre is its GROUP LENGTHS in metric units
 * (4/4→[1,1,1,1], 6/8→[3,3], additive 3+4/8→[3,4]); from the groups every unit gets a TIER:
 *   0 — bar       (unit 0)                → the downbeat
 *   1 — felt pulse (a group start ≠ 0)    → the other pulses you feel
 *   2 — free subdivision (everything else) → the eighths inside a compound group
 * `emphasis` stays `tier === 0`, so every pre-T86 4/4 vector passes untouched.
 *
 * The clock ticks on the UNIT, not the pulse — that is what makes unequal groups (3+4/8)
 * work, since the eighths are evenly spaced even when the pulses are not. The caller passes
 * the unit interval (see `unitIntervalMs`).
 */

export interface BeatPhase {
  /** Which UNIT we are in: `floor(elapsed / unitInterval)`. Counts on past `beats`. */
  beatIndex: number;
  /** True during the on-portion of a unit within the count — the discrete "a beat happened"
   *  signal. Tier-2 units are muted below the strobe threshold (see TIER2_MUTE_MS). */
  lit: boolean;
  /** 0 bar · 1 felt pulse (group start) · 2 free subdivision. */
  tier: 0 | 1 | 2;
  /** `tier === 0` — held across the unit's fade so the frame keeps the downbeat colour. */
  emphasis: boolean;
}

/** 4/4 — the grid an unset metre uses, so today's songs beat exactly as before. */
export const DEFAULT_GROUPS: readonly number[] = [1, 1, 1, 1];

/** Below this unit interval, tier-2 (free subdivision) units are muted — they read as a
 *  strobe rather than texture. Same complaint as T85's "pulsating too quickly", one level down. */
export const TIER2_MUTE_MS = 130;

/** Total metric units in a bar. */
export function unitsPerBar(groups: readonly number[]): number {
  return groups.reduce((a, b) => a + b, 0);
}

/** A count-in is two bars, measured in units (4/4→8, 3/4→6, 6/8→12 units = 4 felt pulses). */
export function countInUnits(groups: readonly number[]): number {
  return 2 * unitsPerBar(groups);
}

/** The tier of unit `u` within a bar, given the group lengths. */
function tierOf(u: number, groups: readonly number[]): 0 | 1 | 2 {
  if (u === 0) return 0; // the bar
  let start = 0;
  for (const g of groups) {
    if (u === start) return 1; // a group start that isn't unit 0 → a felt pulse
    start += g;
  }
  return 2; // between group starts → a free subdivision
}

/** How long the frame takes to fade back to nothing after a beat, clamped to clear before
 *  the next unit even at slow tempos. */
export function decayMs(interval: number): number {
  return Math.min(220, interval * 0.75);
}

/** The `lit` window: the on-portion of a unit, capped at 30% of the interval so the discrete
 *  signal is a transient at every tempo (the sequence test asserts ≤ 35%). */
export function litWindowMs(interval: number): number {
  return Math.min(decayMs(interval), interval * 0.3);
}

/** ms per PULSE for a tempo in bpm (float — truncating 60000/90 to 666 drifts a bar / ~40 s). */
export function intervalMs(bpm: number): number {
  return 60000 / bpm;
}

/** ms per UNIT for a tempo and metre. Uniform groups: the pulse is the group, so a unit is
 *  `pulse / groups[0]` (4/4→quarter, 6/8→eighth). Irregular groups (3+4/8) have no single
 *  pulse length, so tempo counts UNITS: `60000 / bpm`. See T86 §4. */
export function unitIntervalMs(bpm: number, groups: readonly number[]): number {
  const perPulse = 60000 / bpm;
  const uniform = groups.every((g) => g === groups[0]);
  return uniform ? perPulse / groups[0] : perPulse;
}

/** The note the tempo names: ♩ simple · ♩. compound (groups of 3) · ♪ irregular-additive. */
export type TempoUnit = "quarter" | "dotted-quarter" | "eighth";
export function tempoUnit(groups: readonly number[]): TempoUnit {
  const uniform = groups.every((g) => g === groups[0]);
  if (!uniform) return "eighth"; // irregular additive → no single pulse length
  return groups[0] === 3 ? "dotted-quarter" : "quarter";
}

const METER_DENOMS = new Set([1, 2, 4, 8, 16]);

/**
 * Resolve a metre string to its group lengths — the TS mirror of core `app.ParseMeter`
 * (both are pinned to the same behaviour; the studio and the server must agree). LENIENT:
 * anything malformed returns the 4/4 default rather than throwing, so a typo never breaks
 * the beat. Additive literal · compound (n%3==0 && n>3) → n/3 threes · simple → n ones.
 */
export function meterGroups(meter: string | null | undefined): number[] {
  const s = (meter ?? "").trim();
  if (s === "") return [...DEFAULT_GROUPS];
  const parts = s.split("/");
  if (parts.length !== 2) return [...DEFAULT_GROUPS];
  const d = Number(parts[1].trim());
  if (!Number.isInteger(d) || !METER_DENOMS.has(d)) return [...DEFAULT_GROUPS];
  const num = parts[0].trim();
  let groups: number[];
  if (num.includes("+")) {
    groups = num.split("+").map((p) => Number(p.trim()));
  } else {
    const n = Number(num);
    if (!Number.isInteger(n) || n < 1 || n > 32) return [...DEFAULT_GROUPS];
    groups = n % 3 === 0 && n > 3 ? Array<number>(n / 3).fill(3) : Array<number>(n).fill(1);
  }
  if (groups.length === 0 || groups.length > 16) return [...DEFAULT_GROUPS];
  let sum = 0;
  for (const g of groups) {
    if (!Number.isInteger(g) || g < 1 || g > 32) return [...DEFAULT_GROUPS];
    sum += g;
  }
  if (sum > 64) return [...DEFAULT_GROUPS];
  return groups;
}

/**
 * The phase at `elapsedMs`, `intervalMs` per UNIT, for a count of `beats` units under the
 * grid `groups`. Pure; the caller owns time. `beats = Infinity` is continuous mode; `groups`
 * defaults to 4/4 so pre-T86 callers and vectors are unchanged.
 */
export function beatPhase(
  elapsedMs: number,
  intervalMs: number,
  beats: number,
  groups: readonly number[] = DEFAULT_GROUPS,
): BeatPhase {
  const beatIndex = Math.floor(elapsedMs / intervalMs);
  const active = elapsedMs >= 0 && beatIndex < beats;
  const perBar = unitsPerBar(groups);
  const u = ((beatIndex % perBar) + perBar) % perBar;
  const tier = tierOf(u, groups);
  const msSinceBeat = elapsedMs - beatIndex * intervalMs;
  const baseLit = active && msSinceBeat < litWindowMs(intervalMs);
  // Tier-2 units go dark below the strobe threshold; the bar and felt pulses always light.
  const lit = baseLit && !(tier === 2 && intervalMs < TIER2_MUTE_MS);
  const emphasis = active && tier === 0;
  return { beatIndex, lit, tier, emphasis };
}

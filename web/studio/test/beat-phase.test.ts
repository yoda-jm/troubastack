// T110(c) — the beat-phase CONTRACT is a shared-vector check that does not need a browser. Moved here
// from e2e/beat.spec.ts verbatim (Playwright test()->vitest it()); the studio UI tests stay in e2e.
import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { beatPhase, intervalMs, countInUnits, meterGroups, DEFAULT_GROUPS } from "../src/beatPhase";

describe("beatPhase / meterGroups contract", () => {

interface Vector {
  elapsedMs?: number;
  intervalMs?: number;
  beats?: number;
  groups?: number[];
  beatIndex?: number;
  lit?: boolean;
  tier?: 0 | 1 | 2;
  emphasis?: boolean;
  _?: string; // a descriptive comment row (skipped)
}
const vectors: { cases: Vector[] } = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../../../docs/contracts/beat-phase.vectors.json", import.meta.url)),
    "utf8",
  ),
);

// ===========================================================================
// Contract — the shared beat-phase vectors (no browser).
// ===========================================================================
  it("beat contract: the studio beatPhase matches every shared vector (T85/T86)", () => {
  const real = vectors.cases.filter((c) => c.elapsedMs !== undefined);
  expect(real.length).toBeGreaterThanOrEqual(40);
  for (const c of real) {
    const p = beatPhase(c.elapsedMs!, c.intervalMs!, c.beats!, c.groups);
    const got: Record<string, unknown> = { beatIndex: p.beatIndex, lit: p.lit, emphasis: p.emphasis };
    const want: Record<string, unknown> = { beatIndex: c.beatIndex, lit: c.lit, emphasis: c.emphasis };
    if (c.tier !== undefined) {
      got.tier = p.tier;
      want.tier = c.tier;
    }
    expect(got, `elapsed=${c.elapsedMs} interval=${c.intervalMs} groups=${c.groups ?? "4/4"}`).toEqual(
      want,
    );
  }
  // At least one 4/4 case carries no tier (the untouched-backward-compat proof) and one metre case does.
  expect(real.some((c) => c.tier === undefined)).toBe(true);
  expect(real.some((c) => c.groups && c.tier !== undefined)).toBe(true);
});

  it("beat contract: a 120bpm 4/4 count-in is 8 transient beats, downbeats at 0 and 4 (T85)", () => {
  const interval = intervalMs(120); // 500 ms
  const beats = countInUnits(DEFAULT_GROUPS); // 8
  // Count lit→unlit transitions across the whole count-in: one per beat, so exactly 8.
  let transitions = 0;
  let prevLit = false;
  for (let e = 0; e <= beats * interval + 5; e++) {
    const lit = beatPhase(e, interval, beats).lit;
    if (lit && !prevLit) transitions++;
    prevLit = lit;
  }
  expect(transitions).toBe(8);

  // Per-beat lit window is a transient (≤ 35% of the interval), and downbeats land on 0 and 4.
  let maxLitMs = 0;
  const downbeats: number[] = [];
  for (let b = 0; b < beats; b++) {
    let litMs = 0;
    for (let e = b * interval; e < (b + 1) * interval; e++) {
      if (beatPhase(e, interval, beats).lit) litMs++;
    }
    maxLitMs = Math.max(maxLitMs, litMs);
    if (beatPhase(b * interval, interval, beats).emphasis) downbeats.push(b);
  }
  expect(maxLitMs).toBeLessThanOrEqual(interval * 0.35);
  expect(downbeats).toEqual([0, 4]);
});

  it("beat contract: no drift — beat 200 lands at 200×interval, not accumulated (T85)", () => {
  // A monotonic-clock reader has no accumulation error; the app's original bug (chained
  // delays + truncated interval) would land beat 200 tens of ms early. Stub the clock in
  // 1 ms steps around the target and find where beat 200 begins.
  for (const bpm of [120, 90]) {
    const interval = intervalMs(bpm);
    const target = 200 * interval;
    let onset = -1;
    for (let e = Math.floor(target) - 10; e <= Math.ceil(target) + 10; e++) {
      if (beatPhase(e, interval, 1e9).beatIndex >= 200) {
        onset = e;
        break;
      }
    }
    expect(Math.abs(onset - target), `bpm=${bpm}`).toBeLessThanOrEqual(5);
  }
});

  it("beat contract: the count-in is two bars in units, every metre (T86)", () => {
  expect(countInUnits([1, 1, 1, 1])).toBe(8); // 4/4
  expect(countInUnits([1, 1, 1])).toBe(6); // 3/4
  expect(countInUnits([3, 3])).toBe(12); // 6/8 — 12 units = 4 felt pulses
  expect(countInUnits([3, 4])).toBe(14); // 3+4/8
  expect(countInUnits(DEFAULT_GROUPS)).toBe(8); // unset = 4/4, the no-regression case
});

  it("beat contract: 3/4 puts a downbeat on unit 3 — the case the old %4 rule got wrong (T86)", () => {
  const g = [1, 1, 1];
  const interval = intervalMs(120); // 500 ms/quarter
  const downbeats: number[] = [];
  for (let u = 0; u < 6; u++) {
    if (beatPhase(u * interval, interval, 6, g).emphasis) downbeats.push(u);
  }
  expect(downbeats).toEqual([0, 3]); // not [0, 4]
});

  it("beat contract: tier-2 subdivisions mute below 130 ms/unit, and light above (T86)", () => {
  const g = [3, 3]; // 6/8 — unit 1 is a free subdivision (tier 2)
  // At the onset of unit 1 (a tier-2 unit): lit above the threshold, dark below it.
  expect(beatPhase(200, 200, 12, g)).toMatchObject({ tier: 2, lit: true }); // 200 ms/unit
  expect(beatPhase(100, 100, 12, g)).toMatchObject({ tier: 2, lit: false }); // 100 ms/unit — muted
  // The bar and felt pulses always light, even below the threshold.
  expect(beatPhase(0, 100, 12, g)).toMatchObject({ tier: 0, lit: true });
  expect(beatPhase(300, 100, 12, g)).toMatchObject({ tier: 1, lit: true });
});

  it("beat contract: meterGroups mirrors the core parser (T86)", () => {
  const table: Record<string, number[]> = {
    "4/4": [1, 1, 1, 1],
    "3/4": [1, 1, 1],
    "2/2": [1, 1],
    "6/8": [3, 3],
    "9/8": [3, 3, 3],
    "12/8": [3, 3, 3, 3],
    "5/4": [1, 1, 1, 1, 1],
    "3/8": [1, 1, 1],
    "3+2/8": [3, 2],
    "3+4/8": [3, 4],
    "2+2+3/8": [2, 2, 3],
    " 6 / 8 ": [3, 3],
  };
  for (const [m, want] of Object.entries(table)) {
    expect(meterGroups(m), `meterGroups(${JSON.stringify(m)})`).toEqual(want);
  }
  // Malformed → the 4/4 default (never throws).
  for (const bad of ["", "x/y", "4/5", "0/4", "33/4", "3+0/8", "-3/4", "4/4/4", null, undefined]) {
    expect(meterGroups(bad), `meterGroups(${JSON.stringify(bad)})`).toEqual([1, 1, 1, 1]);
  }
});

// ===========================================================================
});

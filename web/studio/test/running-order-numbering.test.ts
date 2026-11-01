import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { runningOrderNumbers, mainSetCountLabel, type RunningOrderEntry } from "../src/runningOrder";

// TroubaStudio reads the CANONICAL shared contract itself (T158) — not a hand-transcribed copy — so the
// same table that guards TroubaStage (Kotlin) and TroubaCore (Go) guards Studio, and the three cannot
// silently diverge. The mid-list intermission / on-call cases are the teeth: a "number every entry"
// implementation makes the following song read one too high and reddens here.
const CONTRACT = fileURLToPath(
  new URL("../../../docs/contracts/running-order-numbering.vectors.json", import.meta.url),
);

type VectorFile = {
  cases: { name: string; entries: RunningOrderEntry[]; expected: (number | null)[] }[];
};

describe("T158 running-order numbering — shared contract", () => {
  const vf = JSON.parse(readFileSync(CONTRACT, "utf8")) as VectorFile;

  it("the contract has cases (never passes vacuously)", () => {
    expect(vf.cases.length).toBeGreaterThan(0);
  });

  for (const c of vf.cases) {
    it(c.name, () => {
      expect(runningOrderNumbers(c.entries)).toEqual(c.expected);
    });
  }
});

// T206 follow-up — the "Main set" count line: songs only, intermissions counted separately in Studio's own
// word, both pluralised, the intermission part dropped when there are none. The bug it fixes: the count used
// the raw main length, so the demo concert (4 songs + 1 intermission) read "5 songs".
describe("Main set count label", () => {
  const song = { kind: "song" };
  const interm = { kind: "intermission" };

  it("songs only — singular, plural, zero, n", () => {
    expect(mainSetCountLabel([])).toBe("0 songs");
    expect(mainSetCountLabel([song])).toBe("1 song");
    expect(mainSetCountLabel([song, song])).toBe("2 songs");
    expect(mainSetCountLabel([song, song, song, song])).toBe("4 songs");
  });

  it("intermissions counted separately and pluralised; dropped when zero", () => {
    // the demo concert: 4 songs + 1 intermission (was wrongly "5 songs")
    expect(mainSetCountLabel([song, song, song, song, interm])).toBe("4 songs · 1 intermission");
    expect(mainSetCountLabel([song, song, interm, interm])).toBe("2 songs · 2 intermissions");
    expect(mainSetCountLabel([song, interm])).toBe("1 song · 1 intermission");
    expect(mainSetCountLabel([interm])).toBe("0 songs · 1 intermission");
  });

  it("a legacy entry with no kind reads as a song", () => {
    expect(mainSetCountLabel([{}, song])).toBe("2 songs");
  });
});

import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { runningOrderNumbers, type RunningOrderEntry } from "../src/runningOrder";

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

/**
 * T92 — pin the metre parser with the SAME vectors Go `app.ParseMeter` runs, so the studio
 * (`meterGroups`) and the server — and, from A35, Kotlin — cannot silently disagree about a metre's
 * groups. The parser is lenient: a malformed metre is treated as UNSET, which the beat reads as 4/4.
 *
 * Per the file's `_comment`, `groups: null` means "unset", and each runtime asserts that in its own
 * idiom: `meterGroups` expresses unset as the 4/4 default `[1,1,1,1]` — it never returns null. A
 * non-null `groups` is the exact grouping. Sibling of `beat.spec.ts`, which pins `beatPhase` the same
 * way; both import from the `tsconfig.contract` compile unit.
 */
import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { meterGroups, DEFAULT_GROUPS } from "../src/beatPhase";

interface MeterCase {
  meter: string;
  groups: number[] | null;
  _?: string;
}
const doc: { cases: MeterCase[] } = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../../../docs/contracts/meter-groups.vectors.json", import.meta.url)),
    "utf8",
  ),
);

test("meterGroups matches the shared metre vectors (T92)", () => {
  let valid = 0;
  let unset = 0;
  for (const c of doc.cases) {
    if (c.groups === null) {
      unset++;
      // "unset" in TS is the 4/4 default; meterGroups never returns null (see the file's _comment).
      expect(meterGroups(c.meter), `${JSON.stringify(c.meter)} → unset (4/4 default)`).toEqual([
        ...DEFAULT_GROUPS,
      ]);
    } else {
      valid++;
      expect(meterGroups(c.meter), `${JSON.stringify(c.meter)}`).toEqual(c.groups);
    }
  }
  // A truncated or half-written file must fail loudly rather than passing fewer cases in silence.
  expect(valid, "valid cases").toBeGreaterThanOrEqual(13);
  expect(unset, "malformed cases").toBeGreaterThanOrEqual(18);
});

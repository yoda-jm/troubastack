// T138 ⟨R1⟩ — the TS lane of the shared default-file contract. Reads the SAME canonical vectors the Go
// lane runs (docs/contracts/default-file.vectors.json) directly, so the two cannot describe "default"
// differently: change either lane's predicate without changing the vectors and that lane goes red. (Both
// lanes reading the one file is stronger than a mirror+diff — there is no second copy to drift.)
import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { defaultFile } from "../src/defaultFile";

type Case = {
  name: string;
  files: { filename: string; contentType: string; displayOrder: number }[];
  expected: string | null;
};

const vectors: { cases: Case[] } = JSON.parse(
  readFileSync(fileURLToPath(new URL("../../../docs/contracts/default-file.vectors.json", import.meta.url)), "utf8"),
);

describe("default-file contract (T138 ⟨R1⟩)", () => {
  it("has cases", () => expect(vectors.cases.length).toBeGreaterThan(0));
  for (const c of vectors.cases) {
    it(c.name, () => {
      expect(defaultFile(c.files)?.filename ?? null).toEqual(c.expected);
    });
  }
});

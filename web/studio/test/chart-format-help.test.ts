import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

// T166 — the editor's "Chart format" help must document every directive the engine accepts.
//
// It documented NONE of them. The measurable consequence, across a real 178-file library: not one chart used
// a header directive, and the only marker family present was `{sot}`/`{eot}` — the one directive the UI
// happens to name, in its tablature hint. An author cannot use what nothing tells them exists.
//
// The vocabulary lives in ONE place, docs/contracts/chart-directives.json, because a list of directives
// hand-written in a second language is the shape that rots silently. The Go side asserts the ENGINE matches
// that file (and that no undeclared directive regex exists); this asserts the HELP does. Add a directive to
// the contract and both sides redden until they learn it.
//
// Same path convention as running-order-numbering.test.ts, which reads its contract the same way.
const CONTRACT = fileURLToPath(new URL("../../../docs/contracts/chart-directives.json", import.meta.url));
const EDITOR = fileURLToPath(new URL("../src/pages/song-editor/ChartEditor.tsx", import.meta.url));

type Contract = {
  markers: { canonical: string; alias: string; what: string }[];
  header: { key: string; what: string }[];
};

describe("T166 chart-format help — every directive the engine accepts is documented", () => {
  const contract = JSON.parse(readFileSync(CONTRACT, "utf8")) as Contract;
  const source = readFileSync(EDITOR, "utf8");

  // The `Chart format` block only — so a directive merely MENTIONED in a code comment or an unrelated hint
  // elsewhere in the file cannot make this pass. It is the help a musician opens that has to say it.
  const help = (() => {
    const start = source.indexOf("<summary>Chart format</summary>");
    expect(start, "the Chart format block must exist").toBeGreaterThan(-1);
    const end = source.indexOf("</details>", start);
    expect(end, "the Chart format block must be closed").toBeGreaterThan(start);
    return source.slice(start, end);
  })();

  it("the contract lists directives (never passes vacuously)", () => {
    expect(contract.markers.length).toBeGreaterThan(0);
    expect(contract.header.length).toBeGreaterThan(0);
  });

  it.each(contract.markers)("documents the $canonical marker and its $alias alias", (m) => {
    expect(help, `{${m.canonical}} is accepted by the engine but absent from the help`).toContain(
      `{${m.canonical}}`,
    );
    expect(help, `{${m.alias}} is accepted by the engine but absent from the help`).toContain(`{${m.alias}}`);
  });

  it.each(contract.header)("documents the $key: header directive", (h) => {
    expect(help, `"${h.key}:" is accepted by the engine but absent from the help`).toMatch(
      new RegExp(`\\b${h.key}\\s*:`),
    );
  });

  // TEETH: a help block that merely said "directives are supported" would pass a looser check. Require the
  // shape an author has to type — a brace marker alone on its line, and a header key with a value.
  it("shows the marker and header FORMS, not just the words", () => {
    expect(help).toMatch(/\{np\}/);
    expect(help).toMatch(/\bsize\s*:\s*\d/);
    expect(help).toMatch(/\bcolumns\s*:\s*\d/);
  });
});

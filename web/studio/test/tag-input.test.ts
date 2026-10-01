// T180 — the tag chip logic. Every case is a spec acceptance line: a space is data not a delimiter, a
// trailing comma makes no empty tag, backspace removes exactly one, and a case/accent variant joins the
// existing spelling instead of minting a second.
import { describe, it, expect } from "vitest";
import {
  backspaceLast,
  cloud,
  CLOUD_DEFAULT_N,
  commitAll,
  commitBuffer,
  suggest,
  type Vocab,
} from "../src/pages/song-editor/tagInput";

const vocab: Vocab = [
  { tag: "encore", count: 5 },
  { tag: "slow blues", count: 3 },
  { tag: "needs work", count: 2 },
  { tag: "Été", count: 1 },
];

describe("commitAll (the Enter gesture)", () => {
  it("stores a multi-word tag as ONE tag, space and all", () => {
    expect(commitAll("slow blues", [])).toEqual(["slow blues"]);
  });
  it("trims the edges but keeps the internal space", () => {
    expect(commitAll("  slow blues  ", [])).toEqual(["slow blues"]);
  });
  it("an empty or whitespace buffer makes no tag", () => {
    expect(commitAll("", ["encore"])).toEqual(["encore"]);
    expect(commitAll("   ", ["encore"])).toEqual(["encore"]);
  });
  it("a case/accent variant of an existing chip does not duplicate it", () => {
    expect(commitAll("Encore", ["encore"])).toEqual(["encore"]);
  });
  it("folds to the band's existing spelling rather than minting a second (⟨D3⟩)", () => {
    expect(commitAll("ENCORE", [], vocab)).toEqual(["encore"]);
    expect(commitAll("ete", [], vocab)).toEqual(["Été"]); // accent-insensitive
  });
  it("a genuinely new tag is kept verbatim", () => {
    expect(commitAll("bridge solo", [], vocab)).toEqual(["bridge solo"]);
  });
});

describe("commitBuffer (comma splits; the pasted-list gesture)", () => {
  it("commits every word BEFORE the last comma and keeps the unfinished tail in the buffer", () => {
    const { tags, buffer } = commitBuffer("slow blues, encore, needs work", []);
    expect(tags).toEqual(["slow blues", "encore"]);
    expect(buffer).toBe(" needs work"); // no trailing comma → the last segment is still being typed
  });
  it("a paste is a finished gesture: commitBuffer then commitAll(residual) yields all three chips", () => {
    // How the component handles paste — the comma path plus one final commit of the tail.
    const step = commitBuffer("slow blues, encore, needs work", []);
    const tags = commitAll(step.buffer, step.tags);
    expect(tags).toEqual(["slow blues", "encore", "needs work"]);
  });
  it("a TRAILING comma commits the words before it and creates NO empty tag", () => {
    const { tags, buffer } = commitBuffer("encore,", []);
    expect(tags).toEqual(["encore"]);
    expect(buffer).toBe(""); // the empty final segment is not a tag and not left in the buffer as content
  });
  it("keeps the unfinished final segment in the buffer for continued typing", () => {
    const { tags, buffer } = commitBuffer("encore, slow bl", []);
    expect(tags).toEqual(["encore"]);
    expect(buffer).toBe(" slow bl");
  });
  it("does not double-add across the split when a variant is already chosen", () => {
    const { tags } = commitBuffer("Encore, encore,", [], vocab);
    expect(tags).toEqual(["encore"]);
  });
});

describe("backspaceLast", () => {
  it("removes exactly the last chip, never the one before it", () => {
    expect(backspaceLast(["a", "b", "c"])).toEqual(["a", "b"]);
  });
  it("is a no-op on an empty list", () => {
    expect(backspaceLast([])).toEqual([]);
  });
});

describe("suggest", () => {
  it("offers band tags containing the folded buffer, most-used first, minus chosen", () => {
    const { matches } = suggest("e", vocab, ["encore"]);
    // "encore" is chosen; "needs work"? no 'e'... "needs" has 'e'. "Été" folds to "ete" has 'e'.
    expect(matches.map((m) => m.tag)).toEqual(["slow blues", "needs work", "Été"]);
  });
  it("offers a Create row for a genuinely new word", () => {
    const { createLabel } = suggest("bridge", vocab, []);
    expect(createLabel).toBe("bridge");
  });
  it("offers NO Create row when the buffer folds to an existing tag (no second spelling)", () => {
    expect(suggest("Encore", vocab, []).createLabel).toBeNull();
    expect(suggest("ete", vocab, []).createLabel).toBeNull();
  });
  it("offers NO Create row for a tag already chosen", () => {
    expect(suggest("mine", [], ["Mine"]).createLabel).toBeNull();
  });
  it("empty buffer offers nothing", () => {
    expect(suggest("  ", vocab, [])).toEqual({ matches: [], createLabel: null });
  });
});

describe("cloud", () => {
  const many: Vocab = Array.from({ length: 20 }, (_, i) => ({ tag: `t${i}`, count: 20 - i }));
  it("bounds to N and reports how many are hidden", () => {
    const { shown, hidden } = cloud(many, [], CLOUD_DEFAULT_N, false);
    expect(shown).toHaveLength(12);
    expect(hidden).toBe(8);
  });
  it("reveals the rest on show-all", () => {
    const { shown, hidden } = cloud(many, [], CLOUD_DEFAULT_N, true);
    expect(shown).toHaveLength(20);
    expect(hidden).toBe(0);
  });
  it("drops already-chosen tags from the cloud", () => {
    const { shown } = cloud(vocab, ["encore"], CLOUD_DEFAULT_N, false);
    expect(shown.map((s) => s.tag)).not.toContain("encore");
  });
  it("no 'show all' needed when the vocab already fits", () => {
    expect(cloud(vocab, [], CLOUD_DEFAULT_N, false).hidden).toBe(0);
  });
});

import { describe, it, expect } from "vitest";
import {
  validateNewTag,
  adoptOffer,
  planRename,
  deleteMessage,
  compareTagNames,
  type TagCount,
} from "../src/pages/band-tags";

describe("compareTagNames (T188 ⟨D4⟩ natural order)", () => {
  const VEC = ["s10", "s2", "S1", "tv", "écran", "Encore", "encore", "zoo"];
  const WANT = ["écran", "Encore", "encore", "S1", "s2", "s10", "tv", "zoo"];

  it("sorts fold- and number-aware, with a deterministic fold tie-break", () => {
    expect([...VEC].sort(compareTagNames)).toEqual(WANT);
  });

  it("tells natural order apart from the naive orders", () => {
    const byteOrder = [...VEC].sort((a, b) => (a < b ? -1 : a > b ? 1 : 0));
    expect(byteOrder).not.toEqual(WANT);
    // byte order's two real flaws, which the comparator fixes:
    expect(byteOrder.indexOf("s10")).toBeLessThan(byteOrder.indexOf("s2")); // non-numeric: s10 before s2
    expect(byteOrder[byteOrder.length - 1]).toBe("écran"); // accent (é=U+00E9) sinks to the end
    expect(compareTagNames("s2", "s10")).toBeLessThan(0); // numeric-aware
    expect(compareTagNames("écran", "zoo")).toBeLessThan(0); // folded into the e's, not after z
  });

  it("is order-independent (property 3): two different input orders give the same output", () => {
    const shuffleA = ["encore", "Encore", "zoo", "s2", "tv", "S1", "s10", "écran"];
    const shuffleB = ["tv", "s10", "écran", "S1", "encore", "s2", "Encore", "zoo"];
    expect([...shuffleA].sort(compareTagNames)).toEqual(WANT);
    expect([...shuffleB].sort(compareTagNames)).toEqual(WANT);
  });
});

describe("validateNewTag (⟨D3⟩ server mirror)", () => {
  it("rejects empty / whitespace-only / comma, accepts a normal tag", () => {
    expect(validateNewTag("")).not.toBeNull();
    expect(validateNewTag("   ")).not.toBeNull();
    expect(validateNewTag("a,b")).not.toBeNull();
    expect(validateNewTag(" blues ")).toBeNull();
  });
});

describe("adoptOffer (⟨D3⟩ cannot mint a third spelling)", () => {
  const tags: TagCount[] = [
    { tag: "encore", count: 2 },
    { tag: "ballad", count: 1 },
  ];
  it("offers the existing spelling when the typed one folds equal but differs", () => {
    // renaming some other tag to "ENCORE" — encore exists
    expect(adoptOffer("ENCORE", tags, "ballad")).toEqual({ tag: "encore", count: 2 });
  });
  it("does NOT offer when the typed spelling exactly matches an existing one (that is a merge)", () => {
    expect(adoptOffer("encore", tags, "ballad")).toBeNull();
  });
  it("does NOT offer when the only fold-equal spelling is `from` itself (recasing your own tag)", () => {
    // encore -> Encore, no other variant: a plain rename, not an adopt
    expect(adoptOffer("Encore", tags, "encore")).toBeNull();
  });
  it("offers the MOST-USED when several other spellings fold equal", () => {
    const many: TagCount[] = [
      { tag: "Encore", count: 1 },
      { tag: "encore", count: 5 },
    ];
    expect(adoptOffer("ENCORE", many, "ballad")).toEqual({ tag: "encore", count: 5 });
  });
  it("offers nothing for an unrelated new spelling", () => {
    expect(adoptOffer("bridge", tags, "ballad")).toBeNull();
  });
});

describe("planRename (⟨D5⟩ consequence in EXACT numbers, from the songs)", () => {
  const s = (...tags: string[]) => ({ tags });
  it("a plain rename states the from, the to and the song count", () => {
    const songs = [s("slow blues"), s("slow blues"), s("slow blues"), s("slow blues"), s("other")];
    const p = planRename("slow blues", "blues", songs);
    expect(p.kind).toBe("rename");
    expect(p.message).toBe('"slow blues" will become "blues" on 4 songs.');
  });
  it("Fable ⟨1⟩: a song carrying BOTH spellings is counted ONCE — the merge says 2, not 3", () => {
    // Encore on 1 song (which also has encore); encore on 2 songs. Union = 2, not 1+2.
    const songs = [s("ballad", "Encore", "encore"), s("encore"), s("ballad")];
    const p = planRename("Encore", "encore", songs);
    expect(p.kind).toBe("merge");
    expect(p.message).toBe('"Encore" will be merged into "encore": 1 song changes, "encore" will be on 2.');
  });
  it("a merge with no overlap sums cleanly", () => {
    const songs = [s("enc"), s("enc"), s("encore"), s("encore")];
    const p = planRename("enc", "encore", songs);
    expect(p.message).toBe('"enc" will be merged into "encore": 2 songs change, "encore" will be on 4.');
  });
  it("a case-only rename with no other variant is a plain rename, not a merge", () => {
    const songs = [s("encore"), s("encore")];
    const p = planRename("encore", "Encore", songs);
    expect(p.kind).toBe("rename");
    expect(p.message).toBe('"encore" will become "Encore" on 2 songs.');
  });
});

describe("deleteMessage", () => {
  it("states the tag and the song count, singular and plural", () => {
    expect(deleteMessage("needs work", 5)).toBe('"needs work" will be removed from 5 songs.');
    expect(deleteMessage("solo", 1)).toBe('"solo" will be removed from 1 song.');
  });
});

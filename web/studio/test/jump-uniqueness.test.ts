import { describe, it, expect } from "vitest";
import { jumpKey, takenJumpKeys, withJumpPartners } from "../src/editor";
import type { AnnotationObject } from "../src/api";

// P206 uniqueness — Fable's ⟨D⟩ ruling: "it is a LEGIBILITY rule, not an integrity one", enforced PER FILE.
// Pairing is by uuid, so a collision corrupts nothing and no bake test can ever see it; the damage is to a
// musician reading two identical segnos on one page at a page turn. That is why it is excluded at
// authoring, and why these vectors are about what the PALETTE may offer.

function mark(uuid: string, over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid,
    layerId: "L1",
    type: "icon",
    points: [
      { x: 0.1, y: 0.1 },
      { x: 0.18, y: 0.16 },
    ],
    page: 0,
    text: "segno",
    order: 0,
    createdAt: 0,
    style: { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 16 },
    ...over,
  };
}

describe("takenJumpKeys (P206 uniqueness)", () => {
  it("claims the combination for BOTH ends of a pair", () => {
    const objs = [mark("dest"), mark("src", { jumpTo: "dest" })];
    expect([...takenJumpKeys(objs)]).toEqual([jumpKey("segno", "#e11d48")]);
  });

  it("does not claim a plain cue stamp that happens to look the same", () => {
    // An ordinary icon is not a jump. Only a pair speaks for a combination.
    expect([...takenJumpKeys([mark("plain")])]).toEqual([]);
  });

  it("claims a destination that is placed but not yet paired", () => {
    // The half-finished chain is a claim in progress — handing the same combination out twice while the
    // author is mid-placement is exactly how two identical pairs get made.
    const objs = [mark("dest")];
    expect([...takenJumpKeys(objs, "dest")]).toEqual([jumpKey("segno", "#e11d48")]);
    expect([...takenJumpKeys(objs, null)]).toEqual([]);
  });

  it("is the COMBINATION, not the glyph: the same segno in another colour stays free", () => {
    const objs = [mark("dest"), mark("src", { jumpTo: "dest" })];
    const taken = takenJumpKeys(objs);
    expect(taken.has(jumpKey("segno", "#e11d48"))).toBe(true);
    expect(taken.has(jumpKey("segno", "#2563eb"))).toBe(false); // a different colour reads as different
    expect(taken.has(jumpKey("coda", "#e11d48"))).toBe(false); // a different glyph too
  });

  it("compares colours case-insensitively — #E11D48 and #e11d48 are one colour to a reader", () => {
    const objs = [
      mark("dest", { style: { color: "#E11D48", opacity: 1, width: 0.004, fontSize: 16 } }),
      mark("src", { jumpTo: "dest" }),
    ];
    expect(takenJumpKeys(objs).has(jumpKey("segno", "#e11d48"))).toBe(true);
  });

  it("claims the destination's own appearance even when the two ends differ", () => {
    // Nothing forces the two ends to match today. If they do differ, BOTH looks are spoken for — a third
    // mark copying either one would be the ambiguity the rule exists to prevent.
    const objs = [
      mark("dest", { text: "coda" }),
      mark("src", { jumpTo: "dest" }),
    ];
    const taken = takenJumpKeys(objs);
    expect(taken.has(jumpKey("segno", "#e11d48"))).toBe(true);
    expect(taken.has(jumpKey("coda", "#e11d48"))).toBe(true);
  });
});

// VLL, 2026-09-08: "deleting one of the jumpmark should delete both". A jump is ONE thing wearing two
// marks; grabbing either end takes the pair. This is what makes the earlier per-end delete — and the
// dangling pointer it left behind — impossible to create in Studio at all.
describe("withJumpPartners (P206 — a jump deletes as one)", () => {
  const dest = mark("dest");
  const src = mark("src", { jumpTo: "dest" });

  it("a source pulls its destination", () => {
    expect(withJumpPartners(["src"], [dest, src])).toEqual(["src", "dest"]);
  });

  it("a destination pulls the source aimed at it — either end takes the pair", () => {
    expect(withJumpPartners(["dest"], [dest, src])).toEqual(["dest", "src"]);
  });

  it("an ordinary mark brings nobody", () => {
    expect(withJumpPartners(["plain"], [mark("plain"), dest, src])).toEqual(["plain"]);
  });

  it("a partner that is not here (another part, or already gone) is simply not added", () => {
    expect(withJumpPartners(["src"], [src])).toEqual(["src"]);
  });

  it("keeps the selection's own order and never repeats an end", () => {
    // Selecting BOTH ends and deleting must not queue either one twice.
    expect(withJumpPartners(["src", "dest"], [dest, src])).toEqual(["src", "dest"]);
  });

  it("a mark pointing at itself does not pull itself in twice", () => {
    const selfish = mark("selfish", { jumpTo: "selfish" });
    expect(withJumpPartners(["selfish"], [selfish])).toEqual(["selfish"]);
  });
});

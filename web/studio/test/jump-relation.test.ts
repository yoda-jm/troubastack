import { describe, it, expect } from "vitest";
import { jumpRelation } from "../src/pages/song-editor/jumpRelation";
import type { AnnotationObject, AnnotationLayer } from "../src/api";

// P206 ⟨D4⟩ — VLL: "I don't know in the toolbar if it is a target or a source". The answer is not a role
// noun but the RELATIONSHIP, which also covers the same-page pair (a segment and an arrow, and nothing
// saying which end you hold) and makes the swap button explain itself.

function mark(uuid: string, over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid, layerId: "L1", type: "icon", page: 0, text: "segno", order: 0, createdAt: 0,
    points: [{ x: 0.2, y: 0.4 }, { x: 0.3, y: 0.5 }],
    style: { color: "#059669", opacity: 1, width: 0.004, fontSize: 16 },
    ...over,
  };
}
const layer = (id: string): AnnotationLayer =>
  ({ id, songId: "s", fileId: "f", name: id, ownerId: "u", zone: "shared", order: 0, access: "rw" }) as AnnotationLayer;
const layers = new Map([["L1", layer("L1")], ["L2", layer("L2")]]);
const allVisible = { L1: true, L2: true };

describe("jumpRelation (P206 ⟨D4⟩)", () => {
  const src = mark("src", { page: 4, jumpTo: "dest" });
  const dest = mark("dest", { page: 6 });

  it("names where a source jumps TO, counting pages as a reader does", () => {
    expect(jumpRelation(src, [src, dest], layers, allVisible)).toEqual({
      label: "Jumps to p.7", partnerUuid: "dest",
    });
  });

  it("names where a destination is jumped to FROM", () => {
    expect(jumpRelation(dest, [src, dest], layers, allVisible)).toEqual({
      label: "Jumped to from p.5", partnerUuid: "src",
    });
  });

  it("covers the same-page pair, which had no role indication at all", () => {
    const a = mark("a", { page: 2, jumpTo: "b" }), b = mark("b", { page: 2 });
    expect(jumpRelation(a, [a, b], layers, allVisible)?.label).toBe("Jumps to the other mark");
    expect(jumpRelation(b, [a, b], layers, allVisible)?.label).toBe("Jumped to from the other mark");
  });

  it("says nothing about an ordinary mark", () => {
    expect(jumpRelation(mark("plain"), [mark("plain")], layers, allVisible)).toBeNull();
  });

  it("says nothing when the partner does not exist — the ⟨D2⟩ flag owns that state", () => {
    const orphan = mark("orphan", { jumpTo: "gone" });
    expect(jumpRelation(orphan, [orphan], layers, allVisible)).toBeNull();
  });

  it("does not pair a mark with itself", () => {
    const selfish = mark("selfish", { jumpTo: "selfish" });
    expect(jumpRelation(selfish, [selfish], layers, allVisible)).toBeNull();
  });

  it("REFUSES with the reason when the partner is on a hidden layer", () => {
    const hidden = mark("dest", { page: 6, layerId: "L2" });
    const r = jumpRelation(mark("src", { page: 4, jumpTo: "dest" }), [src, hidden], layers, { L1: true, L2: false });
    expect(r?.label).toBe("Jumps to p.7");                     // it still SAYS what the mark does…
    expect(r?.disabledReason).toBe("its other end is on a hidden layer"); // …and refuses to go there
  });

  it("does NOT gate on editability — navigation is read-only", () => {
    // A partner on a read-only layer is a perfectly good place to go; only visibility refuses.
    const ro = { ...layer("L2"), access: "ro" } as AnnotationLayer;
    const m = new Map([["L1", layer("L1")], ["L2", ro]]);
    const onRO = mark("dest", { page: 6, layerId: "L2" });
    expect(jumpRelation(src, [src, onRO], m, allVisible)?.disabledReason).toBeUndefined();
  });
});

import { describe, it, expect } from "vitest";
import { brokenJumpUuids } from "../src/editor";
import type { AnnotationObject } from "../src/api";

// P206 ⟨D2⟩ — "a jump's two ends must be on the SAME FILE". Studio flags the pairs the BAKE will drop, at
// placement, because a doomed pair is visually identical to a valid cross-page one (neither draws a
// segment) and the author would otherwise learn about it from a bake warning days later.

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

describe("brokenJumpUuids (P206 ⟨D2⟩)", () => {
  it("a pair whose ends are both in this file is NOT broken — including across pages", () => {
    const objs = [mark("dest", { page: 3 }), mark("src", { jumpTo: "dest" })];
    expect([...brokenJumpUuids(objs)]).toEqual([]);
  });

  it("flags the SOURCE when the destination is not in this file (the cross-file pair)", () => {
    // Viewer passes ONE file's objects (T40), so a destination on another part is simply absent here —
    // the same scope the baker resolves in.
    const objs = [mark("src", { jumpTo: "dest-on-part-b" })];
    expect([...brokenJumpUuids(objs)]).toEqual(["src"]);
  });

  it("flags a landmark pointing at itself", () => {
    expect([...brokenJumpUuids([mark("src", { jumpTo: "src" })])]).toEqual(["src"]);
  });

  it("flags neither the destination nor an ordinary mark", () => {
    const objs = [mark("dest"), mark("src", { jumpTo: "dest" }), mark("plain"), mark("r", { type: "rect" })];
    expect([...brokenJumpUuids(objs)]).toEqual([]);
  });

  it("flags every broken source, not just the first", () => {
    const objs = [mark("a", { jumpTo: "gone" }), mark("b", { jumpTo: "also-gone" }), mark("ok-dest"), mark("c", { jumpTo: "ok-dest" })];
    expect([...brokenJumpUuids(objs)].sort()).toEqual(["a", "b"]);
  });
});

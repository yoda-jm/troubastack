// T161 — the pure undo core in editor.ts: which inverse an action needs, and the shared-canvas guard that
// refuses when a bandmate changed the object since. Wrong answers here are the dangerous ones (undo eating
// someone else's work), so they are pinned by hand-built vectors, not by re-deriving the impl.
import { describe, it, expect } from "vitest";
import {
  planUndo,
  objectContentEqual,
  pushBounded,
  UNDO_LIMIT,
  type UndoEntry,
} from "../src/editor";
import type { AnnotationObject } from "../src/api";

function obj(over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid: "o1",
    layerId: "L1",
    type: "rect",
    points: [
      { x: 0.1, y: 0.1 },
      { x: 0.3, y: 0.3 },
    ],
    page: 0,
    text: "",
    order: 0,
    createdAt: 0,
    style: { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 16 },
    ...over,
  };
}

describe("objectContentEqual", () => {
  it("is true for the same user-content and ignores server-stamped createdAt", () => {
    expect(objectContentEqual(obj(), obj({ createdAt: 999 }))).toBe(true);
  });
  it("is false when any user-controlled field differs", () => {
    expect(objectContentEqual(obj(), obj({ page: 1 }))).toBe(false);
    expect(objectContentEqual(obj(), obj({ order: 5 }))).toBe(false);
    expect(objectContentEqual(obj(), obj({ layerId: "L2" }))).toBe(false);
    expect(objectContentEqual(obj(), obj({ text: "x" }))).toBe(false);
    expect(objectContentEqual(obj(), obj({ points: [{ x: 0.1, y: 0.1 }, { x: 0.9, y: 0.9 }] }))).toBe(false);
    expect(objectContentEqual(obj(), obj({ style: { color: "#000", opacity: 1, width: 0.004, fontSize: 16 } }))).toBe(false);
  });
});

describe("planUndo — create", () => {
  const entry: UndoEntry = { action: "create", uuid: "o1", layerId: "L1", before: null, after: obj() };
  it("inverts to a delete when the object is unchanged since I made it", () => {
    expect(planUndo(entry, obj())).toEqual({ do: "delete", uuid: "o1" });
  });
  it("refuses 'changed' when a bandmate mutated it since (rule 2 — the teeth)", () => {
    // Remove objectContentEqual from planUndo and this becomes a delete → it would erase the bandmate's edit.
    expect(planUndo(entry, obj({ points: [{ x: 0.5, y: 0.5 }, { x: 0.8, y: 0.8 }] }))).toEqual({
      do: "refuse",
      reason: "changed",
    });
  });
  it("refuses 'gone' when the object is already deleted", () => {
    expect(planUndo(entry, undefined)).toEqual({ do: "refuse", reason: "gone" });
  });
});

describe("planUndo — delete", () => {
  const deleted = obj({ text: "keepme" });
  const entry: UndoEntry = { action: "delete", uuid: "o1", layerId: "L1", before: deleted, after: null };
  it("inverts to a restore of the exact object (points + style kept, same uuid), not a re-create", () => {
    expect(planUndo(entry, undefined)).toEqual({ do: "restore", object: deleted });
  });
  it("refuses 'changed' if the object reappeared (a bandmate restored/re-made it)", () => {
    expect(planUndo(entry, obj())).toEqual({ do: "refuse", reason: "changed" });
  });
});

describe("planUndo — move/resize/setStyle/setText/reorder", () => {
  const before = obj({ points: [{ x: 0.1, y: 0.1 }, { x: 0.3, y: 0.3 }] });
  const after = obj({ points: [{ x: 0.4, y: 0.4 }, { x: 0.6, y: 0.6 }] });
  const entry: UndoEntry = { action: "move", uuid: "o1", layerId: "L1", before, after };
  it("re-applies the previous value when the live object still matches what I left", () => {
    expect(planUndo(entry, after)).toEqual({ do: "update", kind: "move", object: before });
  });
  it("refuses 'changed' when someone moved it again after me", () => {
    const theirs = obj({ points: [{ x: 0.7, y: 0.7 }, { x: 0.9, y: 0.9 }] });
    expect(planUndo(entry, theirs)).toEqual({ do: "refuse", reason: "changed" });
  });
  it("refuses 'gone' when the object was deleted since", () => {
    expect(planUndo(entry, undefined)).toEqual({ do: "refuse", reason: "gone" });
  });
});

describe("pushBounded", () => {
  it("keeps at most `limit` entries, dropping the oldest", () => {
    let s: number[] = [];
    for (let i = 0; i < UNDO_LIMIT + 10; i++) s = pushBounded(s, i, UNDO_LIMIT);
    expect(s.length).toBe(UNDO_LIMIT);
    expect(s[0]).toBe(10); // 0..9 fell off
    expect(s[s.length - 1]).toBe(UNDO_LIMIT + 9);
  });
  it("does not mutate the input array", () => {
    const a = [1, 2];
    const b = pushBounded(a, 3, 50);
    expect(a).toEqual([1, 2]);
    expect(b).toEqual([1, 2, 3]);
  });
});

// T161 — the pure undo core in editor.ts: which inverse an action needs, the shared-canvas guard that
// refuses when a bandmate changed the object since, ATOMIC multi-delete restore, and the gesture-bounded
// setStyle coalesce. Wrong answers here are the dangerous ones (undo eating someone else's work, or a
// partial restore), so they are pinned by hand-built vectors, not by re-deriving the impl.
import { describe, it, expect } from "vitest";
import {
  planUndo,
  objectContentEqual,
  pushBounded,
  shouldCoalesceStyle,
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

/** A lookup over a fixed live-doc set, as planUndo consumes. */
const live = (objs: AnnotationObject[]) => (uuid: string) => objs.find((o) => o.uuid === uuid);

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
    expect(planUndo(entry, live([obj()]))).toEqual({ do: "delete", uuid: "o1" });
  });
  it("refuses 'changed' when a bandmate mutated it since (rule 2 — the teeth)", () => {
    expect(planUndo(entry, live([obj({ points: [{ x: 0.5, y: 0.5 }, { x: 0.8, y: 0.8 }] })]))).toEqual({
      do: "refuse",
      reason: "changed",
    });
  });
  it("refuses 'gone' when the object is already deleted", () => {
    expect(planUndo(entry, live([]))).toEqual({ do: "refuse", reason: "gone" });
  });
});

describe("planUndo — delete (atomic, one or many)", () => {
  it("restores a single deleted object when still absent", () => {
    const d = obj({ text: "keepme" });
    const entry: UndoEntry = { action: "delete", layerId: "L1", deleted: [d] };
    expect(planUndo(entry, live([]))).toEqual({ do: "restore", objects: [d] });
  });
  it("restores a multi-select delete ALL at once", () => {
    const a = obj({ uuid: "a" });
    const b = obj({ uuid: "b" });
    const entry: UndoEntry = { action: "delete", layerId: "L1", deleted: [a, b] };
    expect(planUndo(entry, live([]))).toEqual({ do: "restore", objects: [a, b] });
  });
  it("refuses the WHOLE batch if a bandmate re-created any one of them (never a partial restore)", () => {
    const a = obj({ uuid: "a" });
    const b = obj({ uuid: "b" });
    const entry: UndoEntry = { action: "delete", layerId: "L1", deleted: [a, b] };
    // `a` reappeared → refuse everything, so `b` is not restored into a state the user never created.
    expect(planUndo(entry, live([obj({ uuid: "a" })]))).toEqual({ do: "refuse", reason: "changed" });
  });
});

describe("planUndo — delete that orphaned a jump pointer (P206)", () => {
  // Deleting one end of a jump clears the survivor's `jumpTo` (it would point at nothing). Undo must put
  // the pair back WHOLE — the revived end AND the pointer — or it leaves a state the user never created:
  // two landmarks that are no longer a jump.
  const dest = obj({ uuid: "dest", type: "icon", text: "segno" });
  const source = obj({ uuid: "src", type: "icon", text: "segno", jumpTo: "dest" });
  const cleared = { ...source, jumpTo: undefined };
  const entry: UndoEntry = { action: "delete", layerId: "L1", deleted: [dest], repointed: [source] };

  it("restores the deleted end AND re-points the survivor", () => {
    expect(planUndo(entry, live([cleared]))).toEqual({
      do: "restore",
      objects: [dest],
      repoint: [source],
    });
  });

  it("leaves a survivor a bandmate has since aimed elsewhere (rule 2) — the restore still happens", () => {
    const theirs = { ...source, jumpTo: "someone-elses-target" };
    expect(planUndo(entry, live([theirs]))).toEqual({ do: "restore", objects: [dest] });
  });

  it("skips a survivor that is gone, rather than reviving it as a side effect", () => {
    expect(planUndo(entry, live([]))).toEqual({ do: "restore", objects: [dest] });
  });

  it("a delete that broke no pair carries no repoint at all", () => {
    const plain: UndoEntry = { action: "delete", layerId: "L1", deleted: [obj()] };
    expect(planUndo(plain, live([]))).toEqual({ do: "restore", objects: [obj()] });
  });
});

describe("planUndo — move/resize/setStyle/setText/reorder", () => {
  const before = obj({ points: [{ x: 0.1, y: 0.1 }, { x: 0.3, y: 0.3 }] });
  const after = obj({ points: [{ x: 0.4, y: 0.4 }, { x: 0.6, y: 0.6 }] });
  const entry: UndoEntry = { action: "move", uuid: "o1", layerId: "L1", before, after };
  it("re-applies the previous value when the live object still matches what I left", () => {
    expect(planUndo(entry, live([after]))).toEqual({ do: "update", kind: "move", object: before });
  });
  it("refuses 'changed' when someone moved it again after me", () => {
    const theirs = obj({ points: [{ x: 0.7, y: 0.7 }, { x: 0.9, y: 0.9 }] });
    expect(planUndo(entry, live([theirs]))).toEqual({ do: "refuse", reason: "changed" });
  });
  it("refuses 'gone' when the object was deleted since", () => {
    expect(planUndo(entry, live([]))).toEqual({ do: "refuse", reason: "gone" });
  });
});

describe("shouldCoalesceStyle", () => {
  const s = (uuid: string): UndoEntry => ({ action: "setStyle", uuid, layerId: "L1", before: obj(), after: obj() });
  it("merges consecutive setStyle on one object within the gesture window (a slider drag)", () => {
    expect(shouldCoalesceStyle(s("o1"), s("o1"), 30)).toBe(true);
  });
  it("does NOT merge two deliberate edits seconds apart (colour, then width)", () => {
    expect(shouldCoalesceStyle(s("o1"), s("o1"), 1500)).toBe(false);
  });
  it("does NOT merge across different objects or a non-setStyle previous entry", () => {
    expect(shouldCoalesceStyle(s("o1"), s("o2"), 30)).toBe(false);
    const move: UndoEntry = { action: "move", uuid: "o1", layerId: "L1", before: obj(), after: obj() };
    expect(shouldCoalesceStyle(move, s("o1"), 30)).toBe(false);
  });
});

describe("pushBounded", () => {
  it("keeps at most `limit` entries, dropping the oldest", () => {
    let s: number[] = [];
    for (let i = 0; i < UNDO_LIMIT + 10; i++) s = pushBounded(s, i, UNDO_LIMIT);
    expect(s.length).toBe(UNDO_LIMIT);
    expect(s[0]).toBe(10);
    expect(s[s.length - 1]).toBe(UNDO_LIMIT + 9);
  });
  it("does not mutate the input array", () => {
    const a = [1, 2];
    const b = pushBounded(a, 3, 50);
    expect(a).toEqual([1, 2]);
    expect(b).toEqual([1, 2, 3]);
  });
});

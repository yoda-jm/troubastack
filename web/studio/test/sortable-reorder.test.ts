import { describe, expect, it } from "vitest";
import { reorder, reorderTo, dropGapFor } from "../src/components/SortableList";

// T142 — the flagship acceptance: an item can be dropped at the END. The old HTML5-drop model keyed a drop
// to a row's top edge (`reorder`), so there was no gap after the last row and a song could never be moved
// last ("on ne peut pas deplacer un morceau en dernier"). The N+1-gap model (`reorderTo`) has that gap.
describe("reorderTo — the N+1-gap model", () => {
  const ids = ["a", "b", "c", "d"];

  it("drops the first item at the END gap (position === length)", () => {
    expect(reorderTo(ids, 0, ids.length)).toEqual(["b", "c", "d", "a"]);
  });

  it("the OLD reorder cannot reach the end — this is the bug", () => {
    // the furthest a top-edge drop could hint is the last row (to = length-1); it lands ABOVE it.
    expect(reorder(ids, 0, ids.length - 1)).toEqual(["b", "c", "a", "d"]); // NOT [...,"a"] — one slot short
    expect(reorder(ids, 0, ids.length - 1)).not.toEqual(["b", "c", "d", "a"]);
  });

  it("drops the last item at the START gap (position 0)", () => {
    expect(reorderTo(ids, 3, 0)).toEqual(["d", "a", "b", "c"]);
  });

  it("moves down into a middle gap", () => {
    // a (0) into gap 3 (between c and d): remove a → [b,c,d], insertAt = 3-1 = 2 → [b,c,a,d]
    expect(reorderTo(ids, 0, 3)).toEqual(["b", "c", "a", "d"]);
  });

  it("moves up into a middle gap", () => {
    // d (3) into gap 1 (between a and b): remove d → [a,b,c], insertAt = 1 → [a,d,b,c]
    expect(reorderTo(ids, 3, 1)).toEqual(["a", "d", "b", "c"]);
  });

  it("dropping into either gap adjacent to itself is a no-op", () => {
    expect(reorderTo(ids, 1, 1)).toEqual(ids); // gap before itself
    expect(reorderTo(ids, 1, 2)).toEqual(ids); // gap after itself
  });

  it("clamps an out-of-range position instead of throwing", () => {
    expect(reorderTo(ids, 0, 99)).toEqual(["b", "c", "d", "a"]);
    expect(reorderTo(ids, 3, -5)).toEqual(["d", "a", "b", "c"]);
  });
});

// T142 stage 2 — the pointer-to-gap mapping. Rows at midpoints [50,150,250,350]; a pointer's Y picks the
// insertion gap (0..N). The END gap (N) — the whole point — is reached by any Y past the last midpoint.
describe("dropGapFor — pointer Y → insertion gap", () => {
  const mids = [50, 150, 250, 350]; // four rows, 100px apart

  it("above the first midpoint ⇒ gap 0 (before the first row)", () => {
    expect(dropGapFor(mids, 10)).toBe(0);
    expect(dropGapFor(mids, 49)).toBe(0);
  });
  it("between midpoints ⇒ the gap between those rows", () => {
    expect(dropGapFor(mids, 100)).toBe(1); // past row0's mid, before row1's
    expect(dropGapFor(mids, 200)).toBe(2);
    expect(dropGapFor(mids, 300)).toBe(3);
  });
  it("past the LAST midpoint ⇒ the END gap (N) — the position the old model could not reach", () => {
    expect(dropGapFor(mids, 400)).toBe(4);
    expect(dropGapFor(mids, 99999)).toBe(4);
  });
  it("an empty list has only gap 0", () => {
    expect(dropGapFor([], 123)).toBe(0);
  });
});

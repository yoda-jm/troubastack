import { describe, expect, it } from "vitest";
import { reorder, reorderTo } from "../src/components/SortableList";

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

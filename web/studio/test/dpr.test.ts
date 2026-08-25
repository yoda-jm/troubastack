// T110 — the canvas-budget DPR arithmetic (helpers.ts). Wrong answers are SILENT: too high blacks out
// on an Android GPU, too low renders blurry. Constants: budget 32e6 px, 3 canvases/page, max side 4096.
import { describe, it, expect } from "vitest";
import { budgetedRasterDpr } from "../src/pages/song-editor/helpers";

describe("budgetedRasterDpr", () => {
  it("returns the raw DPR when there is nothing to budget", () => {
    expect(budgetedRasterDpr(2, [])).toBe(2);
    expect(budgetedRasterDpr(2, [{ w: 0, h: 0 }])).toBe(2); // degenerate pages are skipped
  });
  it("passes the raw DPR through when neither clamp binds", () => {
    // {1000×1000}: byBudget≈3.27, bySide≈4.10, both above 1.5
    expect(budgetedRasterDpr(1.5, [{ w: 1000, h: 1000 }])).toBeCloseTo(1.5, 12);
  });
  it("clamps to the memory budget on a large area", () => {
    // {4000×4000}: byBudget = sqrt(32e6 / (3·16e6)) = sqrt(2/3) ≈ 0.8165 < rawDpr 2
    expect(budgetedRasterDpr(2, [{ w: 4000, h: 4000 }])).toBeCloseTo(Math.sqrt(2 / 3), 12);
  });
  it("the per-side cap is a HARD ceiling that beats even the 0.5 floor", () => {
    // {20000×20000}: budget floor is 0.5, but bySide = 4096/20000 = 0.2048 wins
    expect(budgetedRasterDpr(2, [{ w: 20000, h: 20000 }])).toBeCloseTo(4096 / 20000, 12);
  });
});

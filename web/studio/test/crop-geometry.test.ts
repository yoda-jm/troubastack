// T198 — the crop window's policy-A aspect lock (the rect is a SQUARE in page fractions: w == h). Pure
// geometry whose wrong answers are silent, so a sabotage (e.g. letting w and h move independently) must turn
// one of these red.
import { describe, it, expect } from "vitest";
import { squareAt, moveRect, clampSquareOnPage, MIN_SIDE } from "../src/pages/song-editor/cropGeometry";

const isSquare = (r: { w: number; h: number }) => Math.abs(r.w - r.h) < 1e-9;
const onPage = (r: { x: number; y: number; w: number; h: number }) =>
  r.x >= -1e-9 && r.y >= -1e-9 && r.x + r.w <= 1 + 1e-9 && r.y + r.h <= 1 + 1e-9;

describe("crop geometry — policy A aspect lock (w == h)", () => {
  it("squareAt always produces a square, whatever the drag shape", () => {
    const r = squareAt(0.1, 0.1, 0.9, 0.2); // a wide, shallow drag
    expect(isSquare(r)).toBe(true);
    expect(onPage(r)).toBe(true);
    expect(r.w).toBeGreaterThan(0.5); // side follows the larger delta (0.8), clamped to the page
  });

  it("squareAt enforces the minimum side", () => {
    const r = squareAt(0.5, 0.5, 0.505, 0.505); // a tiny drag
    expect(isSquare(r)).toBe(true);
    expect(r.w).toBeGreaterThanOrEqual(MIN_SIDE - 1e-9);
    expect(onPage(r)).toBe(true);
  });

  it("squareAt grows toward the drag direction from the fixed corner", () => {
    const r = squareAt(0.9, 0.9, 0.3, 0.3); // anchored bottom-right, drag up-left
    expect(isSquare(r)).toBe(true);
    expect(r.x + r.w).toBeLessThanOrEqual(0.9 + 1e-9);
    expect(r.y + r.h).toBeLessThanOrEqual(0.9 + 1e-9);
    expect(onPage(r)).toBe(true);
  });

  it("moveRect keeps the square on the page and preserves its side", () => {
    const moved = moveRect({ x: 0.3, y: 0.3, w: 0.4, h: 0.4 }, 0.9, -0.9); // shove past both edges
    expect(moved.w).toBeCloseTo(0.4, 9);
    expect(isSquare(moved)).toBe(true);
    expect(onPage(moved)).toBe(true);
    expect(moved.x + moved.w).toBeCloseTo(1, 9);
    expect(moved.y).toBeCloseTo(0, 9);
  });

  it("clampSquareOnPage keeps a valid square unshrunk and on-page", () => {
    const r = clampSquareOnPage({ x: 0.7, y: 0.7, w: 0.6, h: 0.6 });
    expect(isSquare(r)).toBe(true);
    expect(r.w).toBeCloseTo(0.6, 9);
    expect(onPage(r)).toBe(true);
  });
});

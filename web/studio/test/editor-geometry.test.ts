// T110 — hit-testing geometry in studio's editor.ts. Pure functions whose wrong answers are SILENT
// (a mis-grab, an off-by-one selection), driven by hand-derived vectors, not by recomputing the impl.
import { describe, it, expect } from "vitest";
import { clamp01, distToSegment, distToPolyline, insideBox, normalizeRect, isMarquee } from "../src/editor";

describe("clamp01", () => {
  it("clamps to [0,1] and maps NaN to 0 (a NaN coord must not escape as a placement)", () => {
    expect(clamp01(0.5)).toBe(0.5);
    expect(clamp01(-1)).toBe(0);
    expect(clamp01(2)).toBe(1);
    expect(clamp01(NaN)).toBe(0);
  });
});

describe("distToSegment", () => {
  const a = { x: 0, y: 0 };
  const b = { x: 1, y: 0 };
  it("is the perpendicular distance when the projection lands on the segment", () => {
    expect(distToSegment({ x: 0.5, y: 0.5 }, a, b)).toBeCloseTo(0.5, 12);
  });
  it("clamps to endpoint a — p=(-1,0) is ON the infinite line (0) but the segment stops at a → 1", () => {
    expect(distToSegment({ x: -1, y: 0 }, a, b)).toBeCloseTo(1, 12);
  });
  it("clamps to endpoint b when the projection falls past the end", () => {
    expect(distToSegment({ x: 2, y: 0 }, a, b)).toBeCloseTo(1, 12);
  });
  it("is the point distance for a degenerate (zero-length) segment", () => {
    expect(distToSegment({ x: 3, y: 4 }, { x: 0, y: 0 }, { x: 0, y: 0 })).toBeCloseTo(5, 12);
  });
});

describe("distToPolyline", () => {
  const pts = [
    { x: 0, y: 0 },
    { x: 1, y: 0 },
    { x: 1, y: 1 },
  ];
  it("returns the minimum over ALL segments, not just the first", () => {
    // near the LAST segment: seg1 (0,0)-(1,0) gives 0.5; seg2 (1,0)-(1,1) gives 0.1 → min 0.1
    expect(distToPolyline({ x: 0.9, y: 0.5 }, pts)).toBeCloseTo(0.1, 12);
  });
  it("is the point distance for a single-point polyline", () => {
    expect(distToPolyline({ x: 3, y: 4 }, [{ x: 0, y: 0 }])).toBeCloseTo(5, 12);
  });
  it("is Infinity for an empty polyline", () => {
    expect(distToPolyline({ x: 0, y: 0 }, [])).toBe(Infinity);
  });
});

describe("insideBox", () => {
  const box = { minX: 0.2, minY: 0.2, maxX: 0.8, maxY: 0.8 };
  it("true inside, false outside", () => {
    expect(insideBox(box, 0.5, 0.5)).toBe(true);
    expect(insideBox(box, 0.1, 0.5)).toBe(false);
  });
  it("the pad extends the box on each axis", () => {
    expect(insideBox(box, 0.1, 0.5, 0.15, 0.15)).toBe(true); // 0.1 >= 0.2 - 0.15
  });
});

describe("normalizeRect", () => {
  it("orders the corners to (min,min)-(max,max) regardless of drag direction", () => {
    expect(normalizeRect({ x: 0.8, y: 0.2 }, { x: 0.2, y: 0.8 })).toEqual({
      x0: 0.2,
      y0: 0.2,
      x1: 0.8,
      y1: 0.8,
    });
  });
});

describe("isMarquee", () => {
  it("true above the 0.01 diagonal threshold, false below (a stray click is not a marquee)", () => {
    expect(isMarquee({ x0: 0, y0: 0, x1: 0.05, y1: 0 })).toBe(true);
    expect(isMarquee({ x0: 0, y0: 0, x1: 0.005, y1: 0 })).toBe(false);
  });
});

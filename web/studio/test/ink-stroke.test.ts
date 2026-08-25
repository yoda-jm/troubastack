// T110 — ink's pure stroke-width + coordinate helpers (the authoritative renderer, reused server-side).
// Wrong answers are SILENT: strokes the wrong thickness, or ink placed off the page.
import { describe, it, expect } from "vitest";
import { strokePx, toPx } from "@troubastack/ink";

const style = { color: "#000000", width: 0.01 };
const page = { x: 10, y: 20, w: 800, h: 600 };

describe("strokePx (stroke width)", () => {
  it("scales style.width (a fraction of page WIDTH) to pixels", () => {
    expect(strokePx({ ...style, width: 0.01 }, page)).toBe(8); // 0.01 · 800
  });
  it("never drops below a 0.5px floor, so a hairline can't disappear", () => {
    expect(strokePx({ ...style, width: 0.0001 }, page)).toBe(0.5); // 0.08 floored to 0.5
  });
});

describe("toPx (page-relative → absolute pixels)", () => {
  it("applies the page ORIGIN and size on both axes", () => {
    expect(toPx({ x: 0.5, y: 0.25 }, page)).toEqual([410, 170]); // [10+0.5·800, 20+0.25·600]
  });
});

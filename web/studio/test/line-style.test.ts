// T177/T179 — ink's line STYLE: the dash pattern, and the terminator geometry D3 pins once so the editor
// and the bake cannot disagree. Pure functions, testable at the seam; the pixels are checked in web/bake
// and in the editor e2e.
import { describe, it, expect } from "vitest";
import {
  ARROW_HEAD_HALF_W,
  ARROW_HEAD_LEN_W,
  dashArray,
  dashKind,
  endShapes,
  lineEnds,
  type EndGeom,
  type InkObject,
  type InkStyle,
} from "@troubastack/ink";

const style = (extra: Partial<InkStyle> = {}): InkStyle => ({
  color: "#000000",
  opacity: 1,
  width: 0.01,
  fontSize: 0,
  ...extra,
});

const line = (extra: Partial<InkObject> = {}): InkObject => ({
  type: "line",
  points: [{ x: 0, y: 0 }, { x: 1, y: 0 }],
  style: style(),
  ...extra,
});

describe("dash", () => {
  it("absent is solid — what every object drawn before T177 renders as", () => {
    expect(dashKind(style())).toBe("solid");
    expect(dashArray(style(), 8)).toEqual([]);
  });
  it("names the closed set; an unknown name reads as solid rather than inventing a pattern", () => {
    expect(dashKind(style({ dash: "dashed" }))).toBe("dashed");
    expect(dashKind(style({ dash: "dotted" }))).toBe("dotted");
    expect(dashKind(style({ dash: "dot-dash" as never }))).toBe("solid");
  });
  it("measures the pattern in STROKE WIDTHS, so the same style dashes the same at any raster size", () => {
    const editor = dashArray(style({ dash: "dashed" }), 4);
    const bake = dashArray(style({ dash: "dashed" }), 12);
    expect(editor).toEqual([12, 8]);
    expect(bake).toEqual(editor.map((n) => n * 3));
  });
  it("dots are a zero-length dash — the round cap makes the dot", () => {
    expect(dashArray(style({ dash: "dotted" }), 6)).toEqual([0, 12]);
  });
});

describe("endShapes — which terminator each end resolves to", () => {
  it("a bare line is two bare ends", () => {
    expect(endShapes(line())).toEqual({ start: null, end: null });
  });
  it("resolves each end independently to its named shape", () => {
    expect(endShapes(line({ style: style({ ends: { end: "arrow" } }) }))).toEqual({ start: null, end: "arrow" });
    expect(endShapes(line({ style: style({ ends: { start: "circle", end: "square" } }) }))).toEqual({
      start: "circle",
      end: "square",
    });
  });
  it("'none', '' and any UNKNOWN shape all draw nothing — never a substituted shape", () => {
    expect(endShapes(line({ style: style({ ends: { end: "none" } }) }))).toEqual({ start: null, end: null });
    expect(endShapes(line({ style: style({ ends: { end: "" } }) }))).toEqual({ start: null, end: null });
    expect(endShapes(line({ style: style({ ends: { end: "diamond" as never } }) }))).toEqual({ start: null, end: null });
  });
  it("is a LINE's property: the same style on a rect decorates nothing", () => {
    const rect: InkObject = { type: "rect", points: [{ x: 0, y: 0 }, { x: 0.5, y: 0.5 }], style: style({ ends: { end: "arrow" } }) };
    expect(endShapes(rect)).toEqual({ start: null, end: null });
  });
});

const W = 4;
const axialLen = (t: EndGeom, tipx: number, tipy: number): number => {
  // distance from the endpoint to the terminator's inward extreme, along the shaft
  if (t.shape === "arrow") return Math.hypot(tipx - (t.left[0] + t.right[0]) / 2, tipy - (t.left[1] + t.right[1]) / 2);
  if (t.shape === "circle") return 2 * t.r;
  const xs = t.corners.map((c) => c[0]);
  return Math.max(...xs) - Math.min(...xs);
};

describe("lineEnds — the trimmed shaft and the terminators", () => {
  it("no ends → the full shaft, no terminators, byte-for-byte the pre-T177 line", () => {
    const { shaft, terms } = lineEnds(10, 20, 210, 20, W, null, null);
    expect(shaft).toEqual({ ax: 10, ay: 20, bx: 210, by: 20 });
    expect(terms).toEqual([]);
  });

  it("an END arrow: tip at the endpoint, shaft TRIMMED back to its base (the T179 fix)", () => {
    const { shaft, terms } = lineEnds(0, 0, 200, 0, W, null, "arrow");
    const t = terms[0];
    expect(t.shape).toBe("arrow");
    if (t.shape === "arrow") expect(t.tip).toEqual([200, 0]);
    // the shaft no longer reaches the endpoint — it stops a head-length short of it
    expect(shaft.bx).toBeCloseTo(200 - ARROW_HEAD_LEN_W * W);
    expect(shaft.ax).toBe(0); // the bare start is untouched
  });

  it("a START shape trims the START of the shaft and leaves the far end alone", () => {
    const { shaft } = lineEnds(0, 0, 200, 0, W, "arrow", null);
    expect(shaft.ax).toBeCloseTo(ARROW_HEAD_LEN_W * W);
    expect(shaft.bx).toBe(200);
  });

  it("DIFFERENT shapes at each end — the case the old {head,side} could not express", () => {
    const { terms } = lineEnds(0, 0, 300, 0, W, "circle", "square");
    expect(terms.map((t) => t.shape).sort()).toEqual(["circle", "square"]);
  });

  it("§4: circle and square sit with their FAR EDGE at the endpoint, entirely inside the line", () => {
    const { terms } = lineEnds(0, 0, 300, 0, W, null, "circle");
    const c = terms[0];
    if (c.shape === "circle") {
      expect(c.cx + c.r).toBeCloseTo(300); // far edge exactly at the endpoint
      expect(c.cx).toBeLessThan(300); // centre is inward — nothing pokes past the end
    }
    const sq = lineEnds(0, 0, 300, 0, W, null, "square").terms[0];
    if (sq.shape === "square") {
      const maxX = Math.max(...sq.corners.map((p) => p[0]));
      expect(maxX).toBeCloseTo(300); // far edge at the endpoint, never beyond
    }
  });

  it("scales with WIDTH, not the page — double the stroke, double the terminator", () => {
    const small = lineEnds(0, 0, 400, 0, 4, null, "arrow");
    const big = lineEnds(0, 0, 400, 0, 8, null, "arrow");
    expect(400 - big.shaft.bx).toBeCloseTo(2 * (400 - small.shaft.bx));
  });

  it("a ZERO-LENGTH line has no direction: no terminator, no trim (the shaft's cap is the tap's dot)", () => {
    const { shaft, terms } = lineEnds(50, 50, 50, 50, W, "arrow", "arrow");
    expect(terms).toEqual([]);
    expect(shaft).toEqual({ ax: 50, ay: 50, bx: 50, by: 50 });
  });

  it("a line shorter than its terminators shrinks them to fit and never crosses the shaft over itself", () => {
    const nominal = ARROW_HEAD_LEN_W * W;
    const len = nominal; // room for ONE nominal head, asked for TWO
    const { shaft, terms } = lineEnds(0, 0, len, 0, W, "arrow", "arrow");
    for (const t of terms) expect(axialLen(t, ...(t.shape === "arrow" ? t.tip : [0, 0]))).toBeLessThanOrEqual(len / 2 + 1e-9);
    // the trimmed shaft must not invert (start past end)
    expect(shaft.ax).toBeLessThanOrEqual(shaft.bx + 1e-9);
  });

  it("follows a diagonal: the terminator points along the line, not along an axis", () => {
    const { terms } = lineEnds(0, 0, 100, 100, W, null, "arrow");
    const t = terms[0];
    if (t.shape === "arrow") {
      const dx = t.tip[0] - (t.left[0] + t.right[0]) / 2;
      const dy = t.tip[1] - (t.left[1] + t.right[1]) / 2;
      expect(dx).toBeCloseTo(dy); // 45°
      expect(Math.hypot(dx, dy)).toBeCloseTo(ARROW_HEAD_LEN_W * W);
    }
  });

  it("the arrow half-width constant still governs the cross size", () => {
    const { terms } = lineEnds(0, 0, 400, 0, W, null, "arrow");
    const t = terms[0];
    if (t.shape === "arrow") expect(Math.abs(t.left[1] - t.right[1])).toBeCloseTo(2 * ARROW_HEAD_HALF_W * W);
  });
});

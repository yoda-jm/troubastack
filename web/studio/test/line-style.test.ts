// T177 — ink's line STYLE: the dash pattern, and the arrowhead geometry D3 asks to be pinned once so
// the editor and the bake cannot disagree. Pure functions, so they are testable at the seam; the
// pixels they produce are checked in web/bake's parity test and in the editor e2e.
import { describe, it, expect } from "vitest";
import {
  ARROW_HEAD_HALF_W,
  ARROW_HEAD_LEN_W,
  arrowHeads,
  dashArray,
  dashKind,
  endsSpec,
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
    // This is the divergence D2 is about: the bake rasterizes a page at a different pixel size than the
    // editor canvas. A px-constant pattern would be a different-looking dash on the stand.
    const editor = dashArray(style({ dash: "dashed" }), 4);
    const bake = dashArray(style({ dash: "dashed" }), 12); // same style, 3× the raster
    expect(editor).toEqual([12, 8]);
    expect(bake).toEqual(editor.map((n) => n * 3));
  });

  it("dots are a zero-length dash — the round cap makes the dot, so it is exactly the stroke's diameter", () => {
    expect(dashArray(style({ dash: "dotted" }), 6)).toEqual([0, 12]);
  });
});

describe("ends", () => {
  it("absent = an undecorated line", () => {
    expect(endsSpec(line())).toBeNull();
  });

  it("an empty head means the DEFAULT head, the way an empty blend means normal", () => {
    expect(endsSpec(line({ style: style({ ends: {} }) }))).toEqual({ side: "end" });
  });

  it("defaults the side to the far end", () => {
    expect(endsSpec(line({ style: style({ ends: { head: "arrow" } }) }))).toEqual({ side: "end" });
  });

  it("a head this renderer does not know draws NOTHING — never a silent arrow", () => {
    // A wrong mark on a chart reads as a musical instruction, so an unknown decoration must be absent
    // rather than approximated by the one head we happen to have.
    expect(endsSpec(line({ style: style({ ends: { head: "bar" as never, side: "end" } }) }))).toBeNull();
  });

  it("is a LINE's property: the same style on a rect decorates nothing", () => {
    const rect: InkObject = {
      type: "rect",
      points: [{ x: 0, y: 0 }, { x: 0.5, y: 0.5 }],
      style: style({ ends: { head: "arrow", side: "both" } }),
    };
    expect(endsSpec(rect)).toBeNull();
  });
});

describe("arrowHeads (the geometry, pinned in ink)", () => {
  const W = 4; // stroke width in px
  const nominal = ARROW_HEAD_LEN_W * W;

  it("puts one head at the far end, pointing along the line", () => {
    const [h] = arrowHeads(0, 0, 200, 0, W, "end");
    expect(h.tip).toEqual([200, 0]);
    // base sits `nominal` back along the shaft, corners `half` either side
    const half = ARROW_HEAD_HALF_W * W;
    expect(h.left[0]).toBeCloseTo(200 - nominal);
    expect(h.right[0]).toBeCloseTo(200 - nominal);
    expect(Math.abs(h.left[1] - h.right[1])).toBeCloseTo(2 * half);
  });

  it("puts it at the START when asked, pointing the other way", () => {
    const [h] = arrowHeads(0, 0, 200, 0, W, "start");
    expect(h.tip).toEqual([0, 0]);
    expect(h.left[0]).toBeCloseTo(nominal); // base is INSIDE the line
  });

  it("both ends = two heads, one at each tip", () => {
    const hs = arrowHeads(0, 0, 200, 0, W, "both");
    expect(hs.map((h) => h.tip)).toEqual([[200, 0], [0, 0]]);
  });

  it("scales with WIDTH, not with the page — double the stroke, double the head", () => {
    const [small] = arrowHeads(0, 0, 200, 0, 4, "end");
    const [big] = arrowHeads(0, 0, 200, 0, 8, "end");
    expect(200 - big.left[0]).toBeCloseTo(2 * (200 - small.left[0]));
  });

  it("a ZERO-LENGTH line has no direction, so it gets no head", () => {
    expect(arrowHeads(50, 50, 50, 50, W, "both")).toEqual([]);
  });

  it("a line SHORTER than its own head keeps the head's shape, shrunk to fit", () => {
    const len = nominal / 2; // half the head's nominal length — a stray tap
    const [h] = arrowHeads(0, 0, len, 0, W, "end");
    const headLen = h.tip[0] - h.left[0];
    expect(headLen).toBeLessThanOrEqual(len + 1e-9); // never longer than the line carrying it
    expect(headLen).toBeCloseTo(len);
    // shape preserved: the width shrank by the same factor as the length
    const half = Math.abs(h.left[1] - h.right[1]) / 2;
    expect(half / headLen).toBeCloseTo(ARROW_HEAD_HALF_W / ARROW_HEAD_LEN_W);
  });

  it("splits a short line's budget between two heads, so they cannot swallow each other", () => {
    const len = nominal; // enough for ONE nominal head, not two
    const hs = arrowHeads(0, 0, len, 0, W, "both");
    // AXIAL length — tip to the middle of the base. The tip-to-corner edge is the slant and is
    // always longer, which is a property of a triangle and not of the clamp.
    const lenOf = (h: (typeof hs)[number]) =>
      Math.hypot(h.tip[0] - (h.left[0] + h.right[0]) / 2, h.tip[1] - (h.left[1] + h.right[1]) / 2);
    for (const h of hs) expect(lenOf(h)).toBeLessThanOrEqual(len / 2 + 1e-9);
  });

  it("follows a diagonal: the head points along the line, not along an axis", () => {
    const [h] = arrowHeads(0, 0, 100, 100, W, "end");
    const dx = h.tip[0] - (h.left[0] + h.right[0]) / 2;
    const dy = h.tip[1] - (h.left[1] + h.right[1]) / 2;
    expect(dx).toBeCloseTo(dy); // 45°
    expect(Math.hypot(dx, dy)).toBeCloseTo(nominal);
  });
});

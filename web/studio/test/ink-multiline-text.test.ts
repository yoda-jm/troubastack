// T190 — ink's drawText renders a multi-line note as lines, stepping each by fontPx × 1.2 (the SAME 1.2
// line height textBBox assumes), so the glyphs land inside the selection box and the bake matches the
// browser. Pure: a recording ctx captures the fillText calls.
import { describe, it, expect } from "vitest";
import { drawText, toPx, type InkObject, type PageRect, type Ctx2D } from "@troubastack/ink";

const page: PageRect = { x: 10, y: 20, w: 800, h: 600 };

function recordingCtx() {
  const calls: { text: string; x: number; y: number; font: string }[] = [];
  const ctx = {
    font: "",
    textBaseline: "" as CanvasTextBaseline,
    fillStyle: "" as string,
    fillText(text: string, x: number, y: number) {
      calls.push({ text, x, y, font: this.font });
    },
  };
  return { ctx: ctx as unknown as Ctx2D, calls };
}

const textObj = (text: string): InkObject =>
  ({
    type: "text",
    layerId: "L1",
    page: 0,
    points: [{ x: 0.25, y: 0.3 }],
    text,
    style: { color: "#202124", opacity: 1, width: 0, fontSize: 0.04 },
  }) as InkObject;

describe("drawText multi-line (T190 ⟨D2⟩)", () => {
  it("draws one fillText per line, same x, stepping by fontPx × 1.2 rounded to whole pixels", () => {
    const { ctx, calls } = recordingCtx();
    drawText(ctx, textObj("a\nb"), page);

    expect(calls.map((c) => c.text)).toEqual(["a", "b"]);
    const [, anchorY] = toPx({ x: 0.25, y: 0.3 }, page);
    const fpx = parseFloat(calls[0].font); // "<fpx>px <family>"
    expect(fpx).toBeGreaterThan(0);
    expect(calls[0].x).toBeCloseTo(calls[1].x, 6); // same left edge
    expect(calls[0].y).toBeCloseTo(anchorY, 6); // first line at the anchor
    // the 1.2 line step, rounded to a whole pixel so line N keeps line one's sub-pixel phase: Skia (the bake)
    // and Chromium (Studio) snap glyphs differently at a fractional y, which broke I8 parity at small sizes.
    expect(calls[1].y - calls[0].y).toBe(Math.round(fpx * 1.2));
  });

  it("a single-line note is one fillText (unchanged)", () => {
    const { ctx, calls } = recordingCtx();
    drawText(ctx, textObj("just one"), page);
    expect(calls).toHaveLength(1);
    expect(calls[0].text).toBe("just one");
  });

  it("three lines → three calls, evenly stepped", () => {
    const { ctx, calls } = recordingCtx();
    drawText(ctx, textObj("x\ny\nz"), page);
    expect(calls.map((c) => c.text)).toEqual(["x", "y", "z"]);
    const fpx = parseFloat(calls[0].font);
    // each line sits a whole-pixel offset from the first: round(i × 1.2 × fontPx), so drift never accumulates
    expect(calls[1].y - calls[0].y).toBe(Math.round(1 * 1.2 * fpx));
    expect(calls[2].y - calls[0].y).toBe(Math.round(2 * 1.2 * fpx));
  });
});

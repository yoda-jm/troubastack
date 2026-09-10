// @vitest-environment jsdom
//
// P206 — the cross-page direction hint. VLL, 2026-09-09, selecting a real jump source: "I don't see any
// segment to the other". Both his pairs are cross-page (a jump usually IS), and the segment only draws
// when the two ends are co-visible — the cross-page hint was deferred at spec time and never built, so the
// COMMON case showed nothing: no link, and no way to tell a source from a destination.
import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { JumpPageHints, jumpPageHints } from "../src/pages/song-editor/JumpPageHint";
import type { AnnotationObject } from "../src/api";

afterEach(cleanup);

function mark(uuid: string, over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid, layerId: "L1", type: "icon", page: 0, text: "segno", order: 0, createdAt: 0,
    points: [{ x: 0.2, y: 0.4 }, { x: 0.3, y: 0.5 }],
    style: { color: "#059669", opacity: 1, width: 0.004, fontSize: 16 },
    ...over,
  };
}

describe("jumpPageHints (P206 cross-page direction)", () => {
  const src = mark("src", { page: 4, jumpTo: "dest" });
  const dest = mark("dest", { page: 6 });

  it("tells the SOURCE which page it jumps to, counting pages as a reader does", () => {
    expect(jumpPageHints([src], [src, dest], 4)).toEqual([{ uuid: "src", outgoing: true, page: 7 }]);
  });

  it("tells the DESTINATION where the jump comes FROM — the arrow reverses", () => {
    expect(jumpPageHints([dest], [src, dest], 6)).toEqual([{ uuid: "dest", outgoing: false, page: 5 }]);
  });

  it("says nothing when the pair is co-visible — that case has the segment", () => {
    const here = mark("dest", { page: 4 });
    expect(jumpPageHints([src], [src, here], 4)).toEqual([]);
  });

  it("says nothing for an ordinary mark, or for one whose partner is not in this file", () => {
    expect(jumpPageHints([mark("plain", { page: 4 })], [mark("plain", { page: 4 })], 4)).toEqual([]);
    expect(jumpPageHints([src], [src], 4)).toEqual([]);
  });

  it("does not pair a mark with itself", () => {
    const selfish = mark("selfish", { page: 4, jumpTo: "selfish" });
    expect(jumpPageHints([selfish], [selfish], 4)).toEqual([]);
  });

  it("renders the arrow and the page, pinned to the mark", () => {
    render(<JumpPageHints objects={[src]} hints={jumpPageHints([src], [src, dest], 4)} />);
    const chip = screen.getByTestId("jump-page-hint");
    expect(chip.textContent).toBe("→ p.7");
    expect(chip.getAttribute("data-uuid")).toBe("src");
    expect(parseFloat(chip.style.left)).toBeCloseTo(30, 6); // the mark's right edge
    // …and its BOTTOM: the chip hangs below the mark, because the selection toolbar overhangs the
    // space above it (d70fdb14). Asserting the top is what went red when that fix landed.
    expect(parseFloat(chip.style.top)).toBeCloseTo(50, 6);
  });

  it("renders the incoming direction on the destination", () => {
    render(<JumpPageHints objects={[dest]} hints={jumpPageHints([dest], [src, dest], 6)} />);
    expect(screen.getByTestId("jump-page-hint").textContent).toBe("← p.5");
  });
});

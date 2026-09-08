// @vitest-environment jsdom
//
// P206 ⟨D2⟩ — the RENDERED red flag on a jump that cannot resolve. Fable, ⟨GO⟩ 4957749c: Studio can no
// longer author a broken jump (a delete takes the pair; an abandoned chain removes its landmark), so
// nothing reachable through the UI makes this draw and no e2e can reach it — yet the state still arrives
// from an import, an older song, or another server. "A rule that makes a state unauthorable makes its
// handler need testing MORE": the seam moves from the surface to the state, and the broken jump is handed
// in directly rather than seeded over the sync wire.
import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { JumpFlags } from "../src/pages/song-editor/JumpFlags";
import { brokenJumpUuids } from "../src/editor";
import type { AnnotationObject } from "../src/api";

afterEach(cleanup);

function mark(uuid: string, over: Partial<AnnotationObject> = {}): AnnotationObject {
  return {
    uuid,
    layerId: "L1",
    type: "icon",
    points: [
      { x: 0.2, y: 0.4 },
      { x: 0.3, y: 0.5 },
    ],
    page: 0,
    text: "segno",
    order: 0,
    createdAt: 0,
    style: { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 16 },
    ...over,
  };
}

describe("JumpFlags (P206 ⟨D2⟩ — the mark a reader actually sees)", () => {
  it("draws a flag over a jump whose destination is not in this file, at the mark's own box", () => {
    // The shape an import leaves behind: a source pointing at a uuid that is not here.
    const objects = [mark("src", { jumpTo: "gone-with-the-other-server" })];
    render(<JumpFlags objects={objects} broken={brokenJumpUuids(objects)} />);
    const flag = screen.getByTestId("jump-broken");
    expect(flag.getAttribute("data-uuid")).toBe("src");
    // Positioned on the mark, in page percentages — a flag somewhere else is worse than none.
    // Compared as numbers: the percentages come out of float math, and pinning their decimal expansion
    // would make this test about IEEE754 rather than about where the flag is.
    const pct = (v: string) => parseFloat(v);
    expect(pct(flag.style.left)).toBeCloseTo(20, 6);
    expect(pct(flag.style.top)).toBeCloseTo(40, 6);
    expect(pct(flag.style.width)).toBeCloseTo(10, 6); // the box is 0.2 → 0.3
    expect(pct(flag.style.height)).toBeCloseTo(10, 6);
  });

  it("draws nothing for a jump whose pair resolves — the valid case must stay unmarked", () => {
    const objects = [mark("dest", { points: [{ x: 0.6, y: 0.1 }, { x: 0.7, y: 0.2 }] }), mark("src", { jumpTo: "dest" })];
    render(<JumpFlags objects={objects} broken={brokenJumpUuids(objects)} />);
    expect(screen.queryAllByTestId("jump-broken")).toHaveLength(0);
  });

  it("flags the source only, never the destination it points at", () => {
    // Both ends are equally 'part of a broken jump', but the pointer is on the source: that is the mark
    // whose target has to change, so that is the mark to draw attention to.
    const objects = [mark("a", { jumpTo: "nowhere" }), mark("b")];
    render(<JumpFlags objects={objects} broken={brokenJumpUuids(objects)} />);
    const flags = screen.getAllByTestId("jump-broken");
    expect(flags.map((f) => f.getAttribute("data-uuid"))).toEqual(["a"]);
  });

  it("draws one flag per broken jump on the page", () => {
    const objects = [mark("a", { jumpTo: "nowhere" }), mark("b", { jumpTo: "also-nowhere" })];
    render(<JumpFlags objects={objects} broken={brokenJumpUuids(objects)} />);
    expect(screen.getAllByTestId("jump-broken")).toHaveLength(2);
  });
});

// @vitest-environment jsdom
//
// T196 — the rendered link: a dot on the linked text and a thin line to the mark, from data handed in.
import { describe, it, expect, afterEach } from "vitest";
import { render, cleanup } from "@testing-library/react";
import { LinkTies } from "../src/pages/song-editor/LinkTies";

afterEach(cleanup);

const box = { minX: 0.3, minY: 0.25, maxX: 0.4, maxY: 0.3 };

describe("LinkTies", () => {
  it("draws one dot and one line per linked mark, in page pixels", () => {
    const { container } = render(
      <LinkTies
        pageBoxPx={{ w: 1000, h: 1400 }}
        items={[
          { uuid: "a", state: "linked", at: { page: 0, x: 0.1, y: 0.2, h: 0.02 }, box },
          { uuid: "b", state: "check", at: { page: 0, x: 0.35, y: 0.26, h: 0.02 }, box }, // dot inside its box
        ]}
      />,
    );
    const dots = container.querySelectorAll('[data-testid="link-dot"]');
    expect(dots).toHaveLength(2);
    expect(dots[0].getAttribute("cx")).toBe("100");
    expect(Number(dots[0].getAttribute("cy"))).toBeCloseTo(294, 6);
    // only the first has a line: the second's dot is on its own mark
    expect(container.querySelectorAll('[data-testid="link-line"]')).toHaveLength(1);
    expect(container.querySelector('[data-uuid="b"]')?.getAttribute("class")).toContain("link-check");
    // chrome: hidden from assistive tech, never a pointer target (CSS), no text
    expect(container.querySelector("svg")?.getAttribute("aria-hidden")).toBe("true");
  });
  it("renders nothing before the page is measured, or with nothing to draw", () => {
    expect(render(<LinkTies pageBoxPx={null} items={[{ uuid: "a", state: "linked", at: { page: 0, x: 0, y: 0, h: 0 }, box }]} />).container.innerHTML).toBe("");
    expect(render(<LinkTies pageBoxPx={{ w: 10, h: 10 }} items={[]} />).container.innerHTML).toBe("");
  });
});

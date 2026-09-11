// @vitest-environment jsdom
//
// P206 ⟨D4⟩ R4 — the refusal, at the surface. The pure vectors decide WHEN to refuse; this proves the
// refusal is actually offered-and-disabled with its reason rather than silently missing, which is the
// difference between "you can't go there because X" and a button that does nothing.
import { describe, it, expect, afterEach } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { SelectionToolbar } from "../src/pages/song-editor/Toolbar";

afterEach(cleanup);

const noop = () => {};
const base = {
  color: "#059669",
  onColor: noop,
  onBringToFront: noop,
  onSendToBack: noop,
  onDuplicate: noop,
  onDelete: noop,
};

describe("SelectionToolbar jump relationship (⟨D4⟩)", () => {
  it("prints the relationship and goes there when pressed", () => {
    let went = 0;
    render(<SelectionToolbar {...base} jumpRelation={{ label: "Jumps to p.7", onGo: () => went++ }} />);
    const btn = screen.getByTestId("sel-jump-relation");
    expect(btn.textContent).toBe("Jumps to p.7");
    expect(btn.hasAttribute("disabled")).toBe(false);
    btn.click();
    expect(went).toBe(1);
  });

  it("REFUSES with the reason when the other end is on a hidden layer", () => {
    let went = 0;
    render(
      <SelectionToolbar
        {...base}
        jumpRelation={{
          label: "Jumps to p.7",
          disabledReason: "its other end is on a hidden layer",
          onGo: () => went++,
        }}
      />,
    );
    const btn = screen.getByTestId("sel-jump-relation") as HTMLButtonElement;
    expect(btn.disabled).toBe(true);
    expect(btn.title).toContain("its other end is on a hidden layer"); // the reason, not just a dead control
    btn.click();
    expect(went).toBe(0);
  });

  it("says nothing at all for a mark that is not part of a jump", () => {
    render(<SelectionToolbar {...base} />);
    expect(screen.queryByTestId("sel-jump-relation")).toBeNull();
  });
});

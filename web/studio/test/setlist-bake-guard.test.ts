// T124 — the Bake control is unavailable for a setlist with no songs, and available once one is present.
// A bake of an empty setlist produces a "concert" of nothing and reports success; the server now rejects
// it, and the button must not offer it in the first place.
import { describe, it, expect } from "vitest";
import { bakeSetlistDisabled } from "../src/pages/SetlistDetail";

describe("bakeSetlistDisabled (T124)", () => {
  it("disables bake for an empty setlist (no songs)", () => {
    expect(bakeSetlistDisabled(false, 0)).toBe(true);
  });
  it("enables bake once at least one song is present", () => {
    expect(bakeSetlistDisabled(false, 1)).toBe(false);
    expect(bakeSetlistDisabled(false, 5)).toBe(false);
  });
  it("stays disabled while the bake dialog is open, regardless of song count", () => {
    expect(bakeSetlistDisabled(true, 3)).toBe(true);
  });
});

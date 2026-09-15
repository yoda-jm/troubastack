import { describe, it, expect } from "vitest";
import { GLYPH_IDS } from "@troubastack/ink";
import { CUE_ICON_LABELS } from "../src/components/CueGlyphs";

// Glossary D11 — every glyph the pickers can offer must have a HUMAN label. The palette falls back to
// `CUE_ICON_LABELS[id] ?? id`, and that fallback is silent: a glyph added without a label does not break
// anything, it just shows a musician its internal id. The P206 landmarks lived in that state from the day
// the jump tool shipped, and nobody saw it because the fallback renders something plausible.
//
// The guard enumerates the SOURCE — the generated GLYPH_IDS contract — rather than a list kept beside it,
// so a glyph authored tomorrow is covered the moment it exists rather than when somebody remembers.
describe("glyph labels (D11)", () => {
  it("every glyph id has a label", () => {
    const unlabelled = GLYPH_IDS.filter((id) => !CUE_ICON_LABELS[id]);
    expect(
      unlabelled,
      `these glyphs would show their raw id in the picker: ${unlabelled.join(", ")}`,
    ).toEqual([]);
  });

  it("the guard is looking at a real contract", () => {
    // positive control: an empty GLYPH_IDS would make the assertion above pass forever.
    expect(GLYPH_IDS.length).toBeGreaterThan(20);
  });

  it("no label is just the id with different capitalisation", () => {
    // "segno" → "Segno" is a real label; "guitar-electric" → "guitar-electric" would be the fallback
    // wearing a costume. Catches a lazy fill that satisfies the first test without helping anyone.
    const lazy = GLYPH_IDS.filter((id) => CUE_ICON_LABELS[id]?.toLowerCase() === id.toLowerCase() && id.includes("-"));
    expect(lazy, `these labels are just the id: ${lazy.join(", ")}`).toEqual([]);
  });
});

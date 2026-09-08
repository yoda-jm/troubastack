import { describe, it, expect } from "vitest";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
// The authoring source (never shipped) + the generated runtime contract.
import { GLYPHS, CUE_IDS, LANDMARK_IDS } from "../../ink/glyphs.authoring.mjs";
import { GLYPH_IDS, CUE_GLYPH_IDS, LANDMARK_GLYPH_IDS } from "@troubastack/ink";

// P206 ⟨R1⟩ teeth (Fable): every glyph must EXPLICITLY declare a kind. A new glyph the author forgot to
// categorize must fail loudly — not silently default to a cue and surface in the wrong picker. gen-glyphs
// throws on an uncategorized glyph; this pins the same invariant in a test.
describe("glyph kind curation (P206 ⟨R1⟩)", () => {
  it("every authored glyph is in EXACTLY one of CUE_IDS / LANDMARK_IDS", () => {
    const uncategorized = Object.keys(GLYPHS).filter(
      (id) => (CUE_IDS as Set<string>).has(id) === (LANDMARK_IDS as Set<string>).has(id),
    );
    expect(uncategorized, `glyphs not in exactly one kind set (add to CUE_IDS or LANDMARK_IDS): ${uncategorized}`).toEqual([]);
  });

  it("neither kind set names a glyph that does not exist", () => {
    const ids = new Set(Object.keys(GLYPHS));
    const stray = [...(CUE_IDS as Set<string>), ...(LANDMARK_IDS as Set<string>)].filter((id) => !ids.has(id));
    expect(stray, `kind sets name unknown glyphs: ${stray}`).toEqual([]);
  });

  it("the generated contract partitions the ids into cue + landmark", () => {
    expect([...CUE_GLYPH_IDS, ...LANDMARK_GLYPH_IDS].sort()).toEqual([...GLYPH_IDS].sort());
    expect(CUE_GLYPH_IDS.filter((id) => LANDMARK_GLYPH_IDS.includes(id))).toEqual([]);
  });

  it("the jump landmarks are the expected symbol set (no D.S./D.C. — Fable: they are instructions)", () => {
    expect([...LANDMARK_GLYPH_IDS].sort()).toEqual(
      ["circle", "coda", "diamond", "segno", "square", "star", "triangle"].sort(),
    );
  });

  // The generator's SECOND output — the app's CueGlyphData.kt — is a cue-only mirror. b6fe6e06 emitted
  // every glyph into it, so `cueGlyph("segno")` would have resolved a jump landmark as a cue stamp (and
  // the android CueTest went red for 9h). The CI drift guard cannot see this: it regenerates and diffs,
  // so a wrong-but-consistent mirror passes. This is what pins the mirror's CONTENT to the cue set.
  it("the Kotlin mirror (CueGlyphData.kt) carries the cue glyphs ONLY", () => {
    const kt = readFileSync(
      fileURLToPath(new URL("../../../app/shared/src/commonMain/kotlin/com/troubastack/shared/stage/CueGlyphData.kt", import.meta.url)),
      "utf8",
    );
    const mirrored = [...kt.matchAll(/^ {4}"([^"]+)" to CueGlyph\(/gm)].map((m) => m[1]);
    expect(mirrored).toEqual(CUE_GLYPH_IDS);
    expect(mirrored.filter((id) => LANDMARK_GLYPH_IDS.includes(id))).toEqual([]);
  });
});

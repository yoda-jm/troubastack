// T135 stage 2 — the STATEFUL chart highlighter: tab blocks ({sot}…{eot}) are a cross-line zone, so
// the tokenizer must carry state. Text must be preserved line-for-line (the overlay sits under the
// caret). Also covers the {np}/{fn} fix (marker, not plain).
import { describe, it, expect } from "vitest";
import { tokenizeChartSource, isTabStart, isTabEnd } from "../src/pages/song-editor/chartHighlight";

// classesOf returns the single whole-line class where a line is one token, else "mixed".
function lineClasses(src: string): string[] {
  return tokenizeChartSource(src).map((toks) => (toks.length === 1 ? toks[0].cls : "mixed"));
}

// the text of every token, joined per line, must equal the input line exactly (overlay invariant).
function preservesText(src: string): boolean {
  const lines = src.split("\n");
  const toks = tokenizeChartSource(src);
  return lines.every((ln, i) => toks[i].map((t) => t.text).join("") === ln);
}

describe("chart tab highlighter (T135)", () => {
  it("markers vs near-misses", () => {
    expect(isTabStart("{sot}")).toBe(true);
    expect(isTabStart("  {START_OF_TAB}  ")).toBe(true);
    expect(isTabStart("{sot} x")).toBe(false);
    expect(isTabStart("{{sot}}")).toBe(false);
    expect(isTabEnd("{eot}")).toBe(true);
    expect(isTabEnd("{end_of_tab}")).toBe(true);
  });

  it("a block: markers are hl-marker, content hl-tab, a chord row over the strings keeps hl-chord", () => {
    const src = ["# T", "{sot}", "     G     D", "e|--0--2--|", "{eot}", "G", "lyric"].join("\n");
    // lines:      title    marker    chord(in tab)  tab-line     marker    chord   plain
    expect(lineClasses(src)).toEqual([
      "hl-title",
      "hl-marker",
      "hl-chord",
      "hl-tab",
      "hl-marker",
      "hl-chord",
      "hl-plain",
    ]);
  });

  it("inside a block, section/marker/bold are verbatim hl-tab (nothing but the closer is special)", () => {
    const src = ["{sot}", "## Not A Section", "{np}", "**not bold**", "{eot}"].join("\n");
    expect(lineClasses(src)).toEqual(["hl-marker", "hl-tab", "hl-tab", "hl-tab", "hl-marker"]);
  });

  it("{np}/{fn} outside a block are hl-marker, not plain (the pre-T135 bug)", () => {
    expect(lineClasses("{np}\n{fn}\n{new_page}\n{footnote}")).toEqual([
      "hl-marker",
      "hl-marker",
      "hl-marker",
      "hl-marker",
    ]);
  });

  it("an unclosed block runs to EOF", () => {
    expect(lineClasses("{sot}\ne|--0--|\nB|--1--|")).toEqual(["hl-marker", "hl-tab", "hl-tab"]);
  });

  it("preserves text line-for-line (the overlay-under-caret invariant)", () => {
    expect(preservesText("# T\n{sot}\n**x** ## {np}\ne|--0--|\n{eot}\nG C\nla **la**")).toBe(true);
  });
});

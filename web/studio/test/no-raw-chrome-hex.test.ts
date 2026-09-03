import { describe, it, expect } from "vitest";
import { readFileSync, readdirSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { join, sep } from "node:path";

/**
 * BRAND03 drift guard — chrome colour belongs to the token system.
 *
 * Every colour used for application chrome must be a `--brand*`/`--*` token defined in
 * styles.css, referenced with `var(--…)`. A raw hex literal pasted into a component is
 * exactly how `#4f46e5` (an indigo in no brand palette) outlived the mark until BRAND03.
 * This test fails on ANY hex colour literal in a `.ts`/`.tsx` file.
 *
 * The ONLY exemption is the annotation vocabulary. The pen/highlight/cue palettes and the
 * visual-beat tier colours are CONTENT, not chrome: they travel in the bundle, identify who
 * marked what, and are matched by tests and the baked render. BRAND03 is explicit that these
 * are out of scope — restyling them would change what is already on people's charts. They are
 * exempted by FILE, and each file is a known palette, so a new chrome hex cannot hide behind
 * the exemption unless it is pasted into one of these three (annotation-only) modules.
 */
const SRC = fileURLToPath(new URL("../src", import.meta.url));

// Relative paths (from src/) whose hex literals are annotation content, not chrome.
const ANNOTATION_VOCABULARY = new Set([
  "editor.ts", // default pen colour + the pen palette
  join("pages", "song-editor", "MyCuesEditor.tsx"), // the cue-colour picker
  "beatFrame.ts", // visual-beat tier colours (T85: bar · felt pulse · subdivision)
]);

// Valid CSS hex lengths only (3/4/6/8), so a 5-digit id or a git SHA fragment is not a match.
const HEX = /#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})\b/g;

function walk(dir: string, rel = ""): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir, { withFileTypes: true })) {
    const childRel = rel ? rel + sep + entry.name : entry.name;
    if (entry.isDirectory()) out.push(...walk(join(dir, entry.name), childRel));
    else if (/\.(ts|tsx)$/.test(entry.name)) out.push(childRel);
  }
  return out;
}

describe("BRAND03 chrome-colour drift guard", () => {
  it("no raw hex colour in components — chrome colour must be a token in styles.css", () => {
    const offenders: string[] = [];
    for (const rel of walk(SRC)) {
      if (ANNOTATION_VOCABULARY.has(rel)) continue;
      const hits = readFileSync(join(SRC, rel), "utf8").match(HEX);
      if (hits) offenders.push(`${rel}: ${[...new Set(hits)].join(", ")}`);
    }
    expect(
      offenders,
      `Raw chrome hex found — move it to a --token in styles.css and use var(--…):\n${offenders.join("\n")}`,
    ).toEqual([]);
  });
});

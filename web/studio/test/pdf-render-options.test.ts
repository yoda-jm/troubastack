import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { PDF_RENDER_OPTIONS } from "../src/pages/song-editor/pdfOptions";

// T151 — the song editor was a blank page in the Android WebView because pdf.js chose the OffscreenCanvas
// path, which the WebView renders blank. The fix forces the main-thread canvas. This is the guard Fable
// asked for: pin the render option AND assert every getDocument call at the call site actually spreads it,
// so a future refactor cannot silently drop the flag and hand the blank page back to a musician (no tablet
// needed — the device check is the acceptance, this is the per-commit guard). Same shape as T144.
describe("T151 — pdf.js main-thread canvas", () => {
  it("forces isOffscreenCanvasSupported: false", () => {
    expect(PDF_RENDER_OPTIONS.isOffscreenCanvasSupported).toBe(false);
  });

  it("every getDocument call in usePdfDocument spreads PDF_RENDER_OPTIONS", () => {
    const here = dirname(fileURLToPath(import.meta.url));
    const src = readFileSync(resolve(here, "../src/pages/song-editor/usePdfDocument.ts"), "utf8");
    const calls = src.match(/getDocument\([^)]*\)/g) ?? [];
    expect(calls.length).toBeGreaterThan(0);
    for (const c of calls) {
      expect(c, `getDocument call must force the main-thread canvas: ${c}`).toContain("...PDF_RENDER_OPTIONS");
    }
  });
});

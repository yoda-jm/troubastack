/**
 * T44 guard: raster canvases must never exceed the per-side GPU max-texture floor
 * (~4096 px), and stay rendered (not black), even at high zoom on a high-DPI viewport.
 * VLL (Android): page 2 rendered BLACK, worse on zoom-in — a canvas-memory/max-texture
 * failure. budgetedRasterDpr caps the raster DPR so a page canvas is SOFTER past the
 * budget, never black.
 *
 * This is the CI-testable half (the math via emulated dpr/zoom); the black-is-gone proof
 * is on VLL's device. Red-first: pre-fix, at 300% × dpr 2 a Letter page's raster side
 * (~4752 px) exceeds 4096.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

// A high-DPI viewport so the raster (scale × dpr) is large enough to hit the cap.
test.use({ viewport: { width: 900, height: 1200 }, deviceScaleFactor: 2 });

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));
const MAX_SIDE = 4096;

test("raster canvases stay within the GPU side cap + rendered at high zoom (T44)", async ({ page }) => {
  await register(page, `cb_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`CB ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`CBSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  await page.reload();

  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("pdf-page")).toHaveCount(2);

  // Zoom to the max percent option (300%) — at dpr 2 a Letter page's uncapped raster
  // height (~4752 px) would blow the 4096 floor; the budget clamp keeps it ≤ 4096.
  await page.getByTestId("zoom-mode").selectOption("300");
  await page.waitForTimeout(1500); // let the re-raster settle

  // T44 (gate-required): EVERY per-page canvas layer must be under the side cap — the
  // topmost wet EditCanvas + the annotation overlay too, not just the raster (an
  // uncompositable topmost layer is what reads as a black page).
  const allSides = await page.locator(".pdf-canvas, .edit-canvas, .annotation-overlay").evaluateAll((els) =>
    els.map((c) => ({ cls: c.className, w: (c as HTMLCanvasElement).width, h: (c as HTMLCanvasElement).height })),
  );
  for (const c of allSides) {
    expect(c.w, `${c.cls} width ${c.w} ≤ 4096`).toBeLessThanOrEqual(4096);
    expect(c.h, `${c.cls} height ${c.h} ≤ 4096`).toBeLessThanOrEqual(4096);
  }

  const canvases = await page.locator(".pdf-canvas").evaluateAll((els) =>
    els.map((c) => {
      const cv = c as HTMLCanvasElement;
      const ctx = cv.getContext("2d")!;
      let sum = 0;
      let n = 0;
      try {
        const d = ctx.getImageData(0, 0, Math.min(cv.width, 60), Math.min(cv.height, 60)).data;
        for (let k = 0; k < d.length; k += 4) {
          sum += (d[k] + d[k + 1] + d[k + 2]) / 3;
          n++;
        }
      } catch {
        return { w: cv.width, h: cv.height, meanLum: -1 };
      }
      return { w: cv.width, h: cv.height, meanLum: Math.round(sum / n) };
    }),
  );
  expect(canvases.length).toBe(2);
  for (const c of canvases) {
    // Per-side cap: neither dimension may exceed the GPU max-texture floor.
    expect(c.w, `canvas width ${c.w} must be ≤ ${MAX_SIDE}`).toBeLessThanOrEqual(MAX_SIDE);
    expect(c.h, `canvas height ${c.h} must be ≤ ${MAX_SIDE}`).toBeLessThanOrEqual(MAX_SIDE);
    // Still rendered (a blank/black page would be ~0). sample.pdf is light content.
    expect(c.meanLum, `canvas must be rendered, not black (meanLum ${c.meanLum})`).toBeGreaterThan(40);
  }
});

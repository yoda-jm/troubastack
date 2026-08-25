/**
 * T27 stage 4 — touch gesture grammar.
 *
 * The load-bearing, assertable invariant (mirrors editor-wheelzoom for the mouse
 * path): a two-finger PINCH must DECOUPLE the visual zoom (a live CSS transform)
 * from rasterization — the crisp PDF re-raster is committed exactly ONCE, on
 * gesture settle. A fast pinch costs one raster pass (one per page), never one per
 * move tick. Also: the pinch actually changes the zoom.
 *
 * Driven via CDP Input.synthesizePinchGesture on a touch-enabled context; the wet
 * canvas's pointer handlers turn the two touch pointers into the shared burst
 * pipeline (usePdfDocument.beginGesture/updateGesture/endGesture).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

// Touch must be enabled for the browser to emit touch → pointer(type:touch) events.
test.use({ hasTouch: true });

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function uploadPdf(page: Page) {
  // T36: file management moved into the editor's Details panel — open it to reach the
  // upload form, then close it so the canvas is unobstructed for whatever follows.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}
async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}
const renderCount = (page: Page) =>
  page.getByTestId("pdf-render-count").innerText().then((t) => parseInt(t, 10));

test("editor: a two-finger pinch zooms but re-rasters exactly once (not per tick)", async ({
  page,
}) => {
  await register(page, `tc_${stamp()}`);
  await createBandAndOpen(page, `TCBand ${stamp()}`);
  await createSongAndOpen(page, `TCSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Deterministic base scale; let the first raster settle.
  await page.getByTestId("zoom-mode").selectOption("100");
  await expect.poll(() => renderCount(page)).toBeGreaterThan(0);
  await page.waitForTimeout(300);

  const pageCount = await page.getByTestId("pdf-page").count();
  const before = await renderCount(page);

  // Pinch OUT (zoom in) at the first page's centre.
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height / 2;
  const client = await page.context().newCDPSession(page);
  await client.send("Input.synthesizePinchGesture", {
    x: Math.round(cx),
    y: Math.round(cy),
    scaleFactor: 2,
    relativeSpeed: 800,
    gestureSourceType: "touch",
  });

  // Wait past the settle debounce + the single re-raster.
  await page.waitForTimeout(500);

  // The zoom changed (a fit/percent readout above 100%).
  const zoomVal = await page.getByTestId("zoom-mode").inputValue();
  expect(parseInt(zoomVal, 10)).toBeGreaterThan(100);

  // …and the PDF re-rasterized exactly ONCE (one pass = one bump per page), NOT
  // once per pinch move tick — the stage-1 decouple invariant, on touch.
  const after = await renderCount(page);
  expect(after - before).toBe(pageCount);
});

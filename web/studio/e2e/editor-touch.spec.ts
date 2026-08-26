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
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";
import { waitRenderStable } from "./render-helpers";

// Touch must be enabled for the browser to emit touch → pointer(type:touch) events.
test.use({ hasTouch: true });

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

  // Deterministic base scale; wait for the 100% re-raster to settle (T115), not a fixed sleep.
  const baseline = await renderCount(page);
  await page.getByTestId("zoom-mode").selectOption("100");
  await waitRenderStable(page, baseline);

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

  // Wait for the single re-raster to LAND and QUIESCE (T115) — the wait never returns mid-climb, so
  // a per-pinch-tick regression (delta > pageCount) still reddens the assertion below.
  const after = await waitRenderStable(page, before);

  // The zoom changed (a fit/percent readout above 100%).
  const zoomVal = await page.getByTestId("zoom-mode").inputValue();
  expect(parseInt(zoomVal, 10)).toBeGreaterThan(100);

  // …and the PDF re-rasterized exactly ONCE (one pass = one bump per page), NOT
  // once per pinch move tick — the stage-1 decouple invariant, on touch.
  expect(after - before).toBe(pageCount);
});

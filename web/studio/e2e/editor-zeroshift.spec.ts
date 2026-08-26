/**
 * T27 stage 3 — ZERO-SHIFT guard (written FIRST, per the T17→T27 requirement).
 *
 * The fullscreen editor floats its chrome (top bar, contextual style row, the
 * Layers/Annotations panel, the bottom parts/status bar) as position:absolute glass
 * over the canvas. The load-bearing invariant: showing/hiding any of that chrome must
 * NOT move the score. This asserts the first .pdf-page's viewport box is byte-stable
 * across:
 *   1. activating a draw tool (the contextual style row appears),
 *   2. opening then closing the Layers/Annotations panel.
 *
 * Fails on the pre-stage-3 stacked layout (the panel is in flow → toggling it resizes
 * the scroll column → the canvas moves); passes once the chrome floats.
 */
import { test, expect, type Page } from "@playwright/test";
import { openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

async function pageBox(page: Page) {
  // Settle any transient layout before measuring.
  await page.waitForTimeout(200);
  const b = (await page.getByTestId("pdf-page").first().boundingBox())!;
  return { x: Math.round(b.x), y: Math.round(b.y), w: Math.round(b.width), h: Math.round(b.height) };
}
const near = (a: number, b: number, tol = 1) => Math.abs(a - b) <= tol;
function sameBox(a: Awaited<ReturnType<typeof pageBox>>, b: Awaited<ReturnType<typeof pageBox>>) {
  return near(a.x, b.x) && near(a.y, b.y) && near(a.w, b.w) && near(a.h, b.h);
}

// T27 stage 3: the chrome floats (position:absolute) OVER the canvas — the top pill,
// the contextual .ctx style pill, and the on-demand Layers/Annotations drawer. Because
// they float, showing/hiding any of them cannot move the score. This asserts the first
// pdf-page's viewport box is byte-stable across (1) activating a draw tool (the .ctx
// slides in) and (2) opening then closing the drawer.
test("editor: activating a draw tool does NOT shift the canvas (zero-shift, T27 stage 3)", async ({
  page,
}) => {
  await register(page, `zs_${stamp()}`);
  await createBandAndOpen(page, `ZSBand ${stamp()}`);
  await createSongAndOpen(page, `ZSSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
  // new-layer lives in the on-demand drawer; open it, then close it so the baseline
  // is measured with the chrome in its resting (canvas-first) state.
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await page.getByTestId("sidebar-toggle").click(); // close the drawer

  const base = await pageBox(page);

  // 1. Activate a draw tool → the contextual .ctx style pill appears. Canvas must not move.
  await page.getByTestId("tool-rect").click();
  const afterTool = await pageBox(page);
  expect(sameBox(base, afterTool)).toBeTruthy();

  // Back to select → .ctx hides. Still no move.
  await page.getByTestId("tool-select").click();
  const afterSelect = await pageBox(page);
  expect(sameBox(base, afterSelect)).toBeTruthy();

  // Cycle through the other draw tools — each swaps the floating .ctx contents but
  // never moves the score below it.
  for (const t of ["tool-text", "tool-ellipse", "tool-line", "tool-select"]) {
    await page.getByTestId(t).click();
    expect(sameBox(base, await pageBox(page))).toBeTruthy();
  }
});

// The stage-3 CLOSE-OUT (arch guardrail #3): opening/closing the floating drawer must
// NOT shift the score. This was deferred while the drawer's design was in question; the
// tabbed floating drawer is now the approved design, so it flips to a live assertion.
test("editor: opening/closing the drawer does NOT shift the canvas (panel-toggle zero-shift)", async ({
  page,
}) => {
  await register(page, `zsp_${stamp()}`);
  await createBandAndOpen(page, `ZSPBand ${stamp()}`);
  await createSongAndOpen(page, `ZSPSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Baseline: drawer closed (canvas-first default).
  const base = await pageBox(page);

  // Open the Layers drawer → it floats over the canvas; the score must not move.
  await openDrawer(page, "layers");
  await expect(page.getByTestId("viewer-drawer")).toBeVisible();
  expect(sameBox(base, await pageBox(page))).toBeTruthy();

  // Switch to the Annotations tab → still no move.
  await openDrawer(page, "annotations");
  expect(sameBox(base, await pageBox(page))).toBeTruthy();

  // Close it again → back to baseline, no move. T94: the single rail pill toggles the rail closed
  // (drawer-notes is now a tab inside the rail, not a pill that closes it).
  await page.getByTestId("sidebar-toggle").click();
  await expect(page.getByTestId("viewer-drawer")).toHaveCount(0);
  expect(sameBox(base, await pageBox(page))).toBeTruthy();
});

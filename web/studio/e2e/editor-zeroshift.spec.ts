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
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function uploadPdf(page: Page) {
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
}
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

// fixme until T27 stage 3 floats the chrome: on the current stacked layout the
// in-flow Layers/Annotations panel resizes the scroll column when toggled, so the
// canvas moves. Written FIRST per the T17→T27 zero-shift requirement; stage 3 flips
// this `.fixme` to `test(...)` once the chrome is position:absolute over the canvas.
test.fixme("editor: chrome show/hide does NOT shift the canvas (zero-shift, T27 stage 3)", async ({
  page,
}) => {
  await register(page, `zs_${stamp()}`);
  await createBandAndOpen(page, `ZSBand ${stamp()}`);
  await createSongAndOpen(page, `ZSSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const base = await pageBox(page);

  // 1. Activate a draw tool → the contextual style row appears. Canvas must not move.
  await page.getByTestId("new-layer").click();
  await page.getByTestId("tool-rect").click();
  const afterTool = await pageBox(page);
  expect(sameBox(base, afterTool)).toBeTruthy();

  // Back to select → style row hides. Still no move.
  await page.getByTestId("tool-select").click();
  const afterSelect = await pageBox(page);
  expect(sameBox(base, afterSelect)).toBeTruthy();

  // 2. Toggle the Layers/Annotations panel closed then open. Canvas must not move.
  await page.getByTestId("sidebar-toggle").click();
  const afterHide = await pageBox(page);
  expect(sameBox(base, afterHide)).toBeTruthy();

  await page.getByTestId("sidebar-toggle").click();
  const afterShow = await pageBox(page);
  expect(sameBox(base, afterShow)).toBeTruthy();
});

/**
 * BUG #1 — PDF must NOT re-rasterize (flicker / zoom-out-then-back) on every
 * annotation edit.
 *
 * Root cause: the PDF page-render effect depended (via paintOverlay) on
 * doc.objects / visibility, so it re-rasterized every page on each draw / move /
 * restyle, and during the re-raster the fit-width scale transiently went wrong →
 * visible flicker.
 *
 * Reproduce-first: at a FIXED zoom, capture (a) the first PDF page canvas's
 * backing width/height and (b) the `pdf-render-count` probe. Add an annotation,
 * then move it. Assert the backing size is IDENTICAL and the render-count did
 * NOT increase — i.e. the PDF was never re-rasterized by the edit.
 *
 * Screenshot: /tmp/ed4-noflicker.png (edited page at the correct, stable scale).
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
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

async function createBandAndOpen(page: Page, bandName: string): Promise<{ url: string; id: string }> {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const url = page.url();
  return { url, id: url.split("/bands/")[1] };
}

async function createSongAndOpen(page: Page, title: string): Promise<string> {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  return page.url().split("/songs/")[1];
}

async function uploadPdf(page: Page) {
  // T36: file management moved into the editor's Details panel — open it to reach the
  // upload form, then close it so the canvas is unobstructed for whatever follows.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => top + bandH * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));
const renderCount = (page: Page) =>
  page.getByTestId("pdf-render-count").innerText().then((t) => parseInt(t, 10));

/** The first PDF page canvas's backing-store size (the rasterized pixels). */
async function pageCanvasSize(page: Page): Promise<{ w: number; h: number }> {
  return page.getByTestId("pdf-page").first().locator(".pdf-canvas").evaluate((el) => {
    const c = el as unknown as { width: number; height: number };
    return { w: c.width, h: c.height };
  });
}

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}

test("editor: adding/moving an annotation does NOT re-rasterize the PDF (no flicker)", async ({
  page,
}) => {
  await register(page, `flick_${stamp()}`);
  await createBandAndOpen(page, `FlickBand ${stamp()}`);
  await createSongAndOpen(page, `FlickSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Pin an EXPLICIT zoom so fit-width math can't be blamed; let it settle.
  await page.getByTestId("zoom-mode").selectOption("100");
  // Let the (re)render at the new scale finish and the count stabilize.
  await expect.poll(() => renderCount(page)).toBeGreaterThan(0);
  await page.waitForTimeout(300);

  const sizeBefore = await pageCanvasSize(page);
  const rendersBefore = await renderCount(page);
  expect(sizeBefore.w).toBeGreaterThan(0);

  // Create an editable layer + draw a rectangle.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(1);

  // Move it.
  await page.getByTestId("tool-select").click();
  await dragOnPage(page, 0.4, 0.4, 0.5, 0.5, 10);

  // Give any (erroneous) re-raster time to occur.
  await page.waitForTimeout(400);

  const sizeAfter = await pageCanvasSize(page);
  const rendersAfter = await renderCount(page);

  // The backing store is byte-for-byte the same size (no re-raster, no transient
  // wrong-scale), and the render counter never advanced for the edits.
  expect(sizeAfter).toEqual(sizeBefore);
  expect(rendersAfter).toBe(rendersBefore);

  await page.screenshot({ path: "/tmp/ed4-noflicker.png", fullPage: true });
});

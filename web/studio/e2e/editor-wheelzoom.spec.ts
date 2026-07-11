/**
 * T27 stage 1 — Ctrl/⌘-wheel zoom-to-cursor.
 *
 * The load-bearing invariant (Fable): a wheel/pinch zoom must DECOUPLE the
 * visual zoom from rasterization — the live zoom is a cheap CSS transform and
 * the crisp PDF re-raster is committed only ONCE, on wheel-settle. So a fast
 * burst of many Ctrl+wheel ticks must cost exactly ONE raster pass (one per
 * page), never one raster per tick.
 *
 * Two assertions:
 *  1. A burst of N Ctrl+wheel ticks changes the zoom but bumps `pdf-render-count`
 *     by exactly the page count (one pass) — not by N.
 *  2. Plain wheel (no ctrl/meta) does not zoom.
 *  3. After a zoom settles, a normal annotation edit STILL does not re-raster
 *     (the overlay-only edit path survives the new scale) — the no-flicker
 *     invariant holds post-zoom.
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
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}

const renderCount = (page: Page) =>
  page.getByTestId("pdf-render-count").innerText().then((t) => parseInt(t, 10));
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

/** Fire N Ctrl+wheel ticks at the scroll column's centre, synchronously (well
 *  inside the settle window) so the whole burst commits as one raster. */
async function ctrlWheelBurst(page: Page, ticks: number, deltaY: number) {
  await page.getByTestId("viewer-scroll").evaluate(
    (el, { n, dy }) => {
      const r = el.getBoundingClientRect();
      const cx = r.left + r.width / 2;
      const cy = r.top + r.height / 2;
      for (let i = 0; i < n; i++) {
        el.dispatchEvent(
          new WheelEvent("wheel", {
            deltaY: dy,
            ctrlKey: true,
            clientX: cx,
            clientY: cy,
            bubbles: true,
            cancelable: true,
          }),
        );
      }
    },
    { n: ticks, dy: deltaY },
  );
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

test("editor: a Ctrl+wheel burst zooms but re-rasters exactly once (not per tick)", async ({
  page,
}) => {
  await register(page, `wz_${stamp()}`);
  await createBandAndOpen(page, `WZBand ${stamp()}`);
  await createSongAndOpen(page, `WZSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Pin an explicit zoom so the base scale is deterministic; let it settle.
  await page.getByTestId("zoom-mode").selectOption("100");
  await expect.poll(() => renderCount(page)).toBeGreaterThan(0);
  await page.waitForTimeout(300);

  const pageCount = await page.getByTestId("pdf-page").count();
  const before = await renderCount(page);

  // A plain wheel (no ctrl) must NOT zoom.
  await page.getByTestId("viewer-scroll").evaluate((el) => {
    const r = el.getBoundingClientRect();
    el.dispatchEvent(
      new WheelEvent("wheel", {
        deltaY: -120,
        clientX: r.left + r.width / 2,
        clientY: r.top + r.height / 2,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await page.waitForTimeout(200);
  await expect(page.getByTestId("zoom-mode")).toHaveValue("100");
  expect(await renderCount(page)).toBe(before);

  // Now a fast Ctrl+wheel burst: many ticks, all inside the settle window.
  await ctrlWheelBurst(page, 8, -40);
  // Wait past the settle debounce and give the single re-raster time to land.
  await page.waitForTimeout(400);

  // Zoom changed (zoomed IN → a percentage above 100).
  const zoomVal = await page.getByTestId("zoom-mode").inputValue();
  expect(parseInt(zoomVal, 10)).toBeGreaterThan(100);

  // …but the PDF re-rasterized exactly ONCE (one pass = one bump per page),
  // NOT once per wheel tick. This is the decouple-visual-zoom invariant.
  const afterZoom = await renderCount(page);
  expect(afterZoom - before).toBe(pageCount);
});

test("editor: after a wheel-zoom settles, an annotation edit still does NOT re-raster", async ({
  page,
}) => {
  await register(page, `wz2_${stamp()}`);
  await createBandAndOpen(page, `WZBand ${stamp()}`);
  await createSongAndOpen(page, `WZSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("zoom-mode").selectOption("100");
  await expect.poll(() => renderCount(page)).toBeGreaterThan(0);
  await page.waitForTimeout(300);

  // Zoom in with a Ctrl+wheel burst and let it fully settle + re-raster.
  await ctrlWheelBurst(page, 8, -40);
  await page.waitForTimeout(400);
  const afterZoom = await renderCount(page);

  // Draw + move an annotation at the new scale.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(1);
  await page.getByTestId("tool-select").click();
  await dragOnPage(page, 0.4, 0.4, 0.5, 0.5, 10);
  await page.waitForTimeout(400);

  // The edit is overlay-only: no re-raster, even at the zoomed scale.
  expect(await renderCount(page)).toBe(afterZoom);
});

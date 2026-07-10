/**
 * T28 — drawing on a HIDDEN layer must never silently swallow the annotation.
 *
 * The wet canvas renders an in-progress stroke regardless of layer visibility, but
 * the committed dry overlay filters to the visible-layer set — so before the T28
 * auto-reveal, a stroke committed to a hidden active layer showed while drawing and
 * vanished forever at commit (VLL field bug, 2026-07-10: the "stabilo" that
 * disappeared at stroke end and after a tool change).
 *
 * This asserts the FIXED contract end to end, by pixels:
 *   - mid-stroke the wet canvas is painted (the visible-while-drawing contract);
 *   - committing the stroke AUTO-REVEALS the layer (its checkbox is checked again);
 *   - the committed object is painted on the dry overlay immediately (t0), after
 *     the sync echo settles (t1), and after switching tools (t2).
 */
import { test, expect, type Page } from "@playwright/test";
import { openDrawer, closeDrawer } from "./fullscreen-helpers";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, username: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

async function pageXY(page: Page, fx: number, fy: number) {
  const el = page.getByTestId("pdf-page").first();
  await el.scrollIntoViewIfNeeded();
  const box = (await el.boundingBox())!;
  return { x: box.x + box.width * fx, y: box.y + box.height * fy };
}

/** RGBA of one pixel of the first page's dry overlay, at page-relative (fx,fy). */
function overlayRGBA(page: Page, fx: number, fy: number): Promise<[number, number, number, number]> {
  return page
    .getByTestId("pdf-page")
    .first()
    .locator(".annotation-overlay")
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        const d = c
          .getContext("2d")!
          .getImageData(Math.floor(at.fx * c.width), Math.floor(at.fy * c.height), 1, 1).data;
        return [d[0], d[1], d[2], d[3]] as [number, number, number, number];
      },
      { fx, fy },
    );
}

test("drawing on a hidden layer auto-reveals it; the committed stroke stays painted", async ({
  page,
}) => {
  await register(page, `t28_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`T28 ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("Hidden Layer Repro");
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });

  // T27 stage 3: layer management lives in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");

  // A layer to draw on — then HIDE it (the bug's precondition).
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("layer-toggle").first().uncheck();
  await expect(page.getByTestId("layer-toggle").first()).not.toBeChecked();
  // Close the drawer: it overlays the ctx-bar's right end (presets), where Highlight
  // lives now (T33). The layer stays hidden; we reopen the drawer to re-check it below.
  await closeDrawer(page);

  // Draw a Highlight ("stabilo") rect across the page.
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("preset-highlight").click();
  const a = await pageXY(page, 0.2, 0.2);
  const b = await pageXY(page, 0.6, 0.3);
  await page.mouse.move(a.x, a.y);
  await page.mouse.down();
  await page.mouse.move(b.x, b.y, { steps: 8 });

  // Mid-stroke: the WET canvas must be painted (visible-while-drawing contract).
  const wetAlpha = await page
    .getByTestId("edit-canvas")
    .first()
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        return c
          .getContext("2d")!
          .getImageData(Math.floor(at.fx * c.width), Math.floor(at.fy * c.height), 1, 1).data[3];
      },
      { fx: 0.4, fy: 0.25 },
    );
  expect(wetAlpha, "the wet stroke must render while drawing").toBeGreaterThan(0);

  await page.mouse.up();
  await expect.poll(() => page.getByTestId("object-count").innerText()).toContain("1");

  // The commit must AUTO-REVEAL the hidden layer (T28) … (reopen the drawer to inspect
  // the layer's visibility toggle, which we closed for the preset click above).
  await openDrawer(page, "layers");
  await expect(page.getByTestId("layer-toggle").first()).toBeChecked();

  // … and the committed object must be painted: immediately (t0) …
  await expect
    .poll(() => overlayRGBA(page, 0.4, 0.25).then((px) => px[3]), {
      message: "committed highlight must be painted right after the stroke (t0)",
    })
    .toBeGreaterThan(0);

  // … after the sync echo settles (t1) …
  await page.waitForTimeout(2000);
  const t1 = await overlayRGBA(page, 0.4, 0.25);
  expect(t1[3], "committed highlight must survive the sync echo (t1)").toBeGreaterThan(0);

  // … and after switching tools (t2).
  await page.getByTestId("tool-select").click();
  await page.waitForTimeout(400);
  const t2 = await overlayRGBA(page, 0.4, 0.25);
  expect(t2[3], "committed highlight must survive a tool change (t2)").toBeGreaterThan(0);
});

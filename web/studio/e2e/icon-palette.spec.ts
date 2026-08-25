/**
 * T88 — the Icon-tool glyph palette hugs the page's left edge (outside it), clamping to the
 * viewport only when zoom pushes the page past it. The geometry is a pure helper (unit-tested);
 * the e2e confirms it is wired and still stampable.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer, closeDrawer, clearBand } from "./fullscreen-helpers";
import { iconPaletteLeft } from "../src/layout";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

const PDF = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

// ===========================================================================
// Unit — the pure clamp (no browser).
// ===========================================================================
test("layout: iconPaletteLeft hugs the page, clamps to the viewport (T88)", () => {
  const gap = 10, margin = 8, pw = 40;
  const vp = { left: 0, top: 0, right: 2000, bottom: 1000 };
  // narrow page, wide viewport → just outside the page's left edge
  expect(iconPaletteLeft({ left: 800, top: 0, right: 1200, bottom: 900 }, vp, pw, gap, margin)).toBe(800 - gap - pw);
  // page zoomed past the left edge → clamps to viewport + margin
  expect(iconPaletteLeft({ left: 4, top: 0, right: 3000, bottom: 4000 }, vp, pw, gap, margin)).toBe(vp.left + margin);
  // exactly enough room → takes the preferred position (== the clamp value here), not more
  const exactLeft = margin + gap + pw; // preferred lands exactly on the floor
  expect(iconPaletteLeft({ left: exactLeft, top: 0, right: 500, bottom: 500 }, vp, pw, gap, margin)).toBe(exactLeft - gap - pw);
});

test("layout: iconPaletteLeft never returns NaN or a negative left (T88)", () => {
  const zero = { left: 0, top: 0, right: 0, bottom: 0 };
  const l1 = iconPaletteLeft(zero, zero, 40, 10, 8); // degenerate zero-width viewport
  expect(Number.isFinite(l1)).toBe(true);
  expect(l1).toBeGreaterThanOrEqual(0);
  const l2 = iconPaletteLeft(zero, { left: 0, top: 0, right: 100, bottom: 100 }, NaN, 10, 8); // NaN width
  expect(Number.isFinite(l2)).toBe(true);
  expect(l2).toBeGreaterThanOrEqual(0);
});

// ===========================================================================
// Helpers.
// ===========================================================================
// Upload the PDF, reload (auto-selects the file → the page renders), then add an editable layer so
// the Icon tool is enabled. Leaves the layers drawer closed for a clean canvas.
async function iconSongReady(page: Page) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`IP ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible({ timeout: 12000 });
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
}
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10) || 0);
// Drag within the drawable band that clears the floating chrome (mirrors editor-icon-stamp).
async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number) {
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const a = { x: box.x + box.width * fx, y: top + bandH * fy };
  const b = { x: box.x + box.width * tx, y: top + bandH * ty };
  await page.mouse.move(a.x, a.y);
  await page.mouse.down();
  await page.mouse.move(b.x, b.y, { steps: 8 });
  await page.mouse.up();
}

// ===========================================================================
// e2e — wired to the real page geometry.
// ===========================================================================
test("editor: the icon palette hugs the page (outside its left edge) and still stamps (T88)", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 820 });
  await register(page, `ip_${stamp()}`);
  await createBandAndOpen(page, `IPBand ${stamp()}`);
  await iconSongReady(page);
  await page.getByTestId("zoom-mode").selectOption("fit-page");
  await page.waitForTimeout(400);

  await page.getByTestId("tool-icon").click();
  const palette = page.getByTestId("icon-palette");
  await expect(palette).toBeVisible();

  const pageBox = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const palBox = (await palette.boundingBox())!;
  // The palette HUGS the page's left edge: outside it, and only a small gap away — not stranded at
  // the far viewport edge (the bug). The small-gap bound is what the old `left:.75rem` dock fails.
  const gap = pageBox.x - (palBox.x + palBox.width);
  expect(gap).toBeGreaterThanOrEqual(0); // outside the page
  expect(gap).toBeLessThanOrEqual(14); // ~ the 10px hug gap, not a wide empty gutter
  expect(palBox.x).toBeGreaterThanOrEqual(0);

  // Pointer routing survives the reposition: pick a glyph and stamp (icon stamps on a drag).
  await page.getByTestId("icon-pick-mic").click();
  const before = await objectCount(page);
  await dragOnPage(page, 0.4, 0.35, 0.6, 0.6);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
});

test("editor: the icon palette clamps to the viewport when the page is zoomed past its edge (T88)", async ({ page }) => {
  await page.setViewportSize({ width: 900, height: 820 });
  await register(page, `ipc_${stamp()}`);
  await createBandAndOpen(page, `IPCBand ${stamp()}`);
  await iconSongReady(page);
  await page.getByTestId("zoom-mode").selectOption("300"); // page wider than the viewport
  await page.waitForTimeout(400);
  await page.getByTestId("tool-icon").click();
  const palette = page.getByTestId("icon-palette");
  await expect(palette).toBeVisible();

  const scrollBox = (await page.getByTestId("viewer-scroll").boundingBox())!;
  const pageBox = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const palBox = (await palette.boundingBox())!;
  const MARGIN = 8;
  // Premise: the page is now so wide that placing the palette OUTSIDE its left edge would push it
  // off-screen — so the clamp must take over.
  expect(pageBox.x - 10 - palBox.width).toBeLessThan(scrollBox.x + MARGIN);
  // The palette clamps to the viewport edge (scroll.left + margin), stays fully on-screen, and now
  // overlaps the page (intended — the palette is click-through except on its buttons).
  expect(palBox.x).toBeGreaterThanOrEqual(0);
  expect(Math.abs(palBox.x - (scrollBox.x + MARGIN))).toBeLessThanOrEqual(4);
  expect(palBox.x + palBox.width).toBeGreaterThan(pageBox.x);
});

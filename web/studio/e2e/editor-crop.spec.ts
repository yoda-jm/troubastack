// T198 — the Studio per-page crop tool (UI stage U1). Exercises the real in-memory core (the crop endpoint),
// not a mock. Covers: the authoring loop (draw → save → badge), the U1 blocker (a crop-mode drag that reaches
// a non-croppable file must create NO annotation), the Keep/Cancel warning naming the layer, and the passive
// veil that stays when the tool is off.
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 6) {
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

// Drag within a page's crop overlay, in overlay-relative fractions (the overlay is inset over the page).
async function dragCrop(page: Page, i: number, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const ov = page.getByTestId(`crop-overlay-${i}`);
  await expect(ov).toBeVisible();
  const b = (await ov.boundingBox())!;
  await page.mouse.move(b.x + b.width * fx, b.y + b.height * fy);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width * tx, b.y + b.height * ty, { steps });
  await page.mouse.up();
}

const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function openPdf(page: Page) {
  await register(page, `crop_${stamp()}`);
  await createBandAndOpen(page, `CropBand ${stamp()}`);
  await createSongAndOpen(page, `CropSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

test("crop: draw a bake area, save it, and the file is marked cropped — making no annotation", async ({ page }) => {
  await openPdf(page);
  const before = await objectCount(page);

  await page.getByTestId("tool-crop").click();
  await expect(page.getByTestId("crop-save")).toBeDisabled();

  // Draw the window → it appears with handles, and Save enables.
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-handle-0-nw")).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeEnabled();
  // Crop is NOT an annotation: nothing was added to the layer.
  expect(await objectCount(page)).toBe(before);

  // No annotations exist → Save commits straight away (no Keep/Cancel step), and the badge appears.
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeDisabled();

  // Escape during a REDRAW over a saved crop must RESTORE it, never wipe it. Start a new draw from the veil
  // corner (outside the window), move, then press Escape mid-gesture: the window stays and the draft matches
  // the saved crop (Save stays disabled). Pre-fix (draw recorded before:null) this cleared the page.
  const ov = page.getByTestId("crop-overlay-0");
  const b = (await ov.boundingBox())!;
  await page.mouse.move(b.x + b.width * 0.9, b.y + b.height * 0.9);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width * 0.75, b.y + b.height * 0.75, { steps: 5 });
  await page.keyboard.press("Escape");
  await page.mouse.up();
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeDisabled(); // restored to the saved crop → not dirty

  // The veil stays when the tool is off: switch to Move → the window still shows, but its handles are gone.
  await page.getByTestId("tool-move").click();
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-handle-0-nw")).toHaveCount(0);
});

test("crop: switching to a generated chart with the tool active creates no annotation (U1 blocker)", async ({ page }) => {
  await openPdf(page);

  // Add a generated chart to the song (a non-croppable file), and a layer on it so a stray draw COULD land.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("new-text-chart").click();
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await page.getByTestId("chart-source").fill("# Title\n\n## Verse\nsome words here\n");
  await page.getByTestId("chart-save").click();
  // Two files now in the strip.
  await expect(page.getByTestId("file-tab")).toHaveCount(2);

  // Open the chart and give it an editable layer.
  const chartTab = page.getByTestId("file-tab").nth(1);
  await chartTab.click();
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  const before = await objectCount(page);

  // Select Crop while the PDF is open...
  await page.getByTestId("file-tab").first().click();
  await page.getByTestId("tool-crop").click();
  // ...then switch to the chart: the crop tool must drop (not croppable) and NOT persist as a draw tool.
  await chartTab.click();
  await expect(page.getByTestId("tool-crop")).toHaveCount(0); // button gone for a generated chart
  await expect(page.getByTestId("tool-move")).toHaveAttribute("aria-pressed", "true"); // reset off crop

  // A drag on the chart must create NO annotation (pre-fix it saved a type:"" object).
  await dragOnPage(page, 0.2, 0.3, 0.6, 0.6);
  await page.waitForTimeout(300);
  expect(await objectCount(page)).toBe(before);
});

test("crop: a crop that hides a mark asks Keep/Cancel, naming the layer", async ({ page }) => {
  await openPdf(page);

  // Draw a rect annotation in the bottom-right corner on a named layer.
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.8, 0.8, 0.95, 0.95);
  await expect.poll(() => objectCount(page)).toBe(1);
  await closeDrawer(page); // so the floating chrome doesn't cover the crop Save button in the test viewport

  // Crop the top-left: the corner mark falls outside.
  await page.getByTestId("tool-crop").click();
  await dragCrop(page, 0, 0.05, 0.05, 0.4, 0.4);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible(); // the window drew
  // The live dry run flags the outside mark (checkpoint: the mark is detected as hidden).
  await expect(page.getByTestId("crop-warning")).toBeVisible();

  await page.getByTestId("crop-save").click();

  // The Keep/Cancel dialog appears and names the actual LAYER ("My notes", the default). A stubbed empty
  // layer-name list would red here.
  await expect(page.getByTestId("crop-confirm")).toBeVisible();
  await expect(page.getByTestId("crop-confirm")).toContainText("annotation");
  await expect(page.getByTestId("crop-confirm")).toContainText("My notes");
  await page.getByTestId("crop-confirm-cancel").click();
  await expect(page.getByTestId("crop-confirm")).toHaveCount(0);
  await expect(page.getByTestId("file-tab-cropped")).toHaveCount(0);

  // Save again → Keep → it commits and the badge appears.
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("crop-confirm")).toBeVisible();
  await page.getByTestId("crop-confirm-keep").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();
});

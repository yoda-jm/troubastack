/**
 * T161 — undo in the annotation editor. Undo APPENDS the inverse mutation (never rewrites history):
 *  1. Draw a stroke → undo → the object count returns AND the tool stays armed (undo is not a mode change).
 *  2. Delete a stroke → undo → it is RESTORED (same object comes back, via the Restore path).
 *  3. The undo button is disabled when there is nothing to undo, enabled after an action.
 *
 * Shared-canvas refusal (rule 2), permission re-check, and bounding are unit-tested on the pure core in
 * test/undo.test.ts; this spec covers the on-canvas wiring a browser is needed for.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function getAnnotations(page: Page, bandId: string, songId: string) {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as { objects: { uuid: string; type: string }[] };
    },
    [bandId, songId],
  );
}

// Draw/click constrained to the band CLEAR of the floating chrome (top pill + .ctx), matching
// editor.spec.ts — a raw page-box fraction can land under the chrome and miss the canvas.
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

async function clickOnPage(page: Page, fx: number, fy: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  await page.mouse.click(box.x + box.width * fx, top + bandH * fy);
}

const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
}

test("undo: draw then undo returns the count and leaves the tool armed", async ({ page }) => {
  await register(page, `undo_${stamp()}`);
  await createBandAndOpen(page, `UndoBand ${stamp()}`);
  await createSongAndOpen(page, `UndoSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  const before = await objectCount(page);
  await expect(page.getByTestId("undo")).toBeDisabled(); // nothing to undo yet

  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
  await expect(page.getByTestId("undo")).toBeEnabled();

  await page.getByTestId("undo").click();
  await expect.poll(() => objectCount(page)).toBe(before);

  // The tool stayed armed: drawing again (without re-picking the tool) makes another rect.
  await dragOnPage(page, 0.25, 0.3, 0.55, 0.5);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
});

test("undo: undoing a delete restores the object", async ({ page }) => {
  await register(page, `undodel_${stamp()}`);
  const band = await createBandAndOpen(page, `UndoDelBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `UndoDelSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  // Draw a rect and note its uuid (same coords as editor.spec.ts's proven delete flow).
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.3, 0.3, 0.7, 0.6);
  await expect.poll(() => objectCount(page)).toBe(1);
  const drawn = await getAnnotations(page, band.id, songId);
  const uuid = drawn.objects[0].uuid;

  // Select it and delete via the selection-toolbar button (routes through deleteSelected → records undo).
  await page.getByTestId("tool-select").click();
  await clickOnPage(page, 0.5, 0.45);
  await expect(page.getByTestId("delete-object")).toBeEnabled();
  await page.getByTestId("delete-object").click();
  await expect.poll(() => objectCount(page)).toBe(0);

  // Undo → restored, same uuid (the Restore path, not a re-create with a new id).
  await page.getByTestId("undo").click();
  await expect.poll(() => objectCount(page)).toBe(1);
  const after = await getAnnotations(page, band.id, songId);
  expect(after.objects.some((o) => o.uuid === uuid)).toBeTruthy();
});

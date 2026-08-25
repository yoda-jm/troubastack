/**
 * T40 guard: annotations must be scoped to the file they were drawn on. Layers bind to
 * one file (layer.fileId); the viewer's file tabs switch the PDF. Before the fix, the dry
 * overlay render (usePdfDocument) filtered objects by page-index + layer-visibility only —
 * NOT by the selected file — so a rect drawn on file A painted onto file B too (they share
 * page 0). VLL's field bug: "select Vocals/Guitar/Score → same annotations, different PDF."
 *
 * Reproducer: upload two PDFs, draw on file A, switch to file B → B's overlay must be
 * empty where A's rect was. Red-first (fails pre-fix: the ink bleeds across).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function uploadPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await page.getByTestId("my-files-edit").click();
}

// Alpha of the first page's dry overlay at page-relative (fx,fy).
const overlayAlpha = (page: Page, fx: number, fy: number) =>
  page
    .getByTestId("pdf-page")
    .first()
    .locator(".annotation-overlay")
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        return c.getContext("2d")!.getImageData(Math.floor(at.fx * c.width), Math.floor(at.fy * c.height), 1, 1)
          .data[3];
      },
      { fx, fy },
    );

test("annotations drawn on one file do NOT bleed onto another file of the same song (T40)", async ({
  page,
}) => {
  await register(page, `af_${stamp()}`);
  await createBandAndOpen(page, `AFBand ${stamp()}`);
  await createSongAndOpen(page, `AFSong ${stamp()}`);
  await uploadPdf(page); // file A
  await uploadPdf(page); // file B
  await page.reload();

  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await expect(page.getByTestId("file-tab")).toHaveCount(2);

  // On file A (tab 0): make a personal layer + draw a solid rect mid-page.
  await page.getByTestId("file-tab").nth(0).click();
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("preset-highlight").click(); // filled, so the interior has ink to sample
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  await page.mouse.move(box.x + box.width * 0.25, box.y + box.height * 0.25);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.6, box.y + box.height * 0.4, { steps: 8 });
  await page.mouse.up();

  // The rect is painted on file A's overlay (sanity — the draw worked).
  await expect
    .poll(() => overlayAlpha(page, 0.4, 0.3), { message: "rect must paint on file A" })
    .toBeGreaterThan(0);

  // File A's Layers drawer lists A's one layer.
  await openDrawer(page, "layers");
  await expect(page.getByTestId("layer-item")).toHaveCount(1);
  await closeDrawer(page);

  // Switch to file B (tab 1): its overlay must be EMPTY where A's rect was.
  await page.getByTestId("file-tab").nth(1).click();
  await expect(page.getByTestId("file-tab").nth(1)).toHaveAttribute("aria-selected", "true");
  await expect
    .poll(() => overlayAlpha(page, 0.4, 0.3), {
      message: "file B must NOT show file A's annotation (no cross-file bleed)",
    })
    .toBe(0);

  // And file B's Layers drawer must NOT list file A's layer (panel is file-scoped too).
  await openDrawer(page, "layers");
  await expect(page.getByTestId("layer-item")).toHaveCount(0);
  await closeDrawer(page);

  // Back to A: the rect is still there (scoping didn't lose it).
  await page.getByTestId("file-tab").nth(0).click();
  await expect.poll(() => overlayAlpha(page, 0.4, 0.3)).toBeGreaterThan(0);
});

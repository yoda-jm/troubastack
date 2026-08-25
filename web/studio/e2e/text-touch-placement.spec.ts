/**
 * T101 — the text-annotation prompt must survive a TOUCH placement.
 *
 * VLL reported that on a phone, tapping to add a text annotation opened no popup. Reproduced here:
 * WetCanvas opens the in-app prompt on `pointerdown` (finger still down); after `touchend` the
 * browser fires a compatibility `mousedown` targeted at whatever is under the finger. For a tap placed
 * OFF-CENTRE, that element is the just-mounted dialog backdrop — and the backdrop's dismiss handler
 * (Dialog.tsx) cancelled the dialog with the very same tap that opened it. The prompt only flashed.
 *
 * This exercises the touch path at a phone viewport — the environment the T90/T91 tests never drove
 * (they all use `page.mouse.click` at a desktop viewport, so the compat-mouse mechanism never fired
 * and the suite stayed green over a dead tool; same shape as the T99 `crypto.randomUUID` blind spot).
 *
 * The OFF-CENTRE tap is load-bearing: a centred tap lands the compat mousedown on the dialog CARD,
 * which never cancels — so a centred touch test passes even against the bug. Do not "simplify" the tap
 * to page centre.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { clearBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

test.use({ hasTouch: true, viewport: { width: 412, height: 915 } }); // a phone, the surface VLL uses

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function uploadPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function editorWithLayer(page: Page, tag: string) {
  await register(page, `t101${tag}_${stamp()}`);
  await createBandAndOpen(page, `T101${tag} ${stamp()}`);
  await createSongAndOpen(page, `T101${tag}Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page); // the side drawer covers the canvas at phone width
}

// An OFF-CENTRE point inside the clear band — deliberately away from the centred dialog card, so the
// compat mousedown after touchend lands on the backdrop, not the card.
async function offCentrePoint(page: Page) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  return { x: box.x + box.width * 0.5, y: top + bandH * 0.12 }; // near the top edge of the band
}

test("editor: a TOUCH tap opens the text prompt and it survives the tap that opened it (T101)", async ({
  page,
}) => {
  await editorWithLayer(page, "survive");
  const dialog = page.getByTestId("app-dialog");

  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("tool-text")).toHaveAttribute("aria-pressed", "true");

  const p = await offCentrePoint(page);
  await page.touchscreen.tap(p.x, p.y);

  // Let the tap fully settle — INCLUDING the browser's compatibility mouse events after touchend.
  // Against the bug the dialog is cancelled within this window; the assertion below then fails (red).
  await page.waitForTimeout(250);
  await expect(dialog).toBeVisible();

  // And it is usable: type + confirm actually places the annotation.
  await dialog.getByTestId("app-dialog-input").fill("From a phone");
  await dialog.getByTestId("app-dialog-confirm").click();
  await expect(dialog).toHaveCount(0);
  await expect.poll(() => objectCount(page)).toBe(1);

  // T90 still holds: the tool is one-shot — reverts to select after placing.
  await expect(page.getByTestId("tool-text")).toHaveAttribute("aria-pressed", "false");
});

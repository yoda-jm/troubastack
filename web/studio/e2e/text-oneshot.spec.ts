/**
 * T90 — the text tool must be ONE-SHOT. It used to stay armed forever, so on a phone every tap
 * anywhere opened another prompt. After the prompt resolves — placed OR cancelled — the tool returns
 * to `select`, so the next tap selects/deselects instead of re-prompting.
 *
 * T91 — the text prompt is now the in-app dialog (components/Dialog.tsx), not a native one, so these
 * tests drive `app-dialog` directly and assert it does NOT reappear on the second tap.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

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
async function editorWithLayer(page: Page, tag: string) {
  await register(page, `t90${tag}_${stamp()}`);
  await createBandAndOpen(page, `T90${tag} ${stamp()}`);
  await createSongAndOpen(page, `T90${tag}Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
}

test("editor: text is one-shot — after placing, the tool is select and a second tap adds nothing (T90)", async ({
  page,
}) => {
  await editorWithLayer(page, "place");
  const dialog = page.getByTestId("app-dialog");

  const before = await objectCount(page);
  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("tool-text")).toHaveAttribute("aria-pressed", "true");
  await clickOnPage(page, 0.35, 0.4);
  // the in-app text prompt opens; fill and confirm
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("app-dialog-input").fill("Hello");
  await dialog.getByTestId("app-dialog-confirm").click();
  await expect(dialog).toHaveCount(0);
  await expect.poll(() => objectCount(page)).toBe(before + 1);

  // one-shot: reverted to select, and the placed text is still selected (commit path unchanged)
  await expect(page.getByTestId("tool-select")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("tool-text")).toHaveAttribute("aria-pressed", "false");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // the behavioural half that matters: a SECOND tap opens NO prompt and creates NO object
  await clickOnPage(page, 0.62, 0.62);
  await page.waitForTimeout(300);
  await expect(dialog).toHaveCount(0); // no prompt reappeared
  expect(await objectCount(page)).toBe(before + 1);
});

test("editor: cancelling the text prompt also reverts to select and creates nothing (T90)", async ({
  page,
}) => {
  await editorWithLayer(page, "cancel");
  const dialog = page.getByTestId("app-dialog");

  const before = await objectCount(page);
  await page.getByTestId("tool-text").click();
  await clickOnPage(page, 0.35, 0.4);
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("app-dialog-cancel").click(); // cancel
  await expect(dialog).toHaveCount(0);
  await page.waitForTimeout(200);
  expect(await objectCount(page)).toBe(before); // nothing placed
  await expect(page.getByTestId("tool-select")).toHaveAttribute("aria-pressed", "true"); // reverted on cancel

  // second tap: still no prompt, still nothing
  await clickOnPage(page, 0.62, 0.62);
  await page.waitForTimeout(300);
  await expect(dialog).toHaveCount(0);
  expect(await objectCount(page)).toBe(before);
});

/**
 * T190 — the Text-annotation prompt: typing goes in straight away (no click on the field), and a note can
 * span several lines (Enter = newline, Ctrl/Cmd+Enter = add). Other prompts (tag rename) stay single-line.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { clearBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function texts(page: Page, bandId: string, songId: string): Promise<string[]> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const { objects } = (await r.json()) as { objects: { type: string; text?: string }[] };
      return objects.filter((o) => o.type === "text").map((o) => o.text ?? "");
    },
    [bandId, songId],
  );
}

async function editorWithLayer(page: Page, tag: string): Promise<{ bandId: string; songId: string }> {
  await register(page, `t190${tag}_${stamp()}`);
  const band = await createBandAndOpen(page, `T190${tag} ${stamp()}`);
  const songId = await createSongAndOpen(page, `T190${tag}Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
  return { bandId: band.id, songId };
}

// An off-centre point in the clear band (away from the centred dialog card, as T101 requires for touch).
async function clearPoint(page: Page) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  return { x: box.x + box.width * 0.5, y: top + bandH * 0.12, box, top, bandH };
}

// Max overlay alpha across a horizontal strip at page-fraction row fy — robust to landing between glyphs.
async function inkInRow(page: Page, fx0: number, fx1: number, fy: number): Promise<number> {
  return page
    .getByTestId("pdf-page")
    .first()
    .locator(".annotation-overlay")
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        const ctx = c.getContext("2d")!;
        const y = Math.floor(at.fy * c.height);
        let max = 0;
        for (let i = 0; i <= 20; i++) {
          const x = Math.floor((at.fx0 + ((at.fx1 - at.fx0) * i) / 20) * c.width);
          const a = ctx.getImageData(x, y, 1, 1).data[3];
          if (a > max) max = a;
        }
        return max;
      },
      { fx0, fx1, fy },
    );
}

test("focus (mouse): type right after the click, no field click, Ctrl+Enter adds it", async ({ page }) => {
  const { bandId, songId } = await editorWithLayer(page, "fm");
  await page.getByTestId("tool-text").click();
  const p = await clearPoint(page);
  await page.mouse.click(p.x, p.y);
  await expect(page.getByTestId("app-dialog")).toBeVisible();

  // Type WITHOUT clicking the field, then Ctrl+Enter.
  await page.keyboard.type("hello");
  await page.keyboard.press("Control+Enter");

  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
  await expect.poll(() => objectCount(page)).toBe(1);
  expect(await texts(page, bandId, songId)).toEqual(["hello"]);
});

test.describe("touch", () => {
  test.use({ hasTouch: true, viewport: { width: 412, height: 915 } });
  test("focus (touch): tap, then type without tapping the field, Ctrl+Enter adds it", async ({ page }) => {
    const { bandId, songId } = await editorWithLayer(page, "ft");
    await page.getByTestId("tool-text").click();
    const p = await clearPoint(page);
    await page.touchscreen.tap(p.x, p.y);
    await page.waitForTimeout(250); // let the compat mouse events after touchend settle (T101 window)
    await expect(page.getByTestId("app-dialog")).toBeVisible();

    await page.keyboard.type("from a phone");
    await page.keyboard.press("Control+Enter");

    await expect(page.getByTestId("app-dialog")).toHaveCount(0);
    expect(await texts(page, bandId, songId)).toEqual(["from a phone"]);
  });
});

test("multi-line: Enter makes a new line; the stored text keeps the newline and renders two lines", async ({
  page,
}) => {
  const { bandId, songId } = await editorWithLayer(page, "ml");
  await page.getByTestId("tool-text").click();
  const p = await clearPoint(page);
  await page.mouse.click(p.x, p.y);
  await expect(page.getByTestId("app-dialog")).toBeVisible();

  await page.keyboard.type("line one");
  await page.keyboard.press("Enter"); // a newline, NOT submit
  await expect(page.getByTestId("app-dialog")).toBeVisible(); // still open — Enter did not submit
  await page.keyboard.type("line two");
  await page.keyboard.press("Control+Enter"); // now add

  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
  expect(await texts(page, bandId, songId)).toEqual(["line one\nline two"]);

  // Two rendered lines: ink on line one's and line two's rows (anchor + 1 line step), none a row below.
  const fyAnchor = (p.y - p.box.y) / (p.box.height || 1);
  const step = 0.036; // fontSize 0.03 x 1.2, as a fraction of page HEIGHT
  await expect.poll(() => inkInRow(page, 0.5, 0.64, fyAnchor + 0.012)).toBeGreaterThan(0); // line one
  await expect.poll(() => inkInRow(page, 0.5, 0.64, fyAnchor + step + 0.012)).toBeGreaterThan(0); // line two
  expect(await inkInRow(page, 0.5, 0.64, fyAnchor + step * 3)).toBe(0); // nothing a third row below
});

test("Enter alone does not submit the multi-line prompt (the box stays open)", async ({ page }) => {
  await editorWithLayer(page, "ea");
  await page.getByTestId("tool-text").click();
  const p = await clearPoint(page);
  await page.mouse.click(p.x, p.y);
  await expect(page.getByTestId("app-dialog")).toBeVisible();

  await page.keyboard.type("one");
  await page.keyboard.press("Enter");
  await page.keyboard.press("Enter");
  // Still open, nothing placed.
  await expect(page.getByTestId("app-dialog")).toBeVisible();
  await expect.poll(() => objectCount(page)).toBe(0);
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
});

test("single-line prompts still SUBMIT on Enter (the file-rename prompt lands via Enter)", async ({ page }) => {
  // A single-line prompt whose submit has a direct, one-step effect (renaming the file) — no chained confirm,
  // so Enter's submit is cleanly observable. Contrast with the multi-line text prompt, where Enter is a
  // newline. Proves other prompt() callers stayed single-line.
  await register(page, `t190fr_${stamp()}`);
  await createBandAndOpen(page, `T190FR ${stamp()}`);
  await createSongAndOpen(page, `T190FRSong ${stamp()}`);

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-rename").click();
  const input = page.getByTestId("app-dialog-input");
  await expect(input).toBeVisible();
  await input.fill("renamed-by-enter.pdf");
  await expect(input).toHaveValue("renamed-by-enter.pdf");
  await input.press("Enter"); // Enter SUBMITS a single-line prompt (not a newline, not stuck open)

  await expect(panel.getByTestId("file-row")).toContainText("renamed-by-enter.pdf"); // the rename landed
});

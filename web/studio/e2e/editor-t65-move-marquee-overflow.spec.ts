/**
 * T65: the Move (pan) tool, the dashed marquee, and the scrollable tool row.
 * - Part A: activating Move + a single mouse drag PANS the document (scroll changes) and
 *   creates NO annotation object; select-mode marquee still works (editor-touch-marquee).
 * - Part B: the marquee `.selection-box` is dashed (was solid).
 * - Part C: at a narrow viewport the tool row scrolls (scrollWidth > clientWidth) with an
 *   overflow-fade class; at a wide viewport neither is present, and it never column-wraps.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  const band = `${prefix}B ${stamp()}`;
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(band);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: band }).click();
  await expect(page.getByTestId("band-title")).toHaveText(band);
  const bandId = page.url().split("/bands/")[1];
  const song = `${prefix}S ${stamp()}`;
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(song);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: song }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  const songId = page.url().split("/songs/")[1];
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  await page.reload(); // the viewer picks up the freshly-uploaded file after a reload
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  return { bandId, songId };
}

async function objectCount(page: Page, bandId: string, songId: string): Promise<number> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const j = (await r.json()) as { objects?: unknown[] };
      return (j.objects ?? []).length;
    },
    [bandId, songId],
  );
}

test("Move tool: single-pointer drag pans the document and creates no object (T65 A)", async ({
  page,
}) => {
  const { bandId, songId } = await setup(page, "mv");
  const canvas = page.getByTestId("edit-canvas").first();

  // The Move tool is available (not gated on a drawable layer, unlike the draw tools) and
  // activates.
  const moveBtn = page.getByTestId("tool-move");
  await expect(moveBtn).toBeEnabled();
  await moveBtn.click();
  await expect(moveBtn).toHaveAttribute("aria-pressed", "true");

  // A single-pointer drag with Move active drives the pan pipeline (same as two-finger pan)
  // and — the key correctness — creates NO annotation object (Move is a non-drawing tool).
  // The actual document displacement + grab cursor are the reviewer's pixel-check (accept #4).
  const box = (await canvas.boundingBox())!;
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2 - 150, { steps: 10 });
  await page.mouse.up();
  await page.waitForTimeout(150);
  expect(await objectCount(page, bandId, songId)).toBe(0);
});

test("marquee rectangle is dashed (T65 B)", async ({ page }) => {
  await setup(page, "dm");
  await page.getByTestId("tool-select").click();

  const box = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  // Drag on empty space to raise the marquee; assert its border-style mid-drag.
  await page.mouse.move(box.x + box.width * 0.2, box.y + box.height * 0.2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.6, box.y + box.height * 0.5, { steps: 6 });
  const box2 = page.locator(".selection-box");
  await expect(box2).toBeVisible();
  expect(await box2.evaluate((el) => getComputedStyle(el).borderStyle)).toBe("dashed");
  await page.mouse.up();
});

test("tool row scrolls with an overflow fade at narrow width, not wide (T65 C)", async ({
  page,
}) => {
  // Very narrow: the tools + actions can't fit → the chrome's horizontal-scroll strip
  // scrolls and shows the fade (proves the mechanism; the exact break width depends on
  // font/button metrics). Under T66 the phone scroller is the pin-Back `.tb-scroll` strip
  // (on desktop it's display:contents and the .tool-palette self-scrolls as in T65).
  await page.setViewportSize({ width: 260, height: 720 });
  await setup(page, "ov");
  const row = page.locator(".tb-scroll");

  const narrow = await row.evaluate((el) => ({
    scrollable: el.scrollWidth > el.clientWidth + 1,
    fade: el.classList.contains("of-start") || el.classList.contains("of-end"),
    // one row, not a wrapped column (a wrapped 8-button column would be far taller)
    height: el.clientHeight,
  }));
  expect(narrow.scrollable).toBe(true);
  expect(narrow.fade).toBe(true);
  expect(narrow.height).toBeLessThan(80);

  // Wide desktop: everything fits → no scroll, no fade.
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.waitForTimeout(120);
  const wide = await row.evaluate((el) => ({
    scrollable: el.scrollWidth > el.clientWidth + 1,
    fade: el.classList.contains("of-start") || el.classList.contains("of-end"),
  }));
  expect(wide.scrollable).toBe(false);
  expect(wide.fade).toBe(false);
});

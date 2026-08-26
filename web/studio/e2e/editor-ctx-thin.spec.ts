/**
 * T33 — the contextual style bar is ONE slim row, no taller than the main top bar.
 *
 * It used to be ~2× the top pill (three-layer label/control/value stacks). The
 * acceptance number: `ctx-bar` height ≤ `topbar-pill` height + 2px, for BOTH a shape
 * target (rect: color/opacity/width/presets/⋯) and a text target (color/opacity/font).
 * Also guards that the ⋯ overflow popover opens and Blend works inside it (fill/border/
 * blend/hex moved there to keep the row slim).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}
const height = async (page: Page, sel: string) => (await page.locator(sel).boundingBox())!.height;

test("ctx style bar is one slim row (≤ top-bar height + 2px), shape and text", async ({ page }) => {
  await register(page, `ct_${stamp()}`);
  await createBandAndOpen(page, `CTBand ${stamp()}`);
  await createSongAndOpen(page, `CTSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const topH = await height(page, ".viewer-chrome.topbar-pill");

  // Shape target: activating a draw tool shows the contextual style row.
  await page.getByTestId("tool-rect").click();
  await expect(page.getByTestId("style-controls")).toBeVisible();
  const shapeH = await height(page, ".ctx-bar");
  expect(shapeH).toBeLessThanOrEqual(topH + 2);

  // Text target.
  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("style-controls")).toBeVisible();
  const textH = await height(page, ".ctx-bar");
  expect(textH).toBeLessThanOrEqual(topH + 2);

  // The ⋯ overflow popover opens and Blend still works inside it.
  await page.getByTestId("tool-rect").click(); // shape → Blend applies
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-popover")).toBeVisible();

  // CLASS-KILLER (arch HOLD): `toBeVisible` does NOT catch a popover clipped out of
  // paint by an ancestor's overflow (Playwright's actionability scrolls the clip
  // container to reach it; a human can't). Probe the real hit-test: the top element at
  // the Blend select's own centre must BE the select (or its descendant) — not the
  // canvas behind. This fails on the clipped (position:absolute-in-overflow) version.
  const hitOk = await page.getByTestId("style-blend").evaluate((sel) => {
    const r = sel.getBoundingClientRect();
    const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return hit != null && (hit === sel || sel.contains(hit) || hit.contains(sel));
  });
  expect(hitOk, "the ⋯ popover's Blend must be the top element at its own centre (not clipped/covered)").toBe(true);

  // ANCHOR (arch HOLD round 3): the ctx bar has `transform: translateX(-50%)`, which
  // makes it the containing block for `position: fixed` — so an in-bar fixed panel
  // floats ~300px off. Portaling to <body> restores viewport-relative fixed. Assert the
  // panel is docked to the ⋯ trigger: right edges align, and it sits just below.
  const btnBox = (await page.getByTestId("style-more").boundingBox())!;
  const popBox = (await page.getByTestId("style-popover").boundingBox())!;
  expect(Math.abs(popBox.x + popBox.width - (btnBox.x + btnBox.width))).toBeLessThanOrEqual(8);
  const gap = popBox.y - (btnBox.y + btnBox.height);
  expect(gap).toBeGreaterThanOrEqual(0);
  expect(gap).toBeLessThanOrEqual(12);

  await page.getByTestId("style-blend").selectOption("multiply");
  await expect(page.getByTestId("style-blend")).toHaveValue("multiply");
});

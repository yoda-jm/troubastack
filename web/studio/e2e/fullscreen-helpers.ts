/**
 * Shared e2e mechanics for the T27 stage-3 fullscreen editor.
 *
 * The chrome floats over the canvas: a slim top pill, a contextual `.ctx` style
 * pill (only while a draw tool / selection is active), a bottom parts+status pill,
 * and an on-demand right drawer (Layers | Annotations, closed by default). So a
 * gesture must land in the "clear band" — the canvas NOT covered by that chrome —
 * and any layer/annotation control must have the drawer opened first.
 *
 * These helpers change only HOW a test reaches the canvas/controls, never WHAT it
 * asserts (arch ruling 2026-07-10, option (a): assertion-freeze). They are the
 * single home for the sanctioned mechanics so the migration is legible.
 */
import { type Page, expect } from "@playwright/test";

/** The vertical span of canvas NOT covered by the floating chrome: below the top
 *  pill (and the `.ctx` style pill when it is showing), above the bottom pill. */
export async function clearBand(page: Page): Promise<{ top: number; bottom: number }> {
  const vh = page.viewportSize()?.height ?? 720;
  const chrome = await page.getByTestId("viewer-chrome").boundingBox();
  const ctxEl = page.getByTestId("ctx-bar");
  const ctx = (await ctxEl.count()) ? await ctxEl.boundingBox() : null;
  const barEl = page.getByTestId("viewer-bottombar");
  const bar = (await barEl.count()) ? await barEl.boundingBox() : null;
  const top =
    Math.max(0, chrome ? chrome.y + chrome.height : 0, ctx ? ctx.y + ctx.height : 0) + 10;
  const bottom = (bar ? Math.min(vh, bar.y) : vh) - 10;
  return { top, bottom };
}

/** Scroll the viewer so the given page-fraction(s) sit inside the clear band, then
 *  return the (possibly moved) first pdf-page box for true page-relative mapping. */
export async function scrollFracIntoBand(
  page: Page,
  ...pys: number[]
): Promise<{ x: number; y: number; width: number; height: number }> {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  let box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  // Center the requested fraction(s) at the band midpoint. Centering (rather than
  // only nudging when the AVERAGE is out of band) guarantees a whole DRAG RANGE
  // (e.g. a resize from 0.26→0.42) stays clear of the bottom pill — otherwise the
  // average can read "in band" while an endpoint sits off-screen below the fold.
  const frac = pys.length ? pys.reduce((a, b) => a + b, 0) / pys.length : 0.5;
  const mid = (top + bottom) / 2;
  const dy = box.y + box.height * frac - mid;
  if (Math.abs(dy) > 4) {
    await page
      .getByTestId("viewer-scroll")
      .evaluate((s, d) => s.scrollBy(0, d), Math.round(dy));
    await page.waitForTimeout(80);
    box = (await pageEl.boundingBox())!;
  }
  return box;
}

/** Open the on-demand drawer to a tab (default Layers), if not already on it.
 *  Idempotent: does nothing when the wanted tab is already showing. */
export async function openDrawer(page: Page, tab: "layers" | "annotations" = "layers"): Promise<void> {
  const pill = tab === "layers" ? page.getByTestId("sidebar-toggle") : page.getByTestId("drawer-notes");
  const alreadyOnTab = (await pill.getAttribute("aria-pressed")) === "true";
  if (!alreadyOnTab) await pill.click();
  await expect(page.getByTestId("viewer-drawer")).toBeVisible();
}

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
  // Keep band-mapped gestures BELOW the .ctx (its centered controls would otherwise
  // capture a draw), even though the .ctx is now pointer-events pass-through — the
  // pass-through only rescues gestures that unavoidably start under it (e.g. large
  // top-of-page draws at fit-page), it isn't a license to aim at its controls.
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
  // Scroll ONLY when the requested fraction RANGE doesn't already fit inside the
  // clear band. Checking the whole range (min…max), not just the average, keeps a
  // drag's endpoint from sitting under the bottom pill; and NOT scrolling when it
  // already fits avoids sub-pixel drift that pushes a pick off-target (e.g. when the
  // page fits the viewport at fit-page). When a scroll is needed, center the range.
  const ys = pys.length ? pys : [0.5];
  const lo = Math.min(...ys);
  const hi = Math.max(...ys);
  const mappedLo = box.y + box.height * lo;
  const mappedHi = box.y + box.height * hi;
  if (mappedLo < top + 8 || mappedHi > bottom - 8) {
    const mid = (top + bottom) / 2;
    const rangeMid = box.y + box.height * ((lo + hi) / 2);
    await page
      .getByTestId("viewer-scroll")
      .evaluate((s, d) => s.scrollBy(0, d), Math.round(rangeMid - mid));
    await page.waitForTimeout(80);
    box = (await pageEl.boundingBox())!;
  }
  return box;
}

/** Open the on-demand file rail to a tab (default Layers), if not already on it. T94: one pill opens
 *  the rail, the Layers/Notes tabs switch inside it. Idempotent. */
export async function openDrawer(page: Page, tab: "layers" | "annotations" = "layers"): Promise<void> {
  const railPill = page.getByTestId("sidebar-toggle");
  if ((await railPill.getAttribute("aria-pressed")) !== "true") await railPill.click();
  await expect(page.getByTestId("viewer-drawer")).toBeVisible();
  const tabBtn = page.getByTestId(tab === "layers" ? "drawer-layers" : "drawer-notes");
  if ((await tabBtn.getAttribute("aria-pressed")) !== "true") await tabBtn.click();
  // T94 §3.1b — ASSERT we landed on the requested tab. Without this, a helper that ignored `tab`
  // would make every openDrawer(page,"annotations") silently exercise Layers and ~20 specs' annotation
  // assertions would test nothing (a dangling-testid failure in helper form).
  await expect(page.getByTestId(tab === "layers" ? "layers-panel" : "annotation-list")).toBeVisible();
}

/** Dismiss the rail if open, so a following canvas gesture (draw / resize / drag) on the right side
 *  of the score is not intercepted by the floating panel. T94: the single pill toggles the rail. */
export async function closeDrawer(page: Page): Promise<void> {
  if (!(await page.getByTestId("viewer-drawer").count())) return;
  const railPill = page.getByTestId("sidebar-toggle");
  if ((await railPill.getAttribute("aria-pressed")) === "true") await railPill.click();
  await expect(page.getByTestId("viewer-drawer")).toHaveCount(0);
}

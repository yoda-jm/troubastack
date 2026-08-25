/**
 * A11y viewport contract (WCAG 1.4.4) — the guard for `50e0ce8` / arch note #3.
 *
 * The SPA is zoomable everywhere EXCEPT the editor route, where the T27 stage-4
 * in-app pinch owns the gesture and browser zoom would fight it. `index.html` ships
 * the zoomable default for first paint; `Shell` sets `user-scalable=no` on the
 * viewport meta only while on `/bands/:b/songs/:s` and restores the default on leave.
 *
 * Without this spec nothing gates the predicate against drift (the reshape that put
 * `user-scalable=no` on the global meta is exactly the regression class here). The
 * five legs mirror the architect's re-verification: management zoomable, editor
 * clamped, restore on SPA back-nav, and clamp-on-mount after a hard reload landing
 * directly on the editor route (first paint is the zoomable index default).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

// Read the live viewport meta content (the exact node Shell mutates).
const viewportContent = (page: Page) =>
  page.locator('meta[name="viewport"]').getAttribute("content");

// The meta is set by a Shell useEffect (async after a route change), so poll.
const expectZoomable = (page: Page) =>
  expect.poll(() => viewportContent(page)).not.toContain("user-scalable=no");
const expectClamped = (page: Page) =>
  expect.poll(() => viewportContent(page)).toContain("user-scalable=no");

test("viewport meta: zoomable on management pages, clamped only on the editor route", async ({
  page,
}) => {
  await register(page, `va_${stamp()}`);
  // (1) Post-registration /bands — zoomable (no in-app zoom here to fall back on).
  await expect(page).toHaveURL(/\/bands$/);
  await expectZoomable(page);

  // (2) Band page — still zoomable.
  await createBandAndOpen(page, `VABand ${stamp()}`);
  await expectZoomable(page);

  // (3) Editor route — the meta gains user-scalable=no (in-app pinch owns the gesture).
  await createSongAndOpen(page, `VASong ${stamp()}`);
  const editorUrl = page.url();
  await expectClamped(page);

  // (4) SPA history back to the band page — the zoomable default is restored.
  await page.goBack();
  await expect(page.getByTestId("band-title")).toBeVisible();
  await expectZoomable(page);

  // (5) Hard reload landing DIRECTLY on the editor route: first paint is index.html's
  // zoomable default; the Shell effect must clamp it on mount.
  await page.goto(editorUrl);
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  await page.reload();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  await expectClamped(page);
});

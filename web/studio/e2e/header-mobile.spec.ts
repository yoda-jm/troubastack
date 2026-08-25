/**
 * Phone header guard (Pixel 7, 412px): the app header stays on ONE row — brand + nav +
 * the account trigger all inline, the trigger pinned right, nothing clipped, every
 * control tappable; and the account menu never runs off either screen edge.
 *
 * History: T47 compacted a ~3-row header to two rows (user cluster on its own line);
 * T58 then made the trigger avatar-only, leaving a lone avatar stranded on row 2. VLL
 * (2026-07-26) asked for it inline — one row. This guard now encodes that.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register } from "./setup-helpers";

test.use({ viewport: { width: 412, height: 915 }, deviceScaleFactor: 2.625, isMobile: true, hasTouch: true });

// elementFromPoint reachability: the element (or a descendant) is what's actually on top
// at its own center — i.e. nothing clips/occludes it (the T33-era class-killer probe).
async function reachable(page: Page, testid: string): Promise<boolean> {
  return page.getByTestId(testid).evaluate((el) => {
    const r = el.getBoundingClientRect();
    if (r.width === 0 || r.height === 0) return false;
    const at = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return !!at && (at === el || el.contains(at) || at.contains(el));
  });
}

test("app header stays on one row on a phone, account trigger inline (VLL 2026-07-26)", async ({ page }) => {
  await register(page, `hm_${stamp()}`);
  const header = page.locator(".topbar");
  await expect(header).toBeVisible();

  // Structural ONE row (font-independent, unlike a height threshold headless can't
  // reproduce): brand, nav, AND the user cluster all share row 1 — their vertical extents
  // overlap. Pre-fix the user cluster took its own full-width row 2 (order:2,
  // flex-basis:100%), so "user shares row 1 with the brand" fails — a real red-first.
  const rows = await page.evaluate(() => {
    const r = (s: string) => {
      const e = document.querySelector(s);
      if (!e) return null;
      const b = e.getBoundingClientRect();
      return { top: b.top, bottom: b.bottom, left: b.left, right: b.right };
    };
    return { brand: r(".brand"), nav: r(".nav"), user: r(".user") };
  });
  expect(rows.brand && rows.nav && rows.user).toBeTruthy();
  // brand & nav on the same row: their vertical spans overlap.
  expect(rows.nav!.top, "nav shares row 1 with the brand").toBeLessThan(rows.brand!.bottom);
  expect(rows.nav!.bottom, "nav shares row 1 with the brand").toBeGreaterThan(rows.brand!.top);
  // the user cluster is ALSO on row 1 (inline), not stranded on a second line.
  expect(rows.user!.top, "user cluster shares row 1 with the brand").toBeLessThan(rows.brand!.bottom);
  expect(rows.user!.bottom, "user cluster shares row 1 with the brand").toBeGreaterThan(rows.brand!.top);
  // and it's pinned to the right, past the nav.
  expect(rows.user!.left, "account cluster pinned right of the nav").toBeGreaterThanOrEqual(rows.nav!.right - 1);
  // The verbose account name is hidden at phone width (avatar-only trigger).
  await expect(page.locator(".account-name")).toBeHidden();

  // Every topbar control genuinely tappable (nothing clipped/occluded). T58 folded
  // profile/version/logout into the single account trigger; the row-2 controls are
  // now the Invites nav link and the account trigger.
  for (const id of ["nav-invites", "account-trigger"]) {
    expect(await reachable(page, id), `${id} must be tappable`).toBe(true);
  }

  // Nothing overflows the viewport horizontally.
  const overflowX = await page.evaluate(
    () => document.documentElement.scrollWidth - document.documentElement.clientWidth,
  );
  expect(overflowX, "no horizontal overflow").toBeLessThanOrEqual(1);
});

test("account menu stays fully on-screen on a phone (T47/T58)", async ({ page }) => {
  await register(page, `hp_${stamp()}`);
  await page.getByTestId("account-trigger").click();
  const pop = page.getByTestId("account-menu");
  await expect(pop).toBeVisible();
  const box = await pop.evaluate((el) => {
    const b = el.getBoundingClientRect();
    return { left: Math.round(b.left), right: Math.round(b.right), vw: window.innerWidth };
  });
  expect(box.left, `popover left ${box.left} must be ≥ 0 (not off the left edge)`).toBeGreaterThanOrEqual(0);
  expect(box.right, `popover right ${box.right} must be ≤ viewport ${box.vw}`).toBeLessThanOrEqual(box.vw);
});

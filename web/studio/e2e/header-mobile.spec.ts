/**
 * T47 guard: on a phone (Pixel 7, 412px) the app header must stay to ≤ 2 tidy rows
 * (was ~3 — brand, then user, then nav), nothing clipped, every control tappable; and
 * the version popover must never run off either screen edge (was right:0 + 240px min →
 * ~117px off the left). VLL, Pixel 7. Ruling: compact header (approach a) + popover clamp.
 *
 * Red-first: pre-fix the header is ~150px (3 rows) and the popover's left is negative.
 */
import { test, expect, type Page } from "@playwright/test";

test.use({ viewport: { width: 412, height: 915 }, deviceScaleFactor: 2.625, isMobile: true, hasTouch: true });

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill("Marie");
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

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

test("app header stays ≤ 2 rows on a phone, all controls reachable (T47)", async ({ page }) => {
  await register(page, `hm_${stamp()}`);
  const header = page.locator(".topbar");
  await expect(header).toBeVisible();

  // Structural ≤ 2 rows (font-independent, unlike a height threshold headless can't
  // reproduce): the brand + nav share ROW 1 (their vertical extents overlap), and the
  // user cluster sits on ROW 2 (its top is at/below the brand's bottom). Pre-fix the nav
  // was forced to its own full-width row BELOW the user (order:3, flex-basis:100%), so
  // "brand and nav share a row" fails — a real red-first.
  const rows = await page.evaluate(() => {
    const r = (s: string) => {
      const e = document.querySelector(s);
      if (!e) return null;
      const b = e.getBoundingClientRect();
      return { top: b.top, bottom: b.bottom };
    };
    return { brand: r(".brand"), nav: r(".nav"), user: r(".user") };
  });
  expect(rows.brand && rows.nav && rows.user).toBeTruthy();
  // brand & nav on the same row: their vertical spans overlap.
  expect(rows.nav!.top, "nav shares row 1 with the brand").toBeLessThan(rows.brand!.bottom);
  expect(rows.nav!.bottom, "nav shares row 1 with the brand").toBeGreaterThan(rows.brand!.top);
  // user cluster on the row below.
  expect(rows.user!.top, "user cluster is on row 2, below the brand").toBeGreaterThanOrEqual(rows.brand!.bottom - 2);

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

/**
 * T46 guard: Studio "embedded mode" for the app's WebView host. `?embedded=1` on the
 * entry URL suppresses the app topbar (Bands/Invites/profile/Log out — it duplicates the
 * app chrome) and removes the Log out affordance (the app owns the cookie-seeded session).
 * The flag persists across SPA navigation (sessionStorage). Without the param, Studio is
 * unchanged. Signal is a URL param (not the JS bridge) → testable in plain Playwright.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

test("embedded=1 hides the topbar + logout, and survives SPA navigation (T46)", async ({ page }) => {
  await register(page, `em_${stamp()}`);
  // Create a band (normal Studio) so there's a link to SPA-navigate later.
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`EmBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await expect(page.getByTestId("band-link").first()).toBeVisible();

  // Enter embedded mode via the entry-URL param (fresh load).
  await page.goto("/bands?embedded=1");
  await expect(page.getByTestId("band-link").first()).toBeVisible(); // Studio content still there
  await expect(page.locator(".topbar")).toHaveCount(0); // app topbar suppressed
  await expect(page.getByTestId("account-trigger")).toHaveCount(0); // no account menu (⇒ no logout/version/get-app)
  await expect(page.getByTestId("nav-invites")).toHaveCount(0);

  // Client-side navigation to another page: the flag must survive (param is gone now).
  await page.getByTestId("band-link").first().click();
  await expect(page).toHaveURL(/\/bands\/[^/]+$/);
  expect(page.url()).not.toContain("embedded="); // param not in the URL anymore
  await expect(page.locator(".topbar")).toHaveCount(0); // still suppressed → flag persisted
  await expect(page.getByTestId("logout")).toHaveCount(0);
});

test("without ?embedded=1 the topbar + logout are present (T46 regression)", async ({ page }) => {
  await register(page, `nm_${stamp()}`);
  // Fresh context (no embedded flag leaks from the other test).
  await expect(page.locator(".topbar")).toHaveCount(1);
  await expect(page.getByTestId("account-trigger")).toBeVisible(); // account menu present (hosts Log out / version / get-app)
  await expect(page.getByTestId("nav-invites")).toBeVisible();
});

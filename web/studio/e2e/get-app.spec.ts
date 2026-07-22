/**
 * OPS02 "Get the app", now a T58 account-menu item: open the account menu (top-right)
 * → "Get the app" opens the reused QR/download popover. The item appears ONLY when the
 * server carries an app (/api/apps non-empty). The e2e stack runs core without
 * TROUBA_APPS_DIR, so the item is absent by default; the present cases are driven by
 * intercepting /api/apps with a fake manifest. The iOS row reads "Coming soon" until an
 * ios entry rides the manifest, then flips to a live download.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

/** Open the top-right account menu (the "Get the app" item lives inside it now). */
const openMenu = (page: Page) => page.getByTestId("account-trigger").click();

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/); // lands on the shell topbar
}

const androidEntry = {
  platform: "android",
  version: "v9.9.9",
  size: 7_340_032,
  path: "/apps/troubashare.apk",
  filename: "troubashare-v9.9.9.apk",
};
const iosEntry = {
  platform: "ios",
  version: "v9.9.9",
  size: 6_291_456,
  path: "/apps/troubashare.ipa",
  filename: "troubashare-v9.9.9.ipa",
};

const mockApps = (page: Page, apps: object[]) =>
  page.route("**/api/apps", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ apps }) }),
  );

test("no Get-the-app menu item when no apps are embedded", async ({ page }) => {
  await register(page, `noapp_${stamp()}`);
  await openMenu(page);
  await expect(page.getByTestId("get-app-btn")).toHaveCount(0);
});

test("menu item opens a popover: Android download + QR + iOS coming-soon", async ({ page }) => {
  await mockApps(page, [androidEntry]);
  await register(page, `app_${stamp()}`);

  await openMenu(page);
  const btn = page.getByTestId("get-app-btn");
  await expect(btn).toBeVisible();
  await btn.click();
  await expect(page.getByTestId("get-app-popover")).toBeVisible();

  const dl = page.getByTestId("get-app-download");
  await expect(dl).toHaveAttribute("href", "/apps/troubashare.apk");
  await expect(dl).toHaveAttribute("download", "troubashare-v9.9.9.apk");
  await expect(page.getByTestId("get-app-version")).toContainText("v9.9.9");
  await expect(page.getByTestId("get-app-version")).toContainText("Android");

  // iOS is a greyed "Coming soon" row — present, not a link.
  await expect(page.getByTestId("get-app-ios-soon")).toBeVisible();
  await expect(page.getByTestId("get-app-ios-download")).toHaveCount(0);

  // QR is client-rendered as an inline SVG.
  await expect(page.locator('[data-testid="get-app-qr"] svg')).toBeVisible();
});

test("iOS row flips to a live download once the manifest carries ios", async ({ page }) => {
  await mockApps(page, [androidEntry, iosEntry]);
  await register(page, `ios_${stamp()}`);

  await openMenu(page);
  await page.getByTestId("get-app-btn").click();
  await expect(page.getByTestId("get-app-popover")).toBeVisible();
  // The SAME iOS row is now a live download — no coming-soon.
  await expect(page.getByTestId("get-app-ios-soon")).toHaveCount(0);
  await expect(page.getByTestId("get-app-ios-download")).toHaveAttribute("href", "/apps/troubashare.ipa");
});

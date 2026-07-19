/**
 * OPS02: the band page shows a "Get the app" card ONLY when the server carries an
 * app binary (image built with the APK). The e2e stack runs core without
 * TROUBA_APPS_DIR, so /api/apps is empty by default → no card; the card-present
 * case is driven by intercepting /api/apps with a fake manifest.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}

test("band page hides the Get-the-app card when no apps are embedded", async ({ page }) => {
  await register(page, `noapp_${stamp()}`);
  await createBandAndOpen(page, `NoApp ${stamp()}`);
  // Dev stack has no TROUBA_APPS_DIR → empty manifest → card hidden.
  await expect(page.getByTestId("get-app-card")).toHaveCount(0);
});

test("band page shows the Get-the-app card (download + QR) when the server carries an app", async ({
  page,
}) => {
  // Intercept the manifest with a fake Android binary.
  await page.route("**/api/apps", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        apps: [
          {
            platform: "android",
            version: "v9.9.9",
            size: 7_340_032,
            path: "/apps/troubashare.apk",
            filename: "troubashare-v9.9.9.apk",
          },
        ],
      }),
    }),
  );

  await register(page, `app_${stamp()}`);
  await createBandAndOpen(page, `App ${stamp()}`);

  const card = page.getByTestId("get-app-card");
  await expect(card).toBeVisible();

  const dl = page.getByTestId("get-app-download");
  await expect(dl).toHaveAttribute("href", "/apps/troubashare.apk");
  await expect(dl).toHaveAttribute("download", "troubashare-v9.9.9.apk");
  await expect(page.getByTestId("get-app-version")).toContainText("v9.9.9");
  await expect(page.getByTestId("get-app-version")).toContainText("Android");

  // The QR is client-rendered as an inline SVG of the absolute APK URL.
  await expect(page.locator('[data-testid="get-app-qr"] svg')).toBeVisible();
});

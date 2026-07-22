/**
 * T29 build-identity, now inside the T58 account menu: the menu footer shows the
 * SPA's baked version + the server's /api/version (fetched eagerly). A mismatch
 * (server upgraded under a stale browser bundle, or vice versa) flags a warning line
 * AND a glanceable dot on the account trigger, so it isn't buried in a closed menu.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, username: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

test("account menu footer shows Studio + server builds; mismatch flags a warning + trigger dot", async ({
  page,
}) => {
  await register(page, `ver_${stamp()}`);

  // Open the account menu → the footer renders the SPA version + the server's.
  await page.getByTestId("account-trigger").click();
  await expect(page.getByTestId("version-popover")).toBeVisible();
  await expect(page.getByTestId("version-server")).toContainText("Server");
  // Dev stack: vite dev serves the config-defined version and the go-run server
  // reports "dev" — they may legitimately match or differ here, so the MATCH case
  // just asserts no crash; the mismatch UX is asserted deterministically below.
  await page.keyboard.press("Escape"); // close

  // Force a mismatch: intercept /api/version with a different version string.
  await page.route("**/api/version", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ version: "vDIFFERENT-123", builtAt: "2099-01-01T00:00Z", spaEmbedded: true }),
    }),
  );
  await page.reload();
  // The dot is glanceable on the trigger BEFORE opening the menu (eager fetch).
  await expect(page.getByTestId("account-warning-dot")).toBeVisible();
  await page.getByTestId("account-trigger").click();
  await expect(page.getByTestId("version-mismatch")).toBeVisible();
  await expect(page.getByTestId("version-mismatch")).toContainText("differ");
});

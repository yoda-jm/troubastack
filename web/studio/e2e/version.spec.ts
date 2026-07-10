/**
 * T29 — build-identity chip: the Shell shows the SPA's baked version; clicking it
 * fetches the server's /api/version. Matching versions → no warning; a mismatch
 * (server upgraded under a stale browser bundle, or vice versa) → a visible flag.
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

test("version chip shows Studio + server builds; mismatch is flagged", async ({ page }) => {
  await register(page, `ver_${stamp()}`);

  // Chip renders the SPA's own version and opens the popover with the server's.
  const chip = page.getByTestId("version-chip");
  await expect(chip).toBeVisible();
  const spaVersion = (await chip.innerText()).trim();
  expect(spaVersion.length).toBeGreaterThan(0);

  await chip.click();
  await expect(page.getByTestId("version-popover")).toBeVisible();
  await expect(page.getByTestId("version-server")).toContainText("Server");
  // Dev stack: vite dev serves the config-defined version and the go-run server
  // reports "dev" — they may legitimately match or differ here, so the MATCH case
  // just asserts no crash; the mismatch UX is asserted deterministically below.
  await chip.click(); // close

  // Force a mismatch: intercept /api/version with a different version string.
  await page.route("**/api/version", (route) =>
    route.fulfill({
      contentType: "application/json",
      body: JSON.stringify({ version: "vDIFFERENT-123", builtAt: "2099-01-01T00:00Z", spaEmbedded: true }),
    }),
  );
  await page.reload();
  await page.getByTestId("version-chip").click();
  await expect(page.getByTestId("version-mismatch")).toBeVisible();
  await expect(page.getByTestId("version-mismatch")).toContainText("differ");
});

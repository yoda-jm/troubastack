/**
 * B02: an admin bakes a setlist and a download link + history appear.
 *
 * Uses an EMPTY setlist so the bake needs no external toolchain (poppler/web-bake
 * are only invoked for songs with a PDF) — this exercises the Studio Bake card +
 * the core bake/list/download endpoints end to end. The full song→raster→overlay
 * loop is covered by the Go tests (B01 parity + B02 orchestration).
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

test("admin bakes a setlist → download link + history appear", async ({ page }) => {
  await register(page, `bake_${stamp()}`);
  await createBandAndOpen(page, `BakeBand ${stamp()}`);

  // Create a setlist and open it.
  await page.getByTestId("nav-setlists").click();
  await expect(page).toHaveURL(/\/setlists$/);
  await page.getByTestId("setlist-name").fill(`Gig ${stamp()}`);
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // The admin-only Bake card is present; no bake exists yet.
  await expect(page.getByTestId("bake-card")).toBeVisible();
  await expect(page.getByTestId("bake-download")).toHaveCount(0);

  // Bake → a download link and a history row appear (rev 1).
  await page.getByTestId("bake-setlist").click();
  await expect(page.getByTestId("bake-download")).toBeVisible();
  await expect(page.getByTestId("bake-download")).toHaveAttribute(
    "href",
    /\/concerts\/[^/]+\/bundle$/,
  );
  await expect(page.getByTestId("bake-history-row")).toHaveCount(1);
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 1");

  // Re-bake bumps the revision.
  await page.getByTestId("bake-setlist").click();
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 2");
  await expect(page.getByTestId("bake-download")).toContainText("rev 2");
});

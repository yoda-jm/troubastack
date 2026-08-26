/**
 * B02: an admin bakes a setlist and a download link + history appear.
 *
 * Uses an EMPTY setlist so the bake needs no external toolchain (poppler/web-bake
 * are only invoked for songs with a PDF) — this exercises the Studio Bake card +
 * the core bake/list/download endpoints end to end. The full song→raster→overlay
 * loop is covered by the Go tests (B01 parity + B02 orchestration).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

test("admin bakes a setlist → download link + history appear", { tag: "@smoke" }, async ({ page }) => {
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
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(page.getByTestId("bake-download")).toBeVisible();
  await expect(page.getByTestId("bake-download")).toHaveAttribute(
    "href",
    /\/concerts\/[^/]+\/bundle$/,
  );
  await expect(page.getByTestId("bake-history-row")).toHaveCount(1);
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 1");

  // Re-bake bumps the revision.
  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 2");
  await expect(page.getByTestId("bake-download")).toContainText("rev 2");
});

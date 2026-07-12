/**
 * P201 stage 2b — the in-editor LIVE banner.
 *
 * A song is in a setlist; the admin puts that setlist in rehearsal live mode; opening
 * the song's editor shows the fixed LIVE banner ("your edits are publishing to
 * performers"). Turning live mode off makes the banner disappear (polled).
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

test("editor shows the LIVE banner when the song's setlist is live", async ({ page }) => {
  const s = stamp();
  await register(page, `elb_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`ElbBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `ElbBand ${s}` }).click();
  const bandUrl = page.url();

  // A song.
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("Wonderwall");
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: "Wonderwall" })).toBeVisible();
  const songUrl = await page
    .getByTestId("song-link")
    .filter({ hasText: "Wonderwall" })
    .getAttribute("href");

  // A setlist containing the song.
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Rehearsal");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await page.getByTestId("add-item-song").selectOption({ label: "Wonderwall" });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(1);

  // Open the editor BEFORE going live → no banner.
  await page.goto(songUrl!);
  await expect(page.getByTestId("pdf-page").first().or(page.getByTestId("song-viewer"))).toBeVisible();
  await expect(page.getByTestId("editor-live-banner")).toHaveCount(0);

  // Admin flips the setlist live (in another tab-equivalent: navigate back).
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-link").first().click();
  await page.getByTestId("live-toggle").click();
  await expect(page.getByTestId("live-banner")).toBeVisible();

  // Back in the editor, the banner appears (poll on load).
  await page.goto(songUrl!);
  await expect(page.getByTestId("editor-live-banner")).toBeVisible();
  await expect(page.getByTestId("editor-live-banner")).toContainText(/publishing to performers/i);
});

/**
 * P201 stage 2a — rehearsal live mode toggle + banner on the setlist page.
 *
 * An admin flips "Go live (rehearsal)" → the LIVE banner appears and the state
 * persists across a reload; flipping it off clears the banner. (The autobake it
 * enables is core-side, tested in Go; here we cover the admin control + indicator.)
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

test("admin toggles rehearsal live mode; the banner shows + persists", async ({ page }) => {
  const s = stamp();
  await register(page, `live_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`LiveBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `LiveBand ${s}` }).click();
  const bandUrl = page.url();

  // A song, so the setlist isn't empty (not required for the toggle, but realistic).
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("The Open Road");
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: "The Open Road" })).toBeVisible();

  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Rehearsal");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // Off by default: no banner, the toggle offers to go live.
  await expect(page.getByTestId("live-banner")).toHaveCount(0);
  const toggle = page.getByTestId("live-toggle");
  await expect(toggle).toHaveText(/Go live/i);

  // Go live → banner appears, toggle flips to Stop.
  await toggle.click();
  await expect(page.getByTestId("live-banner")).toBeVisible();
  await expect(toggle).toHaveText(/Stop live/i);

  // Persists across a reload (server-side state).
  await page.reload();
  await expect(page.getByTestId("live-banner")).toBeVisible();
  await expect(page.getByTestId("live-toggle")).toHaveText(/Stop live/i);

  // Stop → banner clears.
  await page.getByTestId("live-toggle").click();
  await expect(page.getByTestId("live-banner")).toHaveCount(0);
  await expect(page.getByTestId("live-toggle")).toHaveText(/Go live/i);
});

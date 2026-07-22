/**
 * T61: a setlist item's title links to that song's editor page. A real router <Link>
 * (so middle/ctrl-click + keyboard work), hover-only affordance, and drag-to-reorder
 * (grip-only) is unaffected.
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
async function createBandAndOpen(page: Page, name: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(name);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("band-title")).toHaveText(name);
}
async function addSong(page: Page, bandUrl: string, title: string) {
  await page.goto(bandUrl);
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: title })).toBeVisible();
}

test("setlist item title links to the song editor; reorder still works (T61)", async ({ page }) => {
  await register(page, `sl_${stamp()}`);
  await createBandAndOpen(page, `LinkBand ${stamp()}`);
  const bandUrl = page.url();
  for (const t of ["Aaa", "Bbb"]) await addSong(page, bandUrl, t);

  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Show");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Show" }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  const labels = ["Aaa", "Bbb"];
  for (let i = 0; i < labels.length; i++) {
    // Wait for each row before the next add — the async add-reload resets the select.
    await page.getByTestId("add-item-song").selectOption({ label: labels[i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }
  const setlistUrl = page.url();

  // The title is a REAL anchor to the song editor route (middle/ctrl-click work).
  const firstLink = page.getByTestId("item-row").first().getByTestId("item-title-link");
  await expect(firstLink).toHaveAttribute("href", /\/bands\/[^/]+\/songs\/[^/]+$/);

  // Plain click navigates to that song's editor.
  await firstLink.click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);

  // Reorder (grip drag) still works with the link present: drag row 2 onto row 1.
  await page.goto(setlistUrl);
  await expect(page.getByTestId("item-title").nth(0)).toContainText("Aaa");
  const rows = page.getByTestId("item-row");
  const grip = rows.nth(1).getByTestId("item-grip");
  const target = rows.nth(0);
  await grip.dragTo(target);
  await expect(page.getByTestId("item-title").nth(0)).toContainText("Bbb");
});

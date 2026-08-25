/**
 * T20: duplicating a setlist yields an independent, editable copy.
 * Duplicate → land on the copy → rename it → both the original and the renamed
 * copy are listed.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

test("duplicate a setlist → independent, editable copy; both listed", async ({ page }) => {
  await register(page, `dup_${stamp()}`);
  await createBandAndOpen(page, `DupBand ${stamp()}`);
  const bandUrl = page.url();

  // A song to put in the setlist.
  const song = `Alpha ${stamp()}`;
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(song);
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: song })).toBeVisible();

  // Create a setlist "Show" with the song.
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Show");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Show" }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  await page.getByTestId("add-item-song").selectOption({ label: song });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(1);
  const originalUrl = page.url();

  // Duplicate → navigates to the copy. Wait on the copy's title (only present once
  // the copy page has loaded) before asserting the URL actually changed.
  await page.getByTestId("duplicate-setlist").click();
  await expect(page.getByTestId("setlist-detail-title")).toHaveText("Show (copy)");
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  expect(page.url()).not.toBe(originalUrl);
  await expect(page.getByTestId("item-row")).toHaveCount(1); // item carried over

  // The copy is independently editable: rename it.
  await page.getByTestId("sl-name").fill("Show — August");
  await page.getByTestId("sl-save").click();
  await expect(page.getByTestId("sl-notice")).toBeVisible();

  // Both the original and the renamed copy are listed.
  await page.goto(page.url().replace(/\/setlists\/[^/]+$/, "/setlists"));
  await expect(page).toHaveURL(/\/setlists$/);
  await expect(page.getByTestId("setlist-link").filter({ hasText: "Show" }).first()).toBeVisible();
  await expect(page.getByTestId("setlist-link").filter({ hasText: "Show — August" })).toBeVisible();
});

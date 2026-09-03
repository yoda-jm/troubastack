/**
 * T131: fast actions on the concert row — bake/re-bake (reusing the detail page's bake dialog +
 * T103 kick-and-poll), and PDF + bundle links that appear only once a bake exists. Verified by an
 * ACTUAL bake (a song with no PDF needs no poppler/web-bake toolchain), not a click-handler unit test.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, createSetlist } from "./setup-helpers";

test("concert row: bake from the row, then PDF + bundle appear; an empty setlist can't bake", async ({
  page,
}) => {
  await register(page, `rowb_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `RowBand ${stamp()}`);
  const songTitle = `Open Road ${stamp()}`;
  await createSongAndOpen(page, songTitle); // no PDF → real bake needs no toolchain

  await page.goto(`/bands/${bandId}/setlists`);
  const emptyName = `Empty ${stamp()}`;
  await createSetlist(page, emptyName);
  const gigName = `Gig ${stamp()}`;
  await createSetlist(page, gigName);

  // Put the song on the Gig setlist (from its detail page), then return to the list.
  await page.getByTestId("setlist-link").filter({ hasText: gigName }).click();
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toContainText(songTitle);
  await page.goto(`/bands/${bandId}/setlists`);

  const row = (name: string) =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });

  // Empty setlist: the row's bake action is DISABLED, with the detail page's exact guard wording.
  await row(emptyName).getByTestId("setlist-menu").click();
  const emptyBake = page.getByTestId("setlist-rebake");
  await expect(emptyBake).toBeDisabled();
  await expect(emptyBake).toHaveAttribute("title", /Add at least one song/);
  await page.keyboard.press("Escape");

  // Gig setlist, not yet baked: bake action enabled, but NO PDF/bundle items (never a 404 link).
  await row(gigName).getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-rebake")).toBeEnabled();
  await expect(page.getByTestId("setlist-pdf")).toHaveCount(0);
  await expect(page.getByTestId("setlist-bundle")).toHaveCount(0);

  // Bake it from the row: confirm (naming the concert) → the SAME bake dialog → terminal.
  await page.getByTestId("setlist-rebake").click();
  await page.getByTestId("app-dialog-confirm").click(); // "Bake “Gig …”?"
  await page.getByTestId("bake-dialog-confirm").click();
  await expect(page.getByTestId("bake-dialog")).toBeHidden();

  // After baking, the list refreshes and the row now offers PDF + bundle.
  await row(gigName).getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-pdf")).toBeVisible();
  await expect(page.getByTestId("setlist-bundle")).toBeVisible();
});

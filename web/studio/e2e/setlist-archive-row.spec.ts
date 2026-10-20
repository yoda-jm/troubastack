/**
 * T200: archive a concert from its row. Archiving moves it out of the main (date-ordered) list into a
 * collapsed "Archives (N)" section; unarchiving brings it back. Admin-only, no confirm (reversible, and the
 * move is the feedback) — mirrors the live-row toggle (T132).
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

test("row archive: concert moves to Archives and unarchive brings it back", async ({ page }) => {
  await register(page, `arch_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `ArchBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `ArchGig ${stamp()}`;
  await createSetlist(page, name);

  const row = () =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });
  const inMain = () => page.getByTestId("setlists-list").getByTestId("setlist-link").filter({ hasText: name });
  const inArchives = () =>
    page.getByTestId("archives-list").getByTestId("setlist-link").filter({ hasText: name });

  // Starts in the main list; no Archives section yet.
  await expect(inMain()).toHaveCount(1);
  await expect(page.getByTestId("archives-section")).toHaveCount(0);

  // Archive from the row menu — immediate, no confirm dialog.
  await row().getByTestId("setlist-menu").click();
  await page.getByTestId("setlist-archive").click();
  await expect(page.getByTestId("app-dialog")).toHaveCount(0);

  // It left the main list and sits under "Archives (1)".
  await expect(inMain()).toHaveCount(0);
  await expect(page.getByTestId("archives-toggle")).toHaveText("Archives (1)");

  // Expand the section and unarchive.
  await page.getByTestId("archives-toggle").click();
  await expect(inArchives()).toHaveCount(1);
  await page.getByTestId("archives-list").getByTestId("setlist-unarchive").click();

  // Back in the main list; the Archives section is gone.
  await expect(inMain()).toHaveCount(1);
  await expect(page.getByTestId("archives-section")).toHaveCount(0);
});

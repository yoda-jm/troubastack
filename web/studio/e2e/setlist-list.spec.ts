/**
 * T127 — the concerts list is a working list: the create form is a popup (list not lost below the
 * fold), the per-row "…" menu duplicates without navigating, and the past is folded under a "Past"
 * heading. Ordering/partition logic is unit-tested (test/setlist-order.test.ts); this covers the UI.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

test("the create form is a popup, so the first concert row is visible without scrolling (390px)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 780 });
  await register(page, `cl_${stamp()}`);
  const { id } = await createBandAndOpen(page, `CL ${stamp()}`);
  await page.goto(`/bands/${id}/setlists`);
  await createSetlist(page, "First Gig");
  // The form collapses back to the "+ New concert" button after create, so the row is up top.
  await expect(page.getByTestId("new-setlist-btn")).toBeVisible();
  await expect(page.getByTestId("setlist-link").filter({ hasText: "First Gig" })).toBeInViewport();
});

test('the "…" menu opens without navigating, and Duplicate from a row stays on the list', async ({
  page,
}) => {
  await register(page, `dup_${stamp()}`);
  const { id } = await createBandAndOpen(page, `Dup ${stamp()}`);
  await page.goto(`/bands/${id}/setlists`);
  await createSetlist(page, "Gig A");

  // Tapping "…" must open the menu, NOT navigate into the concert (the trigger is a sibling of the link).
  await page.getByTestId("setlist-menu").click();
  await expect(page).toHaveURL(/\/setlists$/);
  await expect(page.getByTestId("setlist-duplicate")).toBeVisible();

  // Duplicate from the list → stay on the list, and the copy appears.
  await page.getByTestId("setlist-duplicate").click();
  await expect(page).toHaveURL(/\/setlists$/);
  await expect(page.getByTestId("setlist-link")).toHaveCount(2);
  await expect(page.getByTestId("setlist-link").filter({ hasText: "Gig A (copy)" })).toBeVisible();
});

test("past concerts are folded under a muted Past heading, upcoming above it", async ({ page }) => {
  await register(page, `past_${stamp()}`);
  const { id } = await createBandAndOpen(page, `Past ${stamp()}`);
  await page.goto(`/bands/${id}/setlists`);
  await createSetlist(page, "Old Gig", { date: "2020-01-01" });
  await createSetlist(page, "Future Gig", { date: "2099-12-31" });

  await expect(page.getByTestId("setlists-past-heading")).toBeVisible();
  // Upcoming row comes before the Past heading; the past row comes after it.
  const items = page.locator('[data-testid="setlist-link"], [data-testid="setlists-past-heading"]');
  await expect(items.nth(0)).toContainText("Future Gig");
  await expect(items.nth(1)).toHaveText("Past");
  await expect(items.nth(2)).toContainText("Old Gig");
});

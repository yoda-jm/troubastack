/**
 * T132: rehearsal live mode from the concert row — a status chip (dot + the WORD "Live", static under
 * reduced motion) and an admin ⋯ toggle that confirms on ARM (naming the concert + the 3h window) but
 * is immediate on DISARM, sharing the SAME endpoint as the detail card.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

test("row live: arm confirms + chip pulses; disarm is immediate; the detail card agrees", async ({
  page,
}) => {
  await register(page, `live_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `LiveBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `LiveGig ${stamp()}`;
  await createSetlist(page, name);
  const row = () =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });

  // Not live: no chip; the menu offers "Arm live mode".
  await expect(row().getByTestId("setlist-live")).toHaveCount(0);
  await row().getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-live-toggle")).toHaveText("Arm live mode");

  // Arm → the confirm names the concert AND states the 3-hour window.
  await page.getByTestId("setlist-live-toggle").click();
  await expect(page.getByTestId("app-dialog")).toContainText(name);
  await expect(page.getByTestId("app-dialog-body")).toContainText(/3 hours/);
  await page.getByTestId("app-dialog-confirm").click();

  // The chip shows the WORD (the signal), and its dot (::before) pulses by default.
  const chip = row().getByTestId("setlist-live");
  await expect(chip).toBeVisible();
  await expect(chip).toHaveText(/Live/);
  expect(await chip.evaluate((el) => getComputedStyle(el, "::before").animationName)).not.toBe("none");

  // The menu now toggles the other way; disarm is IMMEDIATE — no confirm dialog.
  await row().getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-live-toggle")).toHaveText("Disarm live mode");
  await page.getByTestId("setlist-live-toggle").click();
  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
  await expect(row().getByTestId("setlist-live")).toHaveCount(0);

  // Arm again from the row, then open the detail: the LiveModeCard reflects the SAME state.
  await row().getByTestId("setlist-menu").click();
  await page.getByTestId("setlist-live-toggle").click();
  await page.getByTestId("app-dialog-confirm").click();
  await expect(row().getByTestId("setlist-live")).toBeVisible();
  await row().getByTestId("setlist-link").click();
  await expect(page.getByTestId("live-toggle")).toHaveText(/Stop live mode/);
});

test("the live chip is static under prefers-reduced-motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await register(page, `livem_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `RMBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `RMGig ${stamp()}`;
  await createSetlist(page, name);
  const row = () =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });

  await row().getByTestId("setlist-menu").click();
  await page.getByTestId("setlist-live-toggle").click();
  await page.getByTestId("app-dialog-confirm").click();
  const chip = row().getByTestId("setlist-live");
  await expect(chip).toBeVisible();
  // colour still signals; the pulse does not — the dot's animation is off under reduced motion.
  expect(await chip.evaluate((el) => getComputedStyle(el, "::before").animationName)).toBe("none");
});

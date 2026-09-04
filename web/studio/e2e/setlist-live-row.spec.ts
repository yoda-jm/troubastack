/**
 * T132: rehearsal live mode from the concert row — a status chip (dot + the WORD "Live", static under
 * reduced motion) and an admin ⋯ toggle. No confirm (VLL): the menu is anchored to the row and arming
 * is instantly reversible, so the 3-hour consequence rides in the LABEL (read on touch), not a dialog.
 * The toggle shares the SAME endpoint as the detail card.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

test("row live: arm is immediate (label states 3h, no dialog) → chip pulses; disarm; detail agrees", async ({
  page,
}) => {
  await register(page, `live_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `LiveBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `LiveGig ${stamp()}`;
  await createSetlist(page, name);
  const row = () =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });

  // Not live: no chip; the menu's arm item CARRIES the 3-hour consequence in its label (no hover needed).
  await expect(row().getByTestId("setlist-live")).toHaveCount(0);
  await row().getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-live-toggle")).toHaveText(/Arm live mode · auto-bakes for 3 h/);

  // Arm → IMMEDIATE, no confirm dialog; the chip appearing is the feedback.
  await page.getByTestId("setlist-live-toggle").click();
  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
  const chip = row().getByTestId("setlist-live");
  await expect(chip).toBeVisible();
  await expect(chip).toHaveText(/Live/); // the WORD is the signal
  expect(await chip.evaluate((el) => getComputedStyle(el, "::before").animationName)).not.toBe("none");

  // The menu now reads the other way; disarm is also immediate.
  await row().getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-live-toggle")).toHaveText("Disarm live mode");
  await page.getByTestId("setlist-live-toggle").click();
  await expect(page.getByTestId("app-dialog")).toHaveCount(0);
  await expect(row().getByTestId("setlist-live")).toHaveCount(0);

  // Arm again from the row, then open the detail: the LiveModeCard reflects the SAME state.
  await row().getByTestId("setlist-menu").click();
  await page.getByTestId("setlist-live-toggle").click();
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
  const chip = row().getByTestId("setlist-live");
  await expect(chip).toBeVisible();
  // colour still signals; the pulse does not — the dot's animation is off under reduced motion.
  expect(await chip.evaluate((el) => getComputedStyle(el, "::before").animationName)).toBe("none");
});

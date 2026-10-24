import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// A band with two songs + the common parts seeded, open on the Line-ups tab.
async function seededBand(page: Page, u: string): Promise<string> {
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  for (const title of ["Alpha", "Bravo"]) {
    await page.request.post(`/api/bands/${id}/songs`, { data: { title } });
  }
  await page.getByTestId("nav-lineups").click();
  await page.getByTestId("lineups-add-common").click();
  await expect(page.getByTestId("lineup-cell-voice").first()).toBeVisible();
  return id;
}

// T203 M4 — keyboard navigation across the grid + reorder (buttons; drag shares the same path).
test("line-up grid: arrow-key cell nav + musician reorder persists", async ({ page }) => {
  const u = `m4${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  // A second musician (so there are two columns to reorder) + two songs.
  const g = await page.request.post(`/api/bands/${id}/musicians`, { data: { name: "Zoe Guest" } });
  expect(g.ok()).toBeTruthy();
  for (const title of ["Alpha", "Bravo"]) {
    expect((await page.request.post(`/api/bands/${id}/songs`, { data: { title } })).ok()).toBeTruthy();
  }

  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await page.getByTestId("lineups-add-common").click();
  await expect(page.getByTestId("lineup-cell-voice").first()).toBeVisible();

  // --- Keyboard navigation: focus the first voice cell, ArrowRight moves the roving tabstop to the next cell. ---
  const first = page.getByTestId("lineup-cell-voice").first();
  await first.focus();
  await expect(first).toBeFocused();
  const startCol = await first.getAttribute("data-col");
  await page.keyboard.press("ArrowRight");
  const focusedCol = await page.locator(":focus").getAttribute("data-col");
  expect(Number(focusedCol)).toBe(Number(startCol) + 1); // moved one column right
  // ArrowDown then moves to the next row at the same column.
  const focusedRow = await page.locator(":focus").getAttribute("data-row");
  await page.keyboard.press("ArrowDown");
  expect(Number(await page.locator(":focus").getAttribute("data-row"))).toBe(Number(focusedRow) + 1);

  // --- Reorder: the creator is first; move them right; after reload the order has swapped (persisted). ---
  const firstOwner0 = await page.getByTestId("lineup-col-owner").first().textContent();
  expect(firstOwner0).toContain(`Display ${u}`);
  await page.getByTestId("lineup-musician-right").first().click();
  await page.reload();
  await expect(page.getByTestId("lineup-col-owner").first()).toContainText("Zoe Guest");
});

// Binding 2: L/B keys on a focused Voice cell set/clear the part; Shift adds (L+B).
test("voice cell: L / B keys set and clear, Shift adds", async ({ page }) => {
  await seededBand(page, `m4k${stamp()}`);
  const vc = () => page.getByTestId("lineup-cell-voice").first();
  await vc().focus();
  await page.keyboard.press("l"); // lead ("Lead vocal" → LV)
  await expect(vc()).toHaveText("LV");
  await vc().focus();
  await page.keyboard.press("l"); // same key clears
  await expect(vc()).toHaveText("·");
  await vc().focus();
  await page.keyboard.press("b"); // backing
  await expect(vc()).toHaveText("BV");
  await vc().focus();
  await page.keyboard.press("Shift+l"); // Shift ADDS lead → both
  await expect(vc()).toContainText("LV");
  await expect(vc()).toContainText("BV");
});

// Binding 1: a chip filter that shrinks the rows must not orphan the roving tabstop.
test("roving tabstop survives a filter that shrinks the rows", async ({ page }) => {
  await seededBand(page, `m4c${stamp()}`);
  await expect(page.getByTestId("lineup-row")).toHaveCount(2);
  // Put the tabstop on a cell in the SECOND row (the one the filter will remove).
  const last = page.getByTestId("lineup-cell-voice").last();
  await expect(last).toHaveAttribute("data-row", "1");
  await last.focus();
  await expect(page.locator('.lineup-cell[tabindex="0"]')).toHaveCount(1);
  // Filter down to one row — the second row (where the tabstop was) is gone.
  await page.getByTestId("lineups-filter").fill("Alpha");
  await expect(page.getByTestId("lineup-row")).toHaveCount(1);
  // A cell still carries tabIndex 0, so the grid is still reachable by Tab.
  await expect(page.locator('.lineup-cell[tabindex="0"]')).toHaveCount(1);
});

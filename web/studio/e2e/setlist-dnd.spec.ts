/**
 * Setlist running order: drag-and-drop reordering via the row grip handle
 * (the ↑/↓ buttons remain and are covered by flows.spec test 8). Drag the third
 * song onto the first row → it moves to the top and the numbering follows.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register } from "./setup-helpers";

test("drag a running-order song to the top; numbering follows", async ({ page }) => {
  const s = stamp();
  await register(page, `dnd_${s}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`DndBand ${s}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: `DndBand ${s}` }).click();
  const bandUrl = page.url();

  for (const t of ["Aaa", "Bbb", "Ccc"]) {
    await page.goto(bandUrl);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(t);
    await page.getByTestId("create-song").click();
    await expect(page.getByTestId("song-link").filter({ hasText: t })).toBeVisible();
  }

  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Gig");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  for (let i = 0; i < 3; i++) {
    await page.getByTestId("add-item-song").selectOption({ label: ["Aaa", "Bbb", "Ccc"][i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Ccc");

  // Drag the 3rd row's grip onto the 1st row → Ccc to the top.
  await page.getByTestId("item-grip").nth(2).dragTo(page.getByTestId("item-row").nth(0));

  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Ccc");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Aaa");
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Bbb");

  // Persists across reload.
  await page.reload();
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Ccc");

  // Now a DOWNWARD drag (the previously-broken direction): drag the 1st row (Ccc)
  // onto the 3rd row (Bbb) → Ccc lands ABOVE Bbb, where the drop hint shows, not
  // one slot too low. Expect [Aaa, Ccc, Bbb].
  await page.getByTestId("item-grip").nth(0).dragTo(page.getByTestId("item-row").nth(2));
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Ccc");
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Bbb");
});

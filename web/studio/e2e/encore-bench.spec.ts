/**
 * T23: encore/bench songs. A setlist item can be moved to the "bench" (on call) —
 * baked into the concert and jumpable on stage, but outside the running order and
 * its numbering. This exercises the Studio UI (bench section, move in/out, main
 * numbering unaffected) and that a setlist with a bench song still bakes.
 *
 * The bundle actually carrying `onCall` (band + personal variant, main-then-bench
 * order) is covered by the Go tests; the manifest view doesn't surface the flag, so
 * here we assert the UI contract + that the bake succeeds with a bench song present.
 * Songs have no PDFs, so the bake needs no poppler/web-bake toolchain (same trick as
 * bake.spec.ts).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

async function addSong(page: Page, bandUrl: string, title: string) {
  await page.goto(bandUrl);
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: title })).toBeVisible();
}

test("bench a setlist item → outside the running order, still baked", async ({ page }) => {
  await register(page, `bench_${stamp()}`);
  await createBandAndOpen(page, `BenchBand ${stamp()}`);
  const bandUrl = page.url();

  for (const t of ["Aaa", "Bbb", "Ccc"]) await addSong(page, bandUrl, t);

  // A setlist with all three songs in the running order.
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Show");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Show" }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  const labels = ["Aaa", "Bbb", "Ccc"];
  for (let i = 0; i < labels.length; i++) {
    // Wait for each row to land before adding the next — add is async (POST +
    // reload) and reload resets the select, so back-to-back clicks race (CI-slow).
    await page.getByTestId("add-item-song").selectOption({ label: labels[i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Bbb");
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Ccc");
  await expect(page.getByTestId("bench-empty")).toBeVisible();

  // Bench the MIDDLE song. Main numbering must stay contiguous from 1 (Bbb gone),
  // and the bench section holds Bbb.
  await page.getByTestId("item-row").nth(1).getByTestId("item-tobench").click();
  await expect(page.getByTestId("item-row")).toHaveCount(2);
  await expect(page.getByTestId("item-title").nth(0)).toContainText("1. Aaa");
  await expect(page.getByTestId("item-title").nth(1)).toContainText("2. Ccc");
  await expect(page.getByTestId("bench-row")).toHaveCount(1);
  await expect(page.getByTestId("bench-row").getByTestId("item-title")).toContainText("Bbb");

  // The setlist (3 songs incl. the bench) still bakes — a bench song doesn't break
  // the pipeline. Bake is admin-only; the creator is admin.
  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(page.getByTestId("bake-history-row")).toContainText("3 song");

  // Move it back → running order restored to three, bench empty.
  await page.getByTestId("bench-row").getByTestId("item-tomain").click();
  await expect(page.getByTestId("item-row")).toHaveCount(3);
  await expect(page.getByTestId("item-title").nth(2)).toContainText("3. Ccc");
  await expect(page.getByTestId("bench-empty")).toBeVisible();
});

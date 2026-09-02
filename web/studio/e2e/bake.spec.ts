/**
 * B02: an admin bakes a setlist and a download link + history appear.
 *
 * Bakes a setlist holding ONE song with no PDF, so the bake needs no external
 * toolchain (poppler/web-bake are only invoked for songs with a PDF) — this
 * exercises the Studio Bake card + the core bake/list/download endpoints end to
 * end. The full song→raster→overlay loop is covered by the Go tests (B01 parity
 * + B02 orchestration). An empty setlist can't bake at all (T124), which this
 * spec also asserts before adding the song.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

test("admin bakes a setlist → download link + history appear", { tag: "@smoke" }, async ({ page }) => {
  await register(page, `bake_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `BakeBand ${stamp()}`);
  const songTitle = `Open Road ${stamp()}`;
  await createSongAndOpen(page, songTitle); // a song to put on the setlist (no PDF → no toolchain)

  // Create a setlist and open it. Reach it by URL: createSongAndOpen leaves us in the
  // full-screen song editor, which has no nav sidebar.
  await page.goto(`/bands/${bandId}/setlists`);
  await expect(page).toHaveURL(/\/setlists$/);
  await page.getByTestId("setlist-name").fill(`Gig ${stamp()}`);
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // The admin-only Bake card is present; no bake exists yet.
  await expect(page.getByTestId("bake-card")).toBeVisible();
  await expect(page.getByTestId("bake-download")).toHaveCount(0);

  // T124: an EMPTY setlist cannot be baked — the button is disabled and says why.
  const bakeBtn = page.getByTestId("bake-setlist");
  await expect(bakeBtn).toBeDisabled();
  await expect(bakeBtn).toHaveAttribute("title", /Add at least one song/);

  // Add the song → the setlist has content and the Bake button enables.
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toContainText(songTitle);
  await expect(bakeBtn).toBeEnabled();

  // Bake → a download link and a history row appear (rev 1).
  await bakeBtn.click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(page.getByTestId("bake-download")).toBeVisible();
  await expect(page.getByTestId("bake-download")).toHaveAttribute(
    "href",
    /\/concerts\/[^/]+\/bundle$/,
  );
  await expect(page.getByTestId("bake-history-row")).toHaveCount(1);
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 1");

  // Re-bake bumps the revision.
  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(page.getByTestId("bake-history-row").first()).toContainText("Rev 2");
  await expect(page.getByTestId("bake-download")).toContainText("rev 2");
});

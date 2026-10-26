/**
 * T57: an admin can download a printable PDF of a baked concert (VLL's paper
 * fallback). Bakes a setlist holding ONE song with no PDF, so the bake needs no
 * external toolchain — the full raster+overlay compositing is covered by the Go
 * tests; this asserts the Studio "Download PDF" link appears beside the bundle
 * download and serves an application/pdf body.
 */
import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
import { stamp, register, createBandAndOpen, createSongAndOpen, createSetlist } from "./setup-helpers";

test("admin downloads a concert PDF (paper fallback)", async ({ page }) => {
  await register(page, `pdf_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `PdfBand ${stamp()}`);
  const songTitle = `Open Road ${stamp()}`;
  await createSongAndOpen(page, songTitle); // a song to put on the setlist (no PDF → no toolchain)

  // Reach setlists by URL: createSongAndOpen leaves us in the full-screen song editor (no nav).
  await page.goto(`/bands/${bandId}/setlists`);
  await expect(page).toHaveURL(/\/setlists$/);
  await createSetlist(page, `Gig ${stamp()}`);
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // No PDF link before a bake exists.
  await expect(page.getByTestId("bake-pdf-download")).toHaveCount(0);

  // A setlist needs a song before it can bake (T124).
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toContainText(songTitle);

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog

  // T202: "Download PDF" now opens the PDF dialog (member scope + progress) rather than being a bare link.
  // "Make PDF" runs the real async job and the file downloads itself. (pdf-dialog.spec mocks these routes; this
  // exercises them for real, complementing it.)
  await page.getByTestId("bake-pdf-download").click();
  await expect(page.getByTestId("pdf-dialog")).toBeVisible();
  const dl = page.waitForEvent("download");
  await page.getByTestId("pdf-make").click();
  const download = await dl;
  await expect(page.getByTestId("pdf-done")).toBeVisible();

  // The downloaded file is a sane PDF.
  expect(download.suggestedFilename()).toMatch(/\.pdf$/);
  const body = readFileSync(await download.path());
  expect(body.length).toBeGreaterThan(200);
  expect(body.subarray(0, 5).toString()).toBe("%PDF-");
});

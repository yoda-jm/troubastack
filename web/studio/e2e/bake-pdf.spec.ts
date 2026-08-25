/**
 * T57: an admin can download a printable PDF of a baked concert (VLL's paper
 * fallback). Uses an EMPTY setlist so the bake needs no external toolchain — the
 * full raster+overlay compositing is covered by the Go tests; this asserts the
 * Studio "Download PDF" link appears beside the bundle download and serves an
 * application/pdf body.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

test("admin downloads a concert PDF (paper fallback)", async ({ page }) => {
  await register(page, `pdf_${stamp()}`);
  await createBandAndOpen(page, `PdfBand ${stamp()}`);

  await page.getByTestId("nav-setlists").click();
  await expect(page).toHaveURL(/\/setlists$/);
  await page.getByTestId("setlist-name").fill(`Gig ${stamp()}`);
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // No PDF link before a bake exists.
  await expect(page.getByTestId("bake-pdf-download")).toHaveCount(0);

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog

  const pdfLink = page.getByTestId("bake-pdf-download");
  await expect(pdfLink).toBeVisible();
  await expect(pdfLink).toHaveAttribute("href", /\/concerts\/[^/]+\/pdf$/);

  // Fetch it (shares the browser session): application/pdf, %PDF- header, sane size.
  const href = await pdfLink.getAttribute("href");
  const resp = await page.request.get(href!);
  expect(resp.status()).toBe(200);
  expect(resp.headers()["content-type"]).toContain("application/pdf");
  const body = await resp.body();
  expect(body.length).toBeGreaterThan(200);
  expect(body.subarray(0, 5).toString()).toBe("%PDF-");
});

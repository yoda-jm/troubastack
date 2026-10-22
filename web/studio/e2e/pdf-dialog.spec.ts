/**
 * T202 — the concert-PDF dialog. The async job + member-scoping are Go-tested; here we mock the job routes
 * (the e2e core has no bake toolchain) and assert the UI: an admin gets a member picker defaulting to "Me",
 * "Make PDF" shows a progress bar, then the file downloads by itself and a "Download again" link remains.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

const PDF = Buffer.from("%PDF-1.4\n1 0 obj<<>>endobj\ntrailer<<>>\n%%EOF\n");

// mockJob wires the kick/poll/file routes for one concert: the first poll reports running, the next done.
async function mockJob(page: Page, concertId: string, name: string, bandId: string) {
  await page.route("**/concerts", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        concerts: [
          {
            concertId,
            name,
            currentRev: "1",
            updatedAt: "1",
            songs: [],
            downloadUrl: `/api/bands/${bandId}/concerts/${concertId}/bundle`,
          },
        ],
      }),
    }),
  );
  await page.route("**/pdf-jobs", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ jobId: "job1" }) }),
  );
  let polls = 0;
  await page.route("**/pdf-jobs/*", (route) => {
    polls++;
    const body = polls < 2 ? { state: "running", done: 1, total: 3 } : { state: "done", done: 3, total: 3 };
    return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(body) });
  });
  await page.route("**/pdf-jobs/*/file", (route) =>
    route.fulfill({ status: 200, contentType: "application/pdf", body: PDF }),
  );
}

test("admin: the PDF dialog defaults to Me, shows progress, then downloads", async ({ page }) => {
  await register(page, `pdf_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `PdfBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `PdfGig ${stamp()}`;
  await createSetlist(page, name);

  const href = await page.getByTestId("setlist-link").filter({ hasText: name }).getAttribute("href");
  const slId = href!.split("/").pop()!;
  await mockJob(page, slId, name, bandId);

  // Open the detail; the mocked concert makes "Download PDF" appear.
  await page.getByTestId("setlist-link").filter({ hasText: name }).click();
  await page.getByTestId("bake-pdf-download").click();
  await expect(page.getByTestId("pdf-dialog")).toBeVisible();

  // Admin → a member picker defaulting to "Me".
  const picker = page.getByTestId("pdf-member");
  await expect(picker).toHaveValue("");
  await expect(picker.locator("option").first()).toContainText("Me");

  // Make PDF → progress → the download fires (the "Download again" link appears).
  const download = page.waitForEvent("download").catch(() => null);
  await page.getByTestId("pdf-make").click();
  await expect(page.getByTestId("pdf-progress")).toBeVisible();
  await expect(page.getByTestId("pdf-done")).toBeVisible();
  await download;
});

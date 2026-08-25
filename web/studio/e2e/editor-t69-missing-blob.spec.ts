/**
 * T69 (studio slice): a file whose rendered blob is missing (404) must show a clear,
 * actionable "data missing" state — not a raw status or a blank editor — and the file strip
 * plus every OTHER file must stay usable. (Generated charts self-heal server-side; a 404 that
 * reaches the viewer is an uploaded file whose bytes are genuinely lost.)
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function setupTwoFiles(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`${prefix}B ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`${prefix}S ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);

  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("new-text-chart").click();
  await page.getByTestId("chart-source").fill("# Beta Chart\n\n## Verse 1\nline\n");
  await page.getByTestId("chart-save").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(2);
  await page.getByTestId("my-files-edit").click();
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
}

test("a missing-data file shows a clear state; the strip + other files still work (T69)", async ({
  page,
}) => {
  await setupTwoFiles(page, "t69");
  const tabs = page.getByTestId("file-tab");

  // Select the 2nd file and grab its id (T68 puts it in ?file).
  await tabs.nth(1).click();
  await expect(page).toHaveURL(/\?file=[^&]+/);
  const id2 = new URL(page.url()).searchParams.get("file")!;

  // Simulate the orphaned blob: 404 every fetch of THAT file's bytes (any ?rev).
  await page.route(`**/api/files/${id2}*`, (route) =>
    route.fulfill({ status: 404, contentType: "text/plain", body: "pdf not found" }),
  );

  // Reload → T68 restores the 2nd file, whose fetch now 404s → the clear data-missing state.
  await page.reload();
  const err = page.getByTestId("error");
  await expect(err).toBeVisible();
  await expect(err).toContainText(/missing/i);

  // The editor is NOT blanked: the file strip still shows both files.
  await expect(page.getByTestId("file-tab")).toHaveCount(2);

  // …and the OTHER file (not intercepted) still opens and renders.
  await tabs.nth(0).click();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("error")).toHaveCount(0);
});

/**
 * T79 — good default names for new files. A from-scratch text chart defaults to the SONG's title
 * (both the `# Title` line and the pool row), not a permanent "New chart"; an uploaded file lands
 * under its name WITHOUT the extension (a part is a part — the extension is re-derived at download).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}

test("Files: from-scratch defaults to the song title; upload drops the extension (T79)", async ({
  page,
}) => {
  await register(page, `t79_${stamp()}`);
  await createBandAndOpen(page, `T79Band ${stamp()}`);
  await createSongAndOpen(page, "Hotel California");

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();

  // From scratch → the stub carries the song title, not "New chart".
  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-source")).toHaveValue(/# Hotel California/);
  await expect(panel.getByTestId("chart-source")).not.toHaveValue(/New chart/);
  await panel.getByTestId("chart-save").click();
  await expect(panel.getByTestId("file-download")).toHaveText(/^Hotel California$/); // no ".pdf"

  // Upload sample.pdf → lands as "sample" (extension stripped from the pool name).
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(2);
  await expect(panel.getByTestId("file-download").filter({ hasText: /^sample$/ })).toHaveCount(1);
});

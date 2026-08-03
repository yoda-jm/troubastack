/**
 * T68: the open file is mirrored into ?file=<id>, so a hard refresh restores it instead of
 * snapping back to the first file. Selection uses `replace` (Back still exits the editor), a
 * stale/foreign ?file degrades to the first PDF, and no ?file behaves exactly as before.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

// A song with TWO viewable files: a generated chart + an uploaded PDF (distinct names).
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
  // File 1: a text chart.
  await page.getByTestId("new-text-chart").click();
  await page.getByTestId("chart-source").fill("# Beta Chart\n\n## Verse 1\nline\n");
  await page.getByTestId("chart-save").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  // File 2: an uploaded PDF.
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(2);
  await page.getByTestId("my-files-edit").click(); // close Details
  // The viewer's file strip syncs its "my files" on reload (an upload doesn't push into the
  // viewer in-session — pre-existing, same as the T65/T66 setups); reload to a 2-file strip.
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
}

test("a hard refresh restores the open file via ?file (not the first) — T68", async ({ page }) => {
  await setupTwoFiles(page, "t68");

  const tabs = page.getByTestId("file-tab");
  // Select the SECOND file and remember which one it is.
  await tabs.nth(1).click();
  await expect(tabs.nth(1)).toHaveAttribute("aria-selected", "true");
  const name2 = (await tabs.nth(1).textContent())!.trim();
  await expect(page).toHaveURL(/\?file=[^&]+/);
  const url2 = page.url();

  // Hard reload → the SECOND file is restored (not the first), URL preserved.
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  const activeAfter = page.locator('[data-testid="file-tab"][aria-selected="true"]');
  await expect(activeAfter).toHaveCount(1);
  await expect(activeAfter).toHaveText(name2);
  expect(page.url()).toBe(url2);
  // It's genuinely the SECOND tab that's active, not the first.
  await expect(tabs.nth(1)).toHaveAttribute("aria-selected", "true");
  await expect(tabs.nth(0)).toHaveAttribute("aria-selected", "false");
});

test("switching files uses replace — Back exits the editor, doesn't walk file history (T68)", async ({
  page,
}) => {
  await setupTwoFiles(page, "t68b");
  const tabs = page.getByTestId("file-tab");
  await tabs.nth(0).click();
  const histBefore = await page.evaluate(() => history.length);
  await tabs.nth(1).click();
  await expect(tabs.nth(1)).toHaveAttribute("aria-selected", "true");
  await tabs.nth(0).click();
  await expect(tabs.nth(0)).toHaveAttribute("aria-selected", "true");
  const histAfter = await page.evaluate(() => history.length);
  expect(histAfter, "file switches must not grow history (replace, not push)").toBe(histBefore);
});

test("a stale/foreign ?file degrades to the first PDF; no ?file is unchanged (T68)", async ({
  page,
}) => {
  await setupTwoFiles(page, "t68c");
  // Grab the current song URL, then reopen it with a bogus ?file.
  const songUrl = page.url().split("?")[0];
  await page.goto(`${songUrl}?file=does-not-exist-123`);
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  // Falls back to the FIRST viewable file — never a blank/wedged viewer.
  await expect(page.locator('[data-testid="file-tab"]').nth(0)).toHaveAttribute(
    "aria-selected",
    "true",
  );
  // …and the URL self-heals to the resolved file.
  await expect(page).toHaveURL(/\?file=(?!does-not-exist-123)[^&]+/);
});

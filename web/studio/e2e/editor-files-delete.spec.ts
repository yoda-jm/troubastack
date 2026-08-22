/**
 * T36 guard: the song's file management (upload / ＋ new text chart / delete file) and
 * "Delete song" must be reachable from the fullscreen editor's "Details" pill. The whole
 * surface lives in the Details panel now — SongEditor's clipped <Details>, the substrate
 * of the Playwright-reachable/human-unreachable bug class, was removed.
 *
 * Field report (VLL 2026-07-12): "we cannot add a pdf or a typing text file, we cannot
 * delete a song." Every assertion is SCOPED to the details-panel so no stray copy can
 * false-pass; red-first on pre-T36 main (the panel then holds only metadata + my-files).
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

test("Details panel: add file, new text chart, delete file, delete song all reachable (T36)", async ({
  page,
}) => {
  await register(page, `fd_${stamp()}`);
  await createBandAndOpen(page, `FDBand ${stamp()}`);
  const bandUrl = page.url();
  await createSongAndOpen(page, `FDSong ${stamp()}`);

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click(); // open the Details pill
  await expect(panel).toBeVisible();

  // (1) FILE ADD — the upload form is IN the panel (scoped: pre-T36 it lived in the
  // clipped SongEditor copy, so this is not attached and the test is red).
  await expect(panel.getByTestId("file-input")).toBeAttached();
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
  // Reload so the viewer picks up the new pool file: it reaches the parts bar (renders).
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await page.getByTestId("my-files-edit").click(); // re-open the panel for the rest
  await expect(panel).toBeVisible();

  // (2) NEW TEXT CHART — the ＋ New text chart action opens the T19 chart editor.
  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  await expect(panel.getByTestId("chart-source")).toBeVisible();
  await panel.getByTestId("chart-editor").getByRole("button", { name: "Cancel" }).click();
  await expect(panel.getByTestId("chart-editor")).toHaveCount(0);

  // (3) FILE DELETE — remove the uploaded file via the row … menu (T91 in-app confirm).
  await panel.getByTestId("file-menu").first().click();
  await page.getByTestId("file-menu-delete").first().click();
  await page.getByTestId("app-dialog-confirm").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(0);

  // (4) DELETE SONG — under the Admin tab (T54). Reachability probe: after scrolling the
  // PANEL (not the page), elementFromPoint at the button center resolves to the button
  // itself — nothing clips or occludes it (the bug class this task kills).
  await panel.getByTestId("details-tab-admin").click(); // T54: delete lives under Admin
  const del = panel.getByTestId("delete-song");
  await del.scrollIntoViewIfNeeded();
  const hits = await del.evaluate((el) => {
    const r = el.getBoundingClientRect();
    const at = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return !!at && (at === el || el.contains(at));
  });
  expect(hits, "delete-song must be hittable at the panel tail").toBe(true);
  await del.click();
  await page.getByTestId("app-dialog-confirm").click(); // T91 in-app confirm
  await expect(page).toHaveURL(bandUrl); // song gone → back on the band page
});

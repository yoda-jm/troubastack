/**
 * Regression guard: song metadata (key / artist / tempo / tags / notes) must be
 * reachable + editable from the fullscreen editor's "Details" pill.
 *
 * Field report (VLL 2026-07-11): "where can I edit song info like default key, author,
 * tempo — nothing in Details." Root cause: the T27 full-bleed reshape made the page
 * `height:100dvh; overflow:hidden`, clipping SongEditor's Details section (with the
 * Metadata form) off-screen, and the in-editor Details pill was wired only to the file
 * manager. Fix: render the existing <Metadata> form in the editor's Details panel.
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
async function uploadPdf(page: Page) {
  // T36: file management lives in the editor's Details panel now — open it to reach the
  // upload form, then close it so the canvas is unobstructed for whatever follows.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}
async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

test("editor Details pill: song metadata (key/tempo/artist) is reachable + editable", async ({
  page,
}) => {
  await register(page, `sd_${stamp()}`);
  await createBandAndOpen(page, `SDBand ${stamp()}`);
  await createSongAndOpen(page, `SDSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // The "Details" pill must open a panel that actually shows the song-metadata form.
  // Scope to the panel: pre-fix `meta-*` still existed in SongEditor's clipped-off-screen
  // section (DOM-present, human-unreachable), so an unscoped check would false-pass — the
  // point is the form is reachable IN THE PANEL.
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await expect(panel).toBeVisible();
  await expect(panel.getByTestId("song-meta-form")).toBeVisible();
  for (const id of ["meta-key", "meta-tempo", "meta-artist"]) {
    await expect(panel.getByTestId(id)).toBeVisible();
  }

  // Edit + save.
  await panel.getByTestId("meta-artist").fill("The Testers");
  await panel.getByTestId("meta-key").fill("Em");
  await panel.getByTestId("meta-tempo").fill("128");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();

  // Persisted to the song: reload, reopen Details → the values are back.
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("my-files-edit").click();
  const panel2 = page.getByTestId("details-panel");
  await expect(panel2.getByTestId("meta-key")).toHaveValue("Em");
  await expect(panel2.getByTestId("meta-tempo")).toHaveValue("128");
  await expect(panel2.getByTestId("meta-artist")).toHaveValue("The Testers");
});

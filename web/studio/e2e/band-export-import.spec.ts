/**
 * T62: whole-band export/import. An admin exports a band as a portable .tband archive
 * from Band settings; a different user imports it on the Bands page, landing on a fresh
 * band with a reconciliation report (which member accounts were matched vs created).
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

async function logout(page: Page) {
  await page.getByTestId("account-trigger").click();
  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login$/);
}

test("export a band, then import it as another user (T62)", async ({ page }) => {
  const admin = `adm_${stamp()}`;
  await register(page, admin);

  // Build a band with one song.
  await page.getByTestId("new-band-btn").click();
  const bandName = `Exportable ${stamp()}`;
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);

  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("Wonderwall");
  await page.getByTestId("create-song").click();
  await expect(page.getByTestId("song-link").filter({ hasText: "Wonderwall" })).toBeVisible();

  // Export from Band settings → download the .tband archive.
  await page.getByTestId("nav-settings").click();
  const downloadPromise = page.waitForEvent("download");
  await page.getByTestId("export-band").click();
  const download = await downloadPromise;
  expect(download.suggestedFilename()).toMatch(/\.zip$/);
  const archivePath = await download.path();
  expect(archivePath).toBeTruthy();

  // A different user imports it → the preview dialog (T63) opens. The source admin already
  // has an account here, so under the consent model she's listed as an existing member with
  // the default "invite" — confirm straight through.
  await logout(page);
  const importer = `imp_${stamp()}`;
  await register(page, importer);
  await page.getByTestId("import-band-input").setInputFiles(archivePath!);
  await expect(page.getByTestId("import-dialog")).toBeVisible();
  await expect(page.getByTestId("import-dialog")).toContainText(admin); // existing member listed
  await expect(page.getByTestId(`disposition-${admin}`)).toHaveValue("invite");
  await page.getByTestId("import-confirm").click();

  await expect(page).toHaveURL(/\/bands\/[^/]+$/);
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const report = page.getByTestId("import-report");
  await expect(report).toBeVisible();
  await expect(report).toContainText("1 song");
  // The source admin already had an account → consent-required, so she's INVITED not attached.
  await expect(page.getByTestId("import-invited")).toContainText(admin);

  // The imported band shows the song.
  await expect(page.getByTestId("song-link").filter({ hasText: "Wonderwall" })).toBeVisible();
});

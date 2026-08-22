/**
 * T91 — the studio's destructive actions used blocking `window.confirm`/`window.prompt`. A browser
 * that has shown a few dialogs offers "prevent this page from creating additional dialogs"; once
 * ticked, confirm returns false and prompt returns null, and every delete/rename **silently no-ops**
 * — a T30 "no silent ink" dead end VLL hits on a phone. The fix routes them through one in-app
 * dialog (components/Dialog.tsx). These tests **suppress the native dialogs at the browser level**
 * (confirm→false, prompt→null, alert→noop) and prove the flows still work — which is only possible
 * because nothing calls the native ones any more. Revert any one site to `window.confirm` and the
 * matching suppression test goes red (the teeth-check the spec asks for).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

// Simulate a browser that has suppressed further native dialogs — for the WHOLE session, every frame.
// If any production path still called window.confirm/prompt, it would read false/null here and no-op.
async function suppressNativeDialogs(page: Page) {
  await page.addInitScript(() => {
    window.confirm = () => false;
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    (window as any).prompt = () => null;
    window.alert = () => {};
  });
}

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`Display ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBand(page: Page, name: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(name);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("band-title")).toHaveText(name);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function openDetailsWithOneFile(page: Page) {
  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
  return panel;
}

test("delete-file works with native dialogs suppressed — in-app confirm, not window.confirm (T91)", async ({
  page,
}) => {
  await suppressNativeDialogs(page);
  await register(page, `t91d_${stamp()}`);
  await createBand(page, `T91Band ${stamp()}`);
  await createSongAndOpen(page, `T91Song ${stamp()}`);
  const panel = await openDetailsWithOneFile(page);

  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-delete").click();

  // The in-app dialog appears even though window.confirm is dead.
  const dialog = page.getByTestId("app-dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("app-dialog-confirm").click();

  // The file is actually gone — a stubbed window.confirm would have no-op'd it.
  await expect(panel.getByTestId("file-row")).toHaveCount(0);
  // Details stayed open through it (the dialog is [data-portal]; T89's outside-click exempts it).
  await expect(panel).toBeVisible();
});

test("rename works with native prompt suppressed — in-app prompt, not window.prompt (T91)", async ({
  page,
}) => {
  await suppressNativeDialogs(page);
  await register(page, `t91r_${stamp()}`);
  await createBand(page, `T91RBand ${stamp()}`);
  await createSongAndOpen(page, `T91RSong ${stamp()}`);
  const panel = await openDetailsWithOneFile(page);

  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-rename").click();

  const dialog = page.getByTestId("app-dialog");
  await expect(dialog).toBeVisible();
  const input = dialog.getByTestId("app-dialog-input");
  await expect(input).toBeVisible();
  await input.fill("renamed-by-dialog.pdf");
  await dialog.getByTestId("app-dialog-confirm").click();

  // The rename landed — a stubbed window.prompt would have returned null and no-op'd.
  await expect(panel.getByTestId("file-row")).toContainText("renamed-by-dialog.pdf");
});

test("Escape and outside-click cancel the dialog and do NOT perform the destructive action (T91)", async ({
  page,
}) => {
  await suppressNativeDialogs(page);
  await register(page, `t91c_${stamp()}`);
  await createBand(page, `T91CBand ${stamp()}`);
  await createSongAndOpen(page, `T91CSong ${stamp()}`);
  const panel = await openDetailsWithOneFile(page);
  const dialog = page.getByTestId("app-dialog");

  // Escape cancels — dialog closes itself (Fable's [data-portal] obligation) and the file survives.
  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-delete").click();
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0);
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
  await expect(panel).toBeVisible(); // Details did not collapse with the dialog's Escape

  // Outside-click (on the backdrop, away from the card) cancels — file still survives.
  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-delete").click();
  await expect(dialog).toBeVisible();
  await page.mouse.click(4, 4); // top-left backdrop, well outside the centred card
  await expect(dialog).toHaveCount(0);
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
});

test("delete-setlist works with native dialogs suppressed — a Part B (confirm-only) site (T91)", async ({
  page,
}) => {
  // The SongDetails file/song paths are covered above; this proves the same in-app confirm wiring at
  // a different call site (SetlistDetail) — the whole point of one shared dialog.
  await suppressNativeDialogs(page);
  await register(page, `t91s_${stamp()}`);
  await createBand(page, `T91SBand ${stamp()}`);
  await page.getByTestId("nav-setlists").click();
  await page.getByTestId("setlist-name").fill("Teardown Show");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Teardown Show" }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  const del = page.getByTestId("delete-setlist");
  await del.scrollIntoViewIfNeeded();
  await del.click();
  const dialog = page.getByTestId("app-dialog");
  await expect(dialog).toBeVisible();
  await dialog.getByTestId("app-dialog-confirm").click();
  // Navigated back to the setlists list → the setlist was actually deleted (a dead window.confirm
  // would have left us on the detail page).
  await expect(page).toHaveURL(/\/setlists$/);
});

test("the destructive button is not the default-focused control when the dialog opens (T91)", async ({
  page,
}) => {
  await suppressNativeDialogs(page);
  await register(page, `t91f_${stamp()}`);
  await createBand(page, `T91FBand ${stamp()}`);
  await createSongAndOpen(page, `T91FSong ${stamp()}`);
  const panel = await openDetailsWithOneFile(page);

  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-delete").click();
  const confirmBtn = page.getByTestId("app-dialog-confirm");
  await expect(confirmBtn).toBeVisible();
  // The danger button must never be the initial focus (an accidental Enter must not delete).
  await expect(confirmBtn).not.toBeFocused();
});

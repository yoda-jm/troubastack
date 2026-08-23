/**
 * T94 — Layers/Notes/Details: two honest classes, one dismissal contract.
 *  - The rail (file inspector) is opened by ONE pill (`sidebar-toggle`); Layers ↔ Notes switch on the
 *    tabs INSIDE it (`drawer-layers` / `drawer-notes`), and it remembers the last tab for the session.
 *  - Details (`my-files-edit`) is the song's properties and is mutually exclusive with the rail.
 *  - Both close by ✕ / Escape / outside-click, and both cede Escape + outside clicks to an open
 *    [data-portal] overlay — a delete-layer confirm (T83) opened from the rail closes on Escape and
 *    leaves the rail open, not the rail underneath it.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

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
async function createSong(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function editorWithFile(page: Page, tag: string) {
  await register(page, `t94${tag}_${stamp()}`);
  await createBand(page, `T94${tag} ${stamp()}`);
  await createSong(page, `T94${tag}Song ${stamp()}`);
  // Upload a PDF via Details, then reload into the live editor.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

test("one pill opens the rail; Layers/Notes are tabs inside; it remembers the last tab (T94)", async ({
  page,
}) => {
  await editorWithFile(page, "tabs");
  const rail = page.getByTestId("viewer-drawer");
  const pill = page.getByTestId("sidebar-toggle");

  await pill.click();
  await expect(rail).toBeVisible();
  await expect(page.getByTestId("layers-panel")).toBeVisible(); // defaults to Layers

  await page.getByTestId("drawer-notes").click(); // switch tab inside the rail (does not close it)
  await expect(rail).toBeVisible();
  await expect(page.getByTestId("annotation-list")).toBeVisible();
  await expect(page.getByTestId("layers-panel")).toHaveCount(0);

  await pill.click(); // the one pill closes the rail
  await expect(rail).toHaveCount(0);

  await pill.click(); // reopen → remembers Notes
  await expect(page.getByTestId("annotation-list")).toBeVisible();
});

test("rail and Details are mutually exclusive — opening one closes the other (T94)", async ({
  page,
}) => {
  await editorWithFile(page, "mux");
  const rail = page.getByTestId("viewer-drawer");
  const details = page.getByTestId("details-panel");

  await page.getByTestId("sidebar-toggle").click();
  await expect(rail).toBeVisible();
  await page.getByTestId("my-files-edit").click(); // open Details → rail closes
  await expect(details).toBeVisible();
  await expect(rail).toHaveCount(0);

  await page.getByTestId("sidebar-toggle").click(); // open rail → Details closes
  await expect(rail).toBeVisible();
  await expect(details).toHaveCount(0);
});

test("the rail closes by its ✕ and Escape; an outside click LEAVES it open (inspect-while-edit) (T94)", async ({
  page,
}) => {
  await editorWithFile(page, "dismiss");
  const rail = page.getByTestId("viewer-drawer");
  const open = () => page.getByTestId("sidebar-toggle").click();

  await open();
  await expect(rail).toBeVisible();
  await page.getByTestId("drawer-close").click(); // ✕
  await expect(rail).toHaveCount(0);

  await open();
  await expect(rail).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(rail).toHaveCount(0);

  // Deliberate T94 deviation from §3.3 (flagged at the gate): the rail is a WORKING inspector — you
  // select canvas objects into its list and delete via its own control WHILE it is open. So an outside
  // click on the score/toolbar must NOT dismiss it (that broke 11 inspect-while-edit specs). Its exits
  // are ✕ + Escape. (Details keeps outside-click — you don't edit the canvas with it open — see
  // details-close.spec.ts.)
  await open();
  await expect(rail).toBeVisible();
  await page.mouse.click(4, 300);
  await page.waitForTimeout(150);
  await expect(rail).toBeVisible(); // survived the outside click
});

test("a delete-layer confirm from the rail: Escape closes the dialog and leaves the rail open (T94)", async ({
  page,
}) => {
  await editorWithFile(page, "dlg");
  await page.getByTestId("sidebar-toggle").click();
  await expect(page.getByTestId("layers-panel")).toBeVisible();

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("layer-delete").first().click();

  const dialog = page.getByTestId("delete-layer-dialog");
  await expect(dialog).toBeVisible();
  await page.keyboard.press("Escape");
  // The [data-portal] dialog owns Escape: it closes itself, the rail stays open (does not collapse).
  await expect(dialog).toHaveCount(0);
  await expect(page.getByTestId("viewer-drawer")).toBeVisible();
});

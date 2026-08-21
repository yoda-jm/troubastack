/**
 * T78 — the Files section is a sortable list with a per-row "…" menu. Covers: the menu offers
 * rename / delete / move / view-source; VIEW SOURCE is present only on a text chart, absent on an
 * uploaded PDF (a menu that offered it on a PDF would be a bug); and a menu reorder persists across
 * a reload (the shared pool `displayOrder`).
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

test("Files: … menu — view-source only on charts, menu reorder persists, rename (T78)", async ({
  page,
}) => {
  // rename uses window.prompt, delete uses window.confirm — one handler covers both.
  page.on("dialog", (d) => void d.accept(d.type() === "prompt" ? "Renamed Part" : undefined));
  await register(page, `t78_${stamp()}`);
  await createBandAndOpen(page, `T78Band ${stamp()}`);
  await createSongAndOpen(page, `T78Song ${stamp()}`);

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();

  // Upload a PDF (row 0), then create a text chart (row 1) with a known title.
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# ZZZ Chart\n\n## Verse\nla la la\n");
  await panel.getByTestId("chart-save").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(2);

  const pdfRow = panel.getByTestId("file-row").filter({ hasNot: page.getByTestId("file-chart-badge") });
  const chartRow = panel.getByTestId("file-row").filter({ has: page.getByTestId("file-chart-badge") });

  // View source is present on the text chart's menu, ABSENT on the PDF's menu.
  await chartRow.getByTestId("file-menu").click();
  await expect(page.getByTestId("file-menu-source")).toBeVisible();
  await expect(page.getByTestId("file-menu-rename")).toBeVisible();
  await page.keyboard.press("Escape");

  await pdfRow.getByTestId("file-menu").click();
  await expect(page.getByTestId("file-menu-source")).toHaveCount(0);
  await expect(page.getByTestId("file-menu-rename")).toBeVisible();
  await page.keyboard.press("Escape");

  // Menu reorder: move the PDF (row 0) DOWN — the chart takes row 0. Persists across reload.
  await expect(panel.getByTestId("file-row").first().getByTestId("file-chart-badge")).toHaveCount(0);
  await pdfRow.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-down").click();
  await expect(panel.getByTestId("file-row").first().getByTestId("file-chart-badge")).toHaveCount(1);
  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await expect(panel.getByTestId("file-row").first().getByTestId("file-chart-badge")).toHaveCount(1);

  // Rename the (now first) chart via the menu → the download link shows the new name.
  await panel.getByTestId("file-row").first().getByTestId("file-menu").click();
  await page.getByTestId("file-menu-rename").click();
  await expect(panel.getByTestId("file-download").first()).toHaveText(/Renamed Part/);
});

// ===========================================================================
// T87 — the … menu is portalled, so overflow:hidden on the Details card no longer
// clips it away on the lower rows (it was a dead control there).
// ===========================================================================
async function fourFileRows(page: Page) {
  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
  for (const t of ["AAA", "BBB", "CCC"]) {
    await panel.getByTestId("new-text-chart").click();
    await panel.getByTestId("chart-source").fill(`# ${t} Chart\n\n## Verse\nla\n`);
    await panel.getByTestId("chart-save").click();
  }
  await expect(panel.getByTestId("file-row")).toHaveCount(4);
  return panel;
}

test("Files: the last row's … menu is in-viewport and actionable, not clipped (T87)", async ({
  page,
}) => {
  page.on("dialog", (d) => void d.accept(d.type() === "prompt" ? "Renamed Last" : undefined));
  await register(page, `t87_${stamp()}`);
  await createBandAndOpen(page, `T87Band ${stamp()}`);
  await createSongAndOpen(page, `T87Song ${stamp()}`);
  const panel = await fourFileRows(page);

  // Open the LAST row's menu — the one whose downward panel used to fall past the section's
  // overflow:hidden edge and vanish.
  const lastRow = panel.getByTestId("file-row").last();
  await lastRow.getByTestId("file-menu").click();
  await expect(page.getByTestId("file-menu-rename")).toBeVisible();

  // The portalled panel lies fully within the viewport.
  const box = (await page.locator(".row-menu-panel").boundingBox())!;
  const vw = page.viewportSize()!;
  expect(box.x).toBeGreaterThanOrEqual(0);
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.x + box.width).toBeLessThanOrEqual(vw.width + 1);
  expect(box.y + box.height).toBeLessThanOrEqual(vw.height + 1);

  // The real regression + trap 1: clicking an item actually performs its action (a clipped
  // panel's item is unpainted, so the click would not land and the rename would no-op).
  await page.getByTestId("file-menu-rename").click();
  await expect(lastRow).toContainText("Renamed Last");
});

test("Files: … menu closes on Escape (trap 2) and on an outside click (T87)", async ({ page }) => {
  await register(page, `t87esc_${stamp()}`);
  await createBandAndOpen(page, `T87EscBand ${stamp()}`);
  await createSongAndOpen(page, `T87EscSong ${stamp()}`);
  const panel = await fourFileRows(page);
  const lastRow = panel.getByTestId("file-row").last();
  const item = page.getByTestId("file-menu-rename");

  // Escape closes (the portalled panel's keydown doesn't bubble to the component — trap 2).
  await lastRow.getByTestId("file-menu").click();
  await expect(item).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(item).toHaveCount(0);

  // A genuine outside click still closes it.
  await lastRow.getByTestId("file-menu").click();
  await expect(item).toBeVisible();
  await panel.getByTestId("file-row").first().click();
  await expect(item).toHaveCount(0);
});

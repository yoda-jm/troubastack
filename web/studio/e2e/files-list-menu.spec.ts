/**
 * T78 — the Files section is a sortable list with a per-row "…" menu. Covers: the menu offers
 * rename / delete / move / view-source; VIEW SOURCE is present only on a text chart, absent on an
 * uploaded PDF (a menu that offered it on a PDF would be a bug); and a menu reorder persists across
 * a reload (the shared pool `displayOrder`).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

test("Files: … menu — view-source only on charts, menu reorder persists, rename (T78)", async ({
  page,
}) => {
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

  // T104: the Edit-chart affordance is a one-click ROW control now (not a menu item) — present on
  // the text chart row, ABSENT on the PDF row (there is nothing to edit on a PDF). This is the same
  // type-awareness the old "View source" menu item carried, moved onto the row.
  await expect(chartRow.getByTestId("file-chart-edit")).toBeVisible();
  await expect(pdfRow.getByTestId("file-chart-edit")).toHaveCount(0);

  // Rename remains in each row's ⋯ menu, on both file types.
  await chartRow.getByTestId("file-menu").click();
  await expect(page.getByTestId("file-menu-rename")).toBeVisible();
  await page.keyboard.press("Escape");

  await pdfRow.getByTestId("file-menu").click();
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
  await page.getByTestId("app-dialog-input").fill("Renamed Part"); // T91 in-app prompt
  await page.getByTestId("app-dialog-confirm").click();
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
  // T93 — this loop was the shared fixture's ~6% flake. Saving a chart is a POST + a list reload
  // (ChartEditor.onDone → setChart(null) closes the editor, then load() grows the list); the old
  // loop only asserted the final count of 4, so it raced ahead and the next `new-text-chart` /
  // `chart-save` could land on a half-torn-down editor (chart-save unmounted → timeout). Make each
  // iteration wait for its own observable post-conditions before opening the next editor:
  //   1. the editor is actually mounted before we type (open is an async state round-trip);
  //   2. after save, the row count has GROWN (load() landed) AND the editor has closed.
  let expected = 1; // the uploaded PDF
  for (const t of ["AAA", "BBB", "CCC"]) {
    await panel.getByTestId("new-text-chart").click();
    await expect(panel.getByTestId("chart-source")).toBeVisible();
    await panel.getByTestId("chart-source").fill(`# ${t} Chart\n\n## Verse\nla\n`);
    await panel.getByTestId("chart-save").click();
    expected += 1;
    await expect(panel.getByTestId("file-row")).toHaveCount(expected); // this save's reload landed
    await expect(panel.getByTestId("chart-editor")).toHaveCount(0); // editor closed before the next
  }
  await expect(panel.getByTestId("file-row")).toHaveCount(4);
  return panel;
}

test("Files: the last row's … menu is in-viewport and actionable, not clipped (T87)", async ({
  page,
}) => {
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
  await page.getByTestId("app-dialog-input").fill("Renamed Last"); // T91 in-app prompt
  await page.getByTestId("app-dialog-confirm").click();
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

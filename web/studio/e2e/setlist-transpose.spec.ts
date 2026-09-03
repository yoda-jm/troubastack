/**
 * T60 surface 2 (setlist transpose checkbox): the item edit form's "transpose chords"
 * checkbox is greyed unless the song has a text chart AND both the song key and the
 * typed key override parse. On a chart song it enables the moment a valid override is
 * typed; on a PDF-only song it stays greyed with the "no text chart" tooltip.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function newSong(page: Page, bandUrl: string, title: string) {
  await page.goto(bandUrl);
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}

test("setlist item transpose checkbox greys on a PDF-only song, enables on a chart song (T60)", async ({
  page,
}) => {
  await register(page, `slt_${stamp()}`);
  await createBandAndOpen(page, `SltBand ${stamp()}`);
  const bandUrl = page.url();

  // Chart song: key G + a generated text chart.
  await newSong(page, bandUrl, "Chart Song");
  await page.getByTestId("my-files-edit").click();
  let panel = page.getByTestId("details-panel");
  await panel.getByTestId("meta-key").fill("G");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();
  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# Chart Song\n## Verse\nG            D\nlyric here\n");
  await panel.getByTestId("chart-save").click();

  // PDF-only song: upload a PDF, no chart.
  await newSong(page, bandUrl, "PDF Song");
  await page.getByTestId("my-files-edit").click();
  panel = page.getByTestId("details-panel");
  await panel.getByTestId("file-input").setInputFiles(PDF_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  // Setlist with both songs.
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await createSetlist(page, "Show");
  await page.getByTestId("setlist-link").filter({ hasText: "Show" }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  // Wait for each row to land before adding the next — add is async (POST + reload)
  // and reload resets the select, so back-to-back clicks race on CI-slow.
  const labels = ["Chart Song", "PDF Song"];
  for (let i = 0; i < labels.length; i++) {
    await page.getByTestId("add-item-song").selectOption({ label: labels[i] });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("item-row")).toHaveCount(i + 1);
  }

  const chartRow = page.getByTestId("item-row").filter({ hasText: "Chart Song" });
  const pdfRow = page.getByTestId("item-row").filter({ hasText: "PDF Song" });

  // Chart song: checkbox starts disabled (no override typed), enables on a valid key.
  await chartRow.getByTestId("item-edit").click();
  await expect(chartRow.getByTestId("item-transpose")).toBeDisabled(); // no override yet
  await chartRow.getByTestId("item-key").fill("A");
  await expect(chartRow.getByTestId("item-transpose")).toBeEnabled();
  await chartRow.getByTestId("item-transpose").check();
  await chartRow.getByTestId("item-save").click();

  // PDF-only song: greyed even with a valid override, tooltip explains why.
  await pdfRow.getByTestId("item-edit").click();
  await pdfRow.getByTestId("item-key").fill("A");
  await expect(pdfRow.getByTestId("item-transpose")).toBeDisabled();
  await expect(pdfRow.getByTestId("item-transpose-label")).toHaveAttribute(
    "title",
    "no text chart on this song",
  );
  // And a chart-preview affordance is absent on the PDF-only song (no chart to preview).
  await expect(pdfRow.getByTestId("item-chart-preview")).toHaveCount(0);
});

test("bake surfaces per-song transpose warnings in the bake card (T60)", async ({ page }) => {
  await register(page, `slw_${stamp()}`);
  await createBandAndOpen(page, `SlwBand ${stamp()}`);
  const bandUrl = page.url();

  await newSong(page, bandUrl, "Some Song");
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await createSetlist(page, "Show");
  await page.getByTestId("setlist-link").filter({ hasText: "Show" }).click();
  await page.getByTestId("add-item-song").selectOption({ label: "Some Song" });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(1);

  // Mock the bake POST with a per-song warning (the e2e stack mocks bakes — no poppler);
  // this asserts the CLIENT surfaces the server's `warnings` in the bake card. The
  // server-side derivation is covered in httpapi TestBakeTransposeWarnings.
  const warning = "Some Song: chords not transposed — song key not set or not parseable";
  // T103: the POST just kicks (202); the warnings ride the TERMINAL progress record now, and the studio
  // surfaces them via onDone when the poll reaches succeeded.
  await page.route("**/setlists/*/bake", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: "x" }) }),
  );
  await page.route("**/bakes/*/progress", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ state: "succeeded", done: 1, total: 1, warnings: [warning] }),
    }),
  );

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click();
  const warns = page.getByTestId("bake-warnings");
  await expect(warns).toBeVisible();
  await expect(warns).toContainText(warning);
});

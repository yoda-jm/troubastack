/**
 * T79 — good default names for new files. A from-scratch text chart defaults to the SONG's title
 * (both the `# Title` line and the pool row), not a permanent "New chart"; an uploaded file lands
 * under its name WITHOUT the extension (a part is a part — the extension is re-derived at download).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

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

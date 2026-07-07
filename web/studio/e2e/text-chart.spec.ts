/**
 * T19: write a text chart in Studio → the server renders it to a PDF that enters
 * the song's file pool like any upload. Create → the pool shows a generated file
 * with the "text chart" badge and an Edit-source affordance → the rendered file is
 * a real, downloadable PDF → editing the source re-renders in place (same one
 * file, not a second).
 *
 * The chart's PDF *bytes* (dialect rendering, em-dash safety, chords) are covered
 * by the Go tests (internal/chartpdf golden pdftotext + httpapi lifecycle); and a
 * generated file is byte-for-byte a normal pool PDF, so the bake/Stage path is the
 * one already exercised by the bake Go/e2e tests — no new toolchain here.
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

async function createBandAndOpen(page: Page, name: string) {
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

test("write a text chart → it enters the pool as a generated PDF, editable in place", async ({
  page,
}) => {
  await register(page, `chart_${stamp()}`);
  await createBandAndOpen(page, `ChartBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);

  // Start a new text chart, type the tiny dialect.
  await page.getByTestId("new-text-chart").click();
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await page
    .getByTestId("chart-source")
    .fill("# Road Song\n\n## Verse 1\nG            D\nPack a little light for the road ahead,\n");

  // Preview (T25): renders the PDF into the pane via a blob URL, WITHOUT creating a
  // pool file (rendering fidelity is covered by the Go golden tests).
  await page.getByTestId("chart-preview-btn").click();
  const preview = page.getByTestId("chart-preview");
  await expect(preview).toBeVisible();
  await expect(preview).toHaveAttribute("data", /^blob:/);
  await expect(page.getByTestId("file-row")).toHaveCount(0); // nothing saved yet

  await page.getByTestId("chart-save").click();

  // It appears in the pool as exactly one generated file, badged, download-named
  // from the title.
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await expect(page.getByTestId("file-chart-badge")).toBeVisible();
  const dl = page.getByTestId("file-download");
  await expect(dl).toHaveText("Road Song.pdf");

  // The generated file is a real, servable PDF (a text chart is just a pool file).
  const href = await dl.getAttribute("href");
  expect(href).toBeTruthy();
  const res = await page.request.get(href!);
  expect(res.status()).toBe(200);
  expect(res.headers()["content-type"]).toContain("pdf");

  // Edit the source and re-save: re-renders in place — still exactly one file, and
  // the download name follows the new title.
  await page.getByTestId("file-edit-source").click();
  await expect(page.getByTestId("chart-source")).toHaveValue(/Road Song/);
  await page.getByTestId("chart-source").fill("# Road Song v2\n\n## Chorus\nsing it loud\n");
  await page.getByTestId("chart-save").click();

  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await expect(page.getByTestId("file-download")).toHaveText("Road Song v2.pdf");
});

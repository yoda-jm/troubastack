/**
 * T39 guard: the "Lyrics & chords" source pane is a syntax-highlighted editor — a colored
 * <pre> overlay behind a transparent-text <textarea>. Token classes must match the dialect
 * (# title / ## section / all-chord line / **bold** / plain), editing must round-trip
 * through the textarea, and preview must stay ON-DEMAND (no auto-render on type — the
 * anti-regression of the superseded live-preview idea; T25's decision stands).
 *
 * Red-first: pre-fix there is no `.chart-src-hl` overlay.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

async function openChartEditor(page: Page) {
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  return panel;
}

const SRC = "# Road Song\n## Verse 1\nG        D\nPack a little light\n**bold** words here\n";

test("source pane syntax-highlights the dialect + editing round-trips (T39)", async ({ page }) => {
  await register(page, `hl_${stamp()}`);
  await createBandAndOpen(page, `HLBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openChartEditor(page);

  await panel.getByTestId("chart-source").fill(SRC);
  const pre = panel.locator("pre.chart-src-hl");
  await expect(pre.locator(".hl-title")).toHaveText("# Road Song");
  await expect(pre.locator(".hl-section")).toHaveText("## Verse 1");
  await expect(pre.locator(".hl-chord")).toContainText("G        D"); // an all-chord line
  await expect(pre.locator(".hl-bold")).toHaveText("**bold**"); // markers kept (alignment)
  await expect(pre.locator(".hl-plain").filter({ hasText: "Pack a little light" })).toHaveCount(1);

  // Editing round-trips through the textarea (the caret + value live there).
  await panel.getByTestId("chart-source").fill(SRC + "Am       C\n");
  await expect(panel.getByTestId("chart-source")).toHaveValue(/Am {7}C/);
  await expect(pre.locator(".hl-chord")).toHaveCount(2); // the new chord line is highlighted too
});

test("preview stays ON-DEMAND — typing does NOT auto-render (T39 anti-regression)", async ({
  page,
}) => {
  await register(page, `nr_${stamp()}`);
  await createBandAndOpen(page, `NRBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openChartEditor(page);

  await panel.getByTestId("chart-source").fill("# Title\n## Verse\nsome words\n");
  await page.waitForTimeout(600); // longer than any debounce would be
  await expect(panel.getByTestId("chart-preview")).toHaveCount(0); // NO auto-render

  // The Preview button still renders on demand.
  await panel.getByTestId("chart-preview-btn").click();
  await expect(panel.getByTestId("chart-preview")).toBeVisible();
  await expect(panel.getByTestId("chart-preview")).toHaveAttribute("data", /^blob:/);
});

/**
 * T135 stage 2 (studio): the "New tab" template, the stateful tab highlighting, and the lint that
 * SUGGESTS wrapping a pasted tab (never auto-wraps). Preview stays the on-demand server render, so a
 * tab previews exactly as it bakes.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

async function openDetails(page: Page) {
  await page.getByTestId("my-files-edit").click();
  return page.getByTestId("details-panel");
}

test("New tab seeds a template, highlights as a block, and previews as a PDF (T135)", async ({ page }) => {
  await register(page, `tab_${stamp()}`);
  await createBandAndOpen(page, `TabBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openDetails(page);

  await panel.getByTestId("new-tab-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  // the template pre-fills an open block with the six strings
  await expect(panel.getByTestId("chart-source")).toHaveValue(/\{sot\}[\s\S]*e\|[\s\S]*E\|[\s\S]*\{eot\}/);
  const pre = panel.locator("pre.chart-src-hl");
  await expect(pre.locator(".hl-marker").first()).toContainText("{sot}");
  await expect(pre.locator(".hl-tab").first()).toContainText("e|");

  // preview is on-demand and renders a PDF (the server renders the tab, stage 1)
  await panel.getByTestId("chart-preview-btn").click();
  await expect(panel.getByTestId("chart-preview")).toBeVisible({ timeout: 15000 });
});

test("lint suggests wrapping a pasted tab, and Wrap as tab wraps it (T135)", async ({ page }) => {
  await register(page, `lint_${stamp()}`);
  await createBandAndOpen(page, `LintBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openDetails(page);

  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  await panel.getByTestId("chart-source").fill("# Riff\n\ne|--0--2--0--|\nB|--1--1--1--|\n");

  await expect(panel.getByTestId("tab-lint")).toBeVisible();
  await panel.getByTestId("tab-lint-wrap").click();
  await expect(panel.getByTestId("chart-source")).toHaveValue(/\{start_of_tab\}[\s\S]*\{end_of_tab\}/);
  await expect(panel.getByTestId("tab-lint")).toHaveCount(0); // hint gone once wrapped
  await expect(panel.locator("pre.chart-src-hl .hl-tab").first()).toContainText("e|--0--2--0--|");
});

/**
 * T38 guard: the lyrics dialog's OPT-IN "Label verses & choruses" toggle. VLL's call:
 * build it, but DEFAULT OFF — grouping-only by default, opt-in to `## Verse N` / `##
 * Chorus` labeling (chorus = a stanza that repeats verbatim). The blank-line grouping
 * itself already survives import (T37); this only adds the section headings when asked.
 *
 * Red-first: the toggle doesn't exist pre-T38.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

async function openLyricsDialog(page: Page) {
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("new-lyrics-chart").click();
  await expect(panel.getByTestId("lyrics-dialog")).toBeVisible();
  return panel;
}

// verse / chorus / verse / chorus — the chorus stanza repeats verbatim.
const LYRICS = [
  "First verse line one",
  "First verse line two",
  "",
  "This is the chorus",
  "everybody sing",
  "",
  "Second verse line one",
  "Second verse line two",
  "",
  "This is the chorus",
  "everybody sing",
].join("\n");

test("default OFF: stanzas are grouped but NOT labeled (T38)", async ({ page }) => {
  await register(page, `ls_${stamp()}`);
  await createBandAndOpen(page, `LSBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openLyricsDialog(page);

  await expect(panel.getByTestId("lyrics-sections")).not.toBeChecked(); // default OFF
  await panel.getByTestId("lyrics-text").fill(LYRICS);
  await panel.getByTestId("lyrics-create").click();

  const val = await panel.getByTestId("chart-source").inputValue();
  expect(val).toContain("First verse line one");
  expect(val).toContain("This is the chorus");
  expect(val).not.toContain("## Verse"); // no auto-labeling by default
  expect(val).not.toContain("## Chorus");
});

test("toggle ON: verses numbered, the repeated stanza becomes ## Chorus (T38)", async ({ page }) => {
  await register(page, `lson_${stamp()}`);
  await createBandAndOpen(page, `LSOnBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  const panel = await openLyricsDialog(page);

  await panel.getByTestId("lyrics-text").fill(LYRICS);
  await panel.getByTestId("lyrics-sections").click();
  await expect(panel.getByTestId("lyrics-sections")).toBeChecked(); // settle before create
  await panel.getByTestId("lyrics-create").click();

  const val = await panel.getByTestId("chart-source").inputValue();
  expect(val).toContain("## Verse 1");
  expect(val).toContain("## Verse 2");
  expect(val).toContain("## Chorus");
  expect(val).not.toContain("## Verse 3"); // the 2 chorus stanzas are NOT numbered verses
  // The chorus label precedes the chorus text.
  expect(val).toMatch(/## Chorus\nThis is the chorus/);
});

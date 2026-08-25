/**
 * T60 surface 1 (editor transpose): the chart-source editor's "Transpose…" control
 * transposes a generated chart in place. With "Also update the song key" on, a G→A
 * transpose rewrites the chord rows (G→A, D→E), and the song's Key field reflects the
 * new key. Prefill comes from the song key; the checkbox defaults on when it parses.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

test("editor transpose G→A rewrites the chords and updates the song key (T60)", async ({ page }) => {
  await register(page, `tr_${stamp()}`);
  await createBandAndOpen(page, `TrBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);

  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");

  // Set the song key to G (a parseable "from" → the key path is enabled).
  await panel.getByTestId("meta-key").fill("G");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();

  // Create + save a text chart whose chord row is "G            D".
  await panel.getByTestId("new-text-chart").click();
  await expect(panel.getByTestId("chart-editor")).toBeVisible();
  await panel.getByTestId("chart-source").fill("# Song\n## Verse\nG            D\nwords under the chords here\n");
  await panel.getByTestId("chart-save").click();

  // Re-open the saved (generated) chart's source — Transpose shows for saved charts only.
  await panel.getByTestId("file-chart-edit").click(); // T104: one-click row control, no menu
  await expect(panel.getByTestId("chart-editor")).toBeVisible();

  // Open Transpose: the target key is prefilled from the song key; "also update key" on.
  await panel.getByTestId("chart-transpose-btn").click();
  await expect(panel.getByTestId("transpose-target-key")).toHaveValue("G");
  await expect(panel.getByTestId("transpose-update-key")).toBeChecked();

  // Transpose to A and apply.
  await panel.getByTestId("transpose-target-key").fill("A");
  await panel.getByTestId("transpose-apply").click();

  // The source is rewritten in place (G→A, D→E) …
  await expect(panel.getByTestId("chart-source")).toHaveValue(/A\s+E/);
  await expect(panel.getByTestId("chart-source")).not.toHaveValue(/G\s+D/);
  // … and the song Key field now shows A (the "also update key" side effect).
  await expect(panel.getByTestId("meta-key")).toHaveValue("A");
});

test("Transpose is blocked while the chart editor has unsaved edits (dirty guard, T60)", async ({
  page,
}) => {
  await register(page, `trd_${stamp()}`);
  await createBandAndOpen(page, `TrdBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);

  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("meta-key").fill("G");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();

  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# Song\n## Verse\nG            D\nlyric here\n");
  await panel.getByTestId("chart-save").click();
  await panel.getByTestId("file-chart-edit").click(); // T104: one-click row control, no menu
  await expect(panel.getByTestId("chart-editor")).toBeVisible();

  // Clean: Transpose is available.
  await expect(panel.getByTestId("chart-transpose-btn")).toBeEnabled();

  // Edit the source (now dirty) → Transpose is blocked so Apply can't clobber the edits.
  await panel.getByTestId("chart-source").fill("# Song\n## Verse\nG            D\nEDITED lyric\n");
  await expect(panel.getByTestId("chart-transpose-btn")).toBeDisabled();
  await expect(panel.getByTestId("chart-transpose-btn")).toHaveAttribute(
    "title",
    "Save your chart edits first",
  );
});

test("the chart source is locked while a transpose Apply is in flight (T64 D4)", async ({
  page,
}) => {
  await register(page, `trl_${stamp()}`);
  await createBandAndOpen(page, `TrlBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);

  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("meta-key").fill("G");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();

  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# Song\n## Verse\nG            D\nlyric line\n");
  await panel.getByTestId("chart-save").click();
  await panel.getByTestId("file-chart-edit").click(); // T104: one-click row control, no menu
  await expect(panel.getByTestId("chart-editor")).toBeVisible();

  await panel.getByTestId("chart-transpose-btn").click();
  await panel.getByTestId("transpose-target-key").fill("A");

  // Hold the Apply request open so we can observe the in-flight state.
  let release: () => void = () => {};
  const gate = new Promise<void>((r) => (release = r));
  await page.route(/chart-source:transpose/, async (route) => {
    await gate;
    await route.continue();
  });

  await panel.getByTestId("transpose-apply").click();
  // While the Apply is in flight the source is DISABLED — typing can't be clobbered by the
  // setSource/setSavedSource that lands when the round-trip completes (D4).
  await expect(panel.getByTestId("chart-source")).toBeDisabled();
  release();
  // After it resolves the source is editable again and shows the transposed chords.
  await expect(panel.getByTestId("chart-source")).toBeEnabled();
  await expect(panel.getByTestId("chart-source")).toHaveValue(/A\s+E/);
});

test("transpose Preview renders without persisting; the source is untouched until Apply (T60)", async ({
  page,
}) => {
  await register(page, `trp_${stamp()}`);
  await createBandAndOpen(page, `TrpBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);

  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("meta-key").fill("G");
  await panel.getByTestId("meta-save").click();
  await expect(panel.getByTestId("meta-notice")).toBeVisible();

  await panel.getByTestId("new-text-chart").click();
  await panel.getByTestId("chart-source").fill("# Song\n## Verse\nG            D\nline of lyrics\n");
  await panel.getByTestId("chart-save").click();
  await panel.getByTestId("file-chart-edit").click(); // T104: one-click row control, no menu
  await expect(panel.getByTestId("chart-editor")).toBeVisible();

  await panel.getByTestId("chart-transpose-btn").click();
  await panel.getByTestId("transpose-target-key").fill("A");
  await panel.getByTestId("transpose-preview").click();

  // Preview renders the transposed chart, but the editor source stays G until Apply.
  await expect(panel.getByTestId("chart-preview")).toBeVisible();
  await expect(panel.getByTestId("chart-source")).toHaveValue(/G\s+D/);
});

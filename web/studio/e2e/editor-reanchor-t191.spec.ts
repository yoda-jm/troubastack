/**
 * T191 (realtime path): a mark you MOVE re-anchors to where you put it, so a later text edit does not drag
 * it back. The bug was that a move updated Points but never the anchor, so the serve-side Reproject rebuilt
 * the position from the create-time anchor alone and the mark snapped back to the line it was dragged away
 * from. This drives the REAL editor: draw a highlight on one line of a generated chart, drag it to another
 * line (a move over the live WebSocket → the server re-anchors), then edit the chart source and reload — the
 * mark stays on the line it was moved to, not the one it was created on.
 */
import { test, expect, type Page } from "@playwright/test";
import { openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

type WireObj = { uuid: string; type: string; points: { x: number; y: number }[]; anchor?: { runText: string } };

async function annotations(page: Page, bandId: string, songId: string): Promise<WireObj[]> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return ((await r.json()) as { objects: WireObj[] }).objects;
    },
    [bandId, songId],
  );
}

// A generated chart with many distinct verse lines, so an upper draw and a lower drag land on different runs.
const CHART =
  "# Capo Song\n\n## Verse\nalpha one\nbravo two\ncharlie three\ndelta four\necho five\nfoxtrot six\ngolf seven\nhotel eight\n";
// The reflow edit: the SAME lines with one inserted at the top of the verse — every original line still
// exists (so the moved-to run persists) but everything shifts down by a line.
const CHART2 =
  "# Capo Song\n\n## Verse\nzero zero\nalpha one\nbravo two\ncharlie three\ndelta four\necho five\nfoxtrot six\ngolf seven\nhotel eight\n";

test("a moved highlight re-anchors to its new line and stays there across a text edit (T191)", async ({
  page,
}) => {
  await register(page, `t191_${stamp()}`);
  const band = await createBandAndOpen(page, `T191 ${stamp()}`);
  const songId = await createSongAndOpen(page, `Song ${stamp()}`);
  const bandId = band.id;

  // A generated TEXT chart — anchoring applies to generated charts only.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("new-text-chart").click();
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await page.getByTestId("chart-source").fill(CHART);
  await page.getByTestId("chart-save").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click(); // close Details → the chart renders
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });

  // A personal layer to draw on.
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);

  // Draw a highlight over the FIRST verse line ("alpha one"). The chart's text sits in the top-left of the
  // page (x ~0.04-0.16); these fractions come from the rendered manifest, so the mark overlaps the run.
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("preset-highlight").click();
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const at = (fx: number, fy: number) => ({ x: box.x + box.width * fx, y: box.y + box.height * fy });
  const d0 = at(0.05, 0.119);
  const d1 = at(0.13, 0.133);
  await page.mouse.move(d0.x, d0.y);
  await page.mouse.down();
  await page.mouse.move(d1.x, d1.y, { steps: 8 });
  await page.mouse.up();

  // It anchored to an upper run (generated chart). Record which.
  await expect.poll(async () => (await annotations(page, bandId, songId)).length).toBe(1);
  const createRun = (await annotations(page, bandId, songId))[0].anchor?.runText;
  expect(createRun, "a highlight on a generated chart must anchor to its line").toBeTruthy();

  // Drag it DOWN onto the LAST verse line ("hotel eight") over the live socket — the server re-anchors to
  // where it now sits.
  await page.getByTestId("tool-select").click();
  const m0 = at(0.09, 0.126);
  const m1 = at(0.09, 0.274);
  await page.mouse.move(m0.x, m0.y);
  await page.mouse.down();
  await page.mouse.move(m1.x, m1.y, { steps: 16 });
  await page.mouse.up();

  // THE FIX: the move re-anchored the mark to a DIFFERENT run (the one it was moved onto).
  await expect
    .poll(async () => (await annotations(page, bandId, songId))[0].anchor?.runText, {
      message: "a move must re-anchor the mark to its new line",
    })
    .not.toBe(createRun);
  const movedRun = (await annotations(page, bandId, songId))[0].anchor?.runText;
  expect(movedRun).toBeTruthy();

  // Edit the chart source (insert a line at the top of the verse) and save → re-render; then reload.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-chart-edit").click();
  await expect(page.getByTestId("chart-source")).toHaveValue(/Capo Song/);
  await page.getByTestId("chart-source").fill(CHART2);
  await page.getByTestId("chart-save").click();
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();

  // The mark still belongs to the line it was MOVED to (movedRun), not the one it was created on — it did
  // not snap back. Before the fix its anchor would still be createRun and Reproject would drag it there.
  await expect
    .poll(async () => (await annotations(page, bandId, songId))[0].anchor?.runText, {
      message: "after the text edit the mark must stay on the line it was moved to",
    })
    .toBe(movedRun);
});

/**
 * T177 — the line styles at the JOIN: the editor's controls, and what they write to the wire.
 *
 * The geometry is proven where it belongs — ink's pure functions in vitest, the baked pixels in
 * web/bake. What only a browser can answer is this: does the control exist for the right tool, does a
 * musician's pick survive the round trip through the API, and — the one that keeps old charts honest —
 * does leaving the control alone still write NOTHING.
 */
import { test, expect, type Page } from "@playwright/test";
import { scrollFracIntoBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

type WireStyle = {
  color: string;
  dash?: string;
  ends?: { head?: string; side?: string };
};
type WireObject = { uuid: string; type: string; style: WireStyle };

async function getObjects(page: Page, bandId: string, songId: string): Promise<WireObject[]> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const j = (await r.json()) as { objects: WireObject[] };
      return j.objects;
    },
    [bandId, songId],
  );
}

async function dragPageFrac(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 14) {
  const box = await scrollFracIntoBand(page, fy, ty);
  await page.mouse.move(box.x + box.width * fx, box.y + box.height * fy);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * tx, box.y + box.height * ty, { steps });
  await page.mouse.up();
}

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  const band = await createBandAndOpen(page, `${prefix}Band ${stamp()}`);
  const songId = await createSongAndOpen(page, `${prefix}Song ${stamp()}`);
  await uploadPdf(page);
  return { bandId: band.id, songId };
}

/** A layer to draw on, with the drawer closed again so the ctx bar's ⋯ is reachable (T33). */
async function readyToDraw(page: Page) {
  await openEditorReady(page);
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
}

test("editor: a line's dash and arrow survive the round trip; and Solid/None write nothing", async ({
  page,
}) => {
  const { bandId, songId } = await setup(page, "LineStyle");
  await page.reload();
  await readyToDraw(page);

  // --- an UNSTYLED line first. The controls are deliberately DRIVEN here — picked away and picked
  // back — rather than merely read at their defaults: "solid" has to travel to the wire as ABSENCE,
  // and a test that never touches the control cannot tell absence from a style nobody set.
  await page.getByTestId("tool-line").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toHaveValue("solid");
  await expect(page.getByTestId("style-ends")).toHaveValue("none");
  await page.getByTestId("style-dash").selectOption("dotted");
  await page.getByTestId("style-ends").selectOption("both");
  await page.getByTestId("style-dash").selectOption("solid");
  await page.getByTestId("style-ends").selectOption("none");
  await dragPageFrac(page, 0.15, 0.14, 0.6, 0.14);

  // --- then a dashed line with an arrow at its far end.
  await page.getByTestId("tool-line").click();
  await page.getByTestId("style-more").click(); // the drag above dismissed it
  await page.getByTestId("style-dash").selectOption("dashed");
  await page.getByTestId("style-ends").selectOption("end");
  await dragPageFrac(page, 0.15, 0.3, 0.6, 0.3);

  await expect.poll(async () => (await getObjects(page, bandId, songId)).length).toBe(2);
  const objs = await getObjects(page, bandId, songId);
  const [plain, styled] = objs;

  // Absent means EXACTLY today's drawing — not the word "solid", and not an empty record. An object
  // nobody has restyled since T177 must be byte-identical to what it was before it.
  expect(plain.style.dash).toBeUndefined();
  expect(plain.style.ends).toBeUndefined();

  expect(styled.style.dash).toBe("dashed");
  expect(styled.style.ends).toEqual({ head: "arrow", side: "end" });

  // --- and it READS back: reload, select the styled line, the controls show what was saved.
  await page.reload();
  await openEditorReady(page);
  const box = await scrollFracIntoBand(page, 0.3);
  await page.getByTestId("tool-select").click();
  await page.mouse.click(box.x + box.width * 0.4, box.y + box.height * 0.3);
  await expect(page.getByTestId("selected-bbox")).toBeVisible();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toHaveValue("dashed");
  await expect(page.getByTestId("style-ends")).toHaveValue("end");
});

test("editor: the controls appear for the types that can carry them, and only those", async ({
  page,
}) => {
  await setup(page, "LineCtl");
  await page.reload();
  await readyToDraw(page);

  // A straight line carries both: a pattern, and a decoration on its ends.
  await page.getByTestId("tool-line").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toBeVisible();
  await expect(page.getByTestId("style-ends")).toBeVisible();

  // A rect has a border to dash, but no ends to decorate — offering "Arrow at start" on a
  // rectangle is an offer that does nothing.
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toBeVisible();
  await expect(page.getByTestId("style-ends")).toHaveCount(0);

  // An ellipse is a rect in this respect.
  await page.getByTestId("tool-ellipse").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toBeVisible();
  await expect(page.getByTestId("style-ends")).toHaveCount(0);

  // Text has neither.
  await page.getByTestId("tool-text").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toHaveCount(0);
  await expect(page.getByTestId("style-ends")).toHaveCount(0);

  // Freehand has neither: a dashed pen stroke is a different feature, and this task is not it.
  await page.getByTestId("tool-freehand").click();
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-dash")).toHaveCount(0);
  await expect(page.getByTestId("style-ends")).toHaveCount(0);
});

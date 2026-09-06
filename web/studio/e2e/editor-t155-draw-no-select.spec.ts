/**
 * T155 — a shape you have just drawn should not stay selected.
 *
 * VLL: "apres avoir dessiné une forme elle est selectionnée … on a l'impression qu'on peut la bouger, et
 * c'est encore pire pour les traits et freehand car ça nuit à la lisibilité de ce qu'on a fait."
 *
 * Rule: selection is a state of the SELECT tool. While a drawing tool is armed, a finished stroke leaves the
 * canvas showing the stroke and nothing else — no selection bbox/handles lying on top of the ink — and the
 * tool stays armed for the next mark (marking a chart is several strokes in a row). The object is still
 * created (object-count proves it); it is just not selected. The select tool still selects.
 *
 * Red-first: against the pre-T155 code these draw-then-assert-count-0 checks fail (the draw auto-selected,
 * so selected-bbox count was 1). The TEXT exception is covered separately (text-oneshot.spec.ts).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  await createBandAndOpen(page, `${prefix}Band ${stamp()}`);
  await createSongAndOpen(page, `${prefix}Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
}

/** Drag a stroke across the first page, in page fractions. Does NOT touch the toolbar (arm the tool first). */
async function stroke(page: Page, x0: number, y0: number, x1: number, y1: number) {
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => box.y + box.height * f;
  await page.mouse.move(px(x0), py(y0));
  await page.mouse.down();
  await page.mouse.move(px(x1), py(y1), { steps: 10 });
  await page.mouse.up();
}

async function clickPage(page: Page, x: number, y: number) {
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  await page.mouse.click(box.x + box.width * x, box.y + box.height * y);
}

test("a freshly drawn shape is not selected, and two strokes in a row both persist unselected", async ({
  page,
}) => {
  await setup(page, "t155a");

  // One freehand stroke → it exists, but nothing is selected (the ink is not hidden by a bbox). Strokes
  // stay in the upper-middle band (y ~0.25–0.42), clear of the floating bottom chrome, and are separated
  // in X so the second does not overlap the first.
  await page.getByTestId("tool-freehand").click();
  await stroke(page, 0.2, 0.25, 0.4, 0.4);
  await expect(page.getByTestId("object-count")).toHaveText("1 objects");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);

  // A SECOND stroke without touching the toolbar (the real usage): both exist, neither selected — guards
  // against "fixing" this by clearing the canvas state too aggressively.
  await stroke(page, 0.55, 0.25, 0.78, 0.4);
  await expect(page.getByTestId("object-count")).toHaveText("2 objects");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);
});

test("line and rect are not selected on draw; the select tool still selects them", async ({ page }) => {
  await setup(page, "t155b");

  // All draws stay in the upper-middle band (clear of the floating bottom chrome), separated in X.
  await page.getByTestId("tool-line").click();
  await stroke(page, 0.2, 0.25, 0.42, 0.35);
  await expect(page.getByTestId("object-count")).toHaveText("1 objects");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);

  await page.getByTestId("tool-rect").click();
  await stroke(page, 0.55, 0.25, 0.78, 0.42); // a rect roughly centred at (0.665, 0.335)
  await expect(page.getByTestId("object-count")).toHaveText("2 objects");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);

  // The rule removes selection from DRAWING, not from the product: the select tool selects a drawn shape.
  await page.getByTestId("tool-select").click();
  await clickPage(page, 0.665, 0.335); // inside the rect body
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
});

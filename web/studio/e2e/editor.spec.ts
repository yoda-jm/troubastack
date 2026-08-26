/**
 * Live annotation EDITOR e2e (realtime stack). Builds on the same live backend
 * as the other specs (Go core + Vite SPA, same-origin cookies). Covers:
 *
 *  1. Draw: open the editor, ensure an editable layer, draw a rectangle with
 *     page.mouse over a pdf-page, assert object-count grows and GET annotations
 *     now contains a rect (persisted). Also draws a freehand and a text.
 *  2. Realtime: a 2nd user is invited + accepts, opens the SAME song in a second
 *     browser context; user A draws → user B's object-count rises WITHOUT reload.
 *  3. Delete: select an object + Delete → object-count drops and GET annotations
 *     no longer has it.
 *
 * Screenshots land at /tmp/editor.png (+ optional /tmp/editor-2up.png).
 */
import { test, expect, type Page, type BrowserContext } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

/** Create a band, open it, return its detail URL + id. */

/** GET the persisted annotation doc through the page's same-origin session. */
async function getAnnotations(
  page: Page,
  bandId: string,
  songId: string,
): Promise<{ layers: { id: string }[]; objects: { uuid: string; type: string }[] }> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as { layers: { id: string }[]; objects: { uuid: string; type: string }[] };
    },
    [bandId, songId],
  );
}

/**
 * Drag the mouse across the first pdf-page to draw a shape. Coordinates are
 * fractions of the page box. The page is taller than the viewport, so we scroll
 * its top into view first and keep the drag in the upper region (fractions are
 * applied to the visible band so both endpoints stay on-screen for page.mouse).
 */
async function dragOnPage(
  page: Page,
  fx: number,
  fy: number,
  tx: number,
  ty: number,
  steps = 8,
) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  // Constrain the drawable band to the canvas clear of the floating chrome (below
  // the top pill + .ctx, above the bottom pill) — T27 stage 3.
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => top + bandH * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

/** Click a point on the first pdf-page (page-box fractions), scrolled into view. */
async function clickOnPage(page: Page, fx: number, fy: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  await page.mouse.click(box.x + box.width * fx, top + bandH * fy);
}

const objectCount = (page: Page) =>
  page
    .getByTestId("object-count")
    .innerText()
    .then((t) => parseInt(t, 10));

/** Open the editor: wait for the PDF page + live connection, ensure a layer. */
async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  // Wait for the realtime socket to open (snapshot received).
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls (new-layer / active-layer / delete-object) live in
  // the on-demand drawer (closed by default). Open it as setup. Draws are center-
  // left; the drawer floats top-right and does not intercept them.
  await openDrawer(page, "layers");
}

test("editor: draw rect + freehand + text, persists to annotations", async ({ page }) => {
  await register(page, `editor_${stamp()}`);
  const band = await createBandAndOpen(page, `EditBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `EditSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // No editable layer yet → create one explicitly.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  const before = await objectCount(page);

  // Rectangle: pick the tool, set a color, drag.
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("style-color").fill("#2563eb");
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(before + 1);

  // Freehand stroke.
  await page.getByTestId("tool-freehand").click();
  await dragOnPage(page, 0.15, 0.7, 0.5, 0.8, 12);
  await expect.poll(() => objectCount(page)).toBe(before + 2);

  // Text via the in-app prompt (T91).
  await page.getByTestId("tool-text").click();
  await clickOnPage(page, 0.3, 0.4);
  await page.getByTestId("app-dialog-input").fill("Cue!");
  await page.getByTestId("app-dialog-confirm").click();
  await expect.poll(() => objectCount(page)).toBe(before + 3);

  await page.screenshot({ path: "/tmp/editor.png", fullPage: true });

  // Persisted: GET annotations now has a rect among the 3 objects.
  const doc = await getAnnotations(page, band.id, songId);
  expect(doc.objects.length).toBeGreaterThanOrEqual(3);
  expect(doc.objects.some((o) => o.type === "rect")).toBeTruthy();
  expect(doc.objects.some((o) => o.type === "freehand")).toBeTruthy();
  expect(doc.objects.some((o) => o.type === "text")).toBeTruthy();
});

test("editor realtime: user A draws → user B sees it without reload", async ({ browser }) => {
  const ctxA: BrowserContext = await browser.newContext();
  const ctxB: BrowserContext = await browser.newContext();
  const a = await ctxA.newPage();
  const b = await ctxB.newPage();

  const userB = `rt_b_${stamp()}`;
  const bandName = `RTBand ${stamp()}`;

  // A registers, makes a band + song, uploads a PDF.
  await register(a, `rt_a_${stamp()}`);
  const band = await createBandAndOpen(a, bandName);
  const songId = await createSongAndOpen(a, `RTSong ${stamp()}`);
  await uploadPdf(a);

  // A invites B by username.
  await a.goto(band.url);
  await a.getByTestId("invite-toggle").click();
  await a.getByTestId("invite-identifier").fill(userB);
  await a.getByTestId("invite-submit").click();
  await expect(a.getByTestId("invite-notice")).toBeVisible();

  // B registers + accepts the invite.
  await register(b, userB);
  await b.getByTestId("nav-invites").click();
  await b.getByTestId("invite-accept").click();
  await expect(b.getByTestId("invites-empty")).toBeVisible();

  // Both open the SAME song editor.
  const songUrl = `${band.url}/songs/${songId}`;
  await a.goto(songUrl);
  await openEditorReady(a);
  await b.goto(songUrl);
  await openEditorReady(b);

  const bBefore = await objectCount(b);

  // A creates an editable layer + draws a rectangle.
  await a.getByTestId("new-layer").click();
  await expect(a.getByTestId("active-layer")).not.toHaveValue("");
  await a.getByTestId("tool-rect").click();
  await a.getByTestId("style-color").fill("#e11d48");
  await dragOnPage(a, 0.25, 0.3, 0.65, 0.6);

  // B's live UI reflects the new object WITHOUT a reload (echo reached B).
  await expect.poll(() => objectCount(b), { timeout: 8_000 }).toBe(bBefore + 1);

  await a.screenshot({ path: "/tmp/editor-2up.png", fullPage: true });

  await ctxA.close();
  await ctxB.close();
});

test("editor: select + delete removes an object", async ({ page }) => {
  await register(page, `del_${stamp()}`);
  const band = await createBandAndOpen(page, `DelBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `DelSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  // Draw a rectangle to delete.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.3, 0.3, 0.7, 0.6);
  await expect.poll(() => objectCount(page)).toBe(1);

  const docBefore = await getAnnotations(page, band.id, songId);
  const rect = docBefore.objects.find((o) => o.type === "rect")!;
  expect(rect).toBeTruthy();

  // Select it (click its center) then delete via the button.
  await page.getByTestId("tool-select").click();
  await clickOnPage(page, 0.5, 0.45);
  await expect(page.getByTestId("delete-object")).toBeEnabled();
  await page.getByTestId("delete-object").click();

  await expect.poll(() => objectCount(page)).toBe(0);

  // Persisted: GET annotations no longer lists that object as live.
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, band.id, songId);
      return doc.objects.some((o) => o.uuid === rect.uuid);
    })
    .toBeFalsy();
});

/**
 * T51 guard: the "Icon" tool stamps a tinted glyph as a page-space annotation. Pick a
 * glyph from the floating palette, drag a bbox on the score → an `icon` object persists
 * (survives reload), moves like any object. VLL's workflow: mark a verse, stamp "shaker"
 * beside it. Red-first: no Icon tool / the server drops the `icon` type pre-T51.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers"); // layer controls live in the on-demand drawer
}
// Drag/click within the drawable band that clears the floating chrome (T27 stage 3).
async function bandPoint(page: Page, fx: number, fy: number) {
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  return { x: box.x + box.width * fx, y: top + bandH * fy };
}
async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const a = await bandPoint(page, fx, fy);
  const b = await bandPoint(page, tx, ty);
  await page.mouse.move(a.x, a.y);
  await page.mouse.down();
  await page.mouse.move(b.x, b.y, { steps });
  await page.mouse.up();
}
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));
type Obj = { uuid: string; type: string; text: string; style: { color: string } };
async function icons(page: Page, bandId: string, songId: string): Promise<Obj[]> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const j = (await r.json()) as { objects: Obj[] };
      return j.objects.filter((o) => o.type === "icon");
    },
    [bandId, songId] as const,
  );
}

test("place a blue shaker icon → persists across reload; moves (T51)", async ({ page }) => {
  await register(page, `icn_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `IcnBand ${stamp()}`);
  const songId = await createSongAndOpen(page, "Slide Away");
  await uploadPdf(page);
  await page.reload(); // fresh load auto-selects the uploaded file → the page renders
  await openEditorReady(page);

  // An editable layer to draw on.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  const before = await objectCount(page);

  // Select the Icon tool → the glyph palette appears; pick shaker; tint blue.
  await page.getByTestId("tool-icon").click();
  await expect(page.getByTestId("icon-palette")).toBeVisible();
  await page.getByTestId("icon-pick-shaker").click();
  await page.getByTestId("style-color").fill("#2563eb");

  // Drag a bbox on the page → stamps the icon.
  await dragOnPage(page, 0.4, 0.35, 0.6, 0.6);
  await expect.poll(() => objectCount(page)).toBe(before + 1);

  // The icon object exists server-side: type "icon", glyph id "shaker", blue tint.
  await expect.poll(async () => (await icons(page, bandId, songId)).length).toBe(1);
  const [obj] = await icons(page, bandId, songId);
  expect(obj.text).toBe("shaker");
  expect(obj.style.color.toLowerCase()).toBe("#2563eb");

  // Persists across a full reload.
  await page.reload();
  await openEditorReady(page);
  await expect.poll(async () => (await icons(page, bandId, songId)).length).toBe(1);

  // It's a normal object — select it and nudge it; still exactly one icon, same glyph.
  await page.getByTestId("tool-select").click();
  const c = await bandPoint(page, 0.5, 0.475); // inside the stamped bbox
  await page.mouse.click(c.x, c.y);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await dragOnPage(page, 0.5, 0.475, 0.65, 0.6);
  await expect.poll(async () => (await icons(page, bandId, songId)).length).toBe(1);
  expect((await icons(page, bandId, songId))[0].text).toBe("shaker");
});

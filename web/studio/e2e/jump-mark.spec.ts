/**
 * P206 — the Jump mark authoring tool (first cut). A jump is a PAIR of matching icon landmarks (same
 * glyph + colour): the destination is placed first, then the source, which carries jumpTo = the
 * destination's uuid. Reuses OBJECT_TYPE_ICON (no new type); the tool offers only landmark glyphs.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await register(page, `jm_${stamp()}`);
  await createBandAndOpen(page, `JmBand ${stamp()}`);
  await createSongAndOpen(page, `JmSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

type IconRow = { uuid: string; jumpTo?: string; points: { x: number; y: number }[] };

/** The persisted icon objects of the open song — the ground truth the canvas is drawn from. */
async function readIcons(page: Page): Promise<IconRow[]> {
  return await page.evaluate(async () => {
    const m = location.pathname.match(/\/bands\/([^/]+)\/songs\/([^/]+)/);
    if (!m) return [];
    const [, bandId, songId] = m;
    const doc = await (
      await fetch(`/api/bands/${bandId}/songs/${songId}/annotations`, { credentials: "same-origin" })
    ).json();
    return (doc.objects ?? []).filter((o: { type: string }) => o.type === "icon");
  });
}

/** The pair as {source (carries jumpTo), dest}, each by its first point — enough to see one end move. */
async function readEnds(page: Page) {
  const icons = await readIcons(page);
  const source = icons.find((o) => o.jumpTo)!;
  const dest = icons.find((o) => !o.jumpTo)!;
  return { source: source.points[0], dest: dest.points[0] };
}

// P206: a jump is placed by a CLICK (a fixed-size stamp), not a drag.
async function clickAt(page: Page, cx: number, cy: number) {
  const cb = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  await page.mouse.click(cb.x + cx, cb.y + cy);
}

test("jump tool offers ONLY landmark glyphs (curated), not cue stamps", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  const palette = page.getByTestId("jump-palette");
  await expect(palette).toBeVisible();
  // The landmark set is present…
  await expect(palette.getByRole("button", { name: "segno" })).toBeVisible();
  await expect(palette.getByRole("button", { name: "coda" })).toBeVisible();
  // …and cue stamps are NOT (curation via glyph `kind`).
  await expect(palette.getByRole("button", { name: "mic" })).toHaveCount(0);
  await expect(palette.getByRole("button", { name: "shaker" })).toHaveCount(0);
});

test("two placements create a matching pair; the source carries jumpTo = the destination (VLL)", async ({
  page,
}) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();

  await clickAt(page, 260, 520); // destination first — a CLICK stamps it (no drag)
  await expect(page.getByText(/now place the source/i)).toBeVisible(); // the two-step guides you
  await clickAt(page, 260, 300); // source second

  await expect(page.getByText("2 objects")).toBeVisible(); // both landmarks placed

  // Prove the wiring: exactly one object carries jumpTo, and it points at the OTHER object's uuid,
  // both are icons, both carry the same glyph (segno). Read it back from the persisted annotations.
  const pair = await page.evaluate(async () => {
    const m = location.pathname.match(/\/bands\/([^/]+)\/songs\/([^/]+)/);
    if (!m) return null;
    const [, bandId, songId] = m;
    const doc = await (
      await fetch(`/api/bands/${bandId}/songs/${songId}/annotations`, { credentials: "same-origin" })
    ).json();
    const objs = (doc.objects ?? []).filter((o: { type: string }) => o.type === "icon");
    const sources = objs.filter((o: { jumpTo?: string }) => o.jumpTo);
    return {
      icons: objs.length,
      sources: sources.length,
      pointsAtRealDest: sources.every((s: { jumpTo?: string }) => objs.some((o: { uuid: string }) => o.uuid === s.jumpTo)),
      sameGlyph: objs.every((o: { text: string }) => o.text === objs[0].text),
    };
  });
  expect(pair, "annotations read back").not.toBeNull();
  expect(pair!.icons).toBe(2);
  expect(pair!.sources).toBe(1); // one-way: only the source carries a target
  expect(pair!.pointsAtRealDest).toBe(true);
  expect(pair!.sameGlyph).toBe(true);
});

test("the size lives in the toolbar (a mm readout) with a live box hint (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  // The size control is in the toolbar (not the palette): a slider + a mm readout.
  const size = page.getByTestId("jump-size");
  await expect(size).toBeVisible();
  const value = page.getByTestId("jump-size-value");
  await expect(value).toHaveText(/^\d+ mm$/);
  const before = await value.textContent();
  // Dragging the size updates the readout AND flashes the hint box.
  await size.focus();
  await page.keyboard.press("ArrowRight");
  await page.keyboard.press("ArrowRight");
  await expect(value).not.toHaveText(before ?? "");
  await expect(page.getByTestId("style-size-preview").locator(".size-hud-box")).toBeVisible();
});

test("selecting ONE end selects BOTH and draws the segment (VLL: \"selecting one should select both\")", async ({
  page,
}) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source
  // Switch to select and pick just ONE end…
  await page.getByTestId("tool-select").click();
  await clickAt(page, 300, 300);
  // …and BOTH ends highlight, tied by the dashed segment. The pair is SHOWN (Fable's ruling on 504b435e:
  // "even if both selected" grants the state) — the next two tests prove it is not WELDED.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);
  await expect(page.locator(".jump-segment").first()).toBeVisible();
});

// The teeth for the ruling: with BOTH ends selected, a drag on one end must move that end ALONE. If the
// pair were welded (the group move this selection used to trigger), the partner would travel by the same
// delta and this assertion would fail on the partner, not on the dragged end.
test("with both selected, dragging one end moves ONLY that end (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source (carries jumpTo)
  await page.getByTestId("tool-select").click();
  await clickAt(page, 300, 300); // selects BOTH ends, focused on the source
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);

  const before = await readEnds(page);
  const cb = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  await page.mouse.move(cb.x + 300, cb.y + 300);
  await page.mouse.down();
  await page.mouse.move(cb.x + 380, cb.y + 300, { steps: 8 }); // drag the SOURCE right
  await page.mouse.up();
  await expect
    .poll(async () => (await readEnds(page)).source.x > before.source.x + 0.02)
    .toBe(true); // the grabbed end travelled…
  const after = await readEnds(page);
  expect(Math.abs(after.dest.x - before.dest.x)).toBeLessThan(0.005); // …and its partner stayed put
  expect(Math.abs(after.dest.y - before.dest.y)).toBeLessThan(0.005);
});

test("with both selected, Delete removes ONLY the end you grabbed (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source
  await page.getByTestId("tool-select").click();
  await clickAt(page, 300, 300); // both selected, source focused
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);

  await page.keyboard.press("Delete");
  // One landmark survives — the destination, the end that was NOT grabbed.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);
  await expect.poll(async () => (await readIcons(page)).length).toBe(1);
  const [survivor] = await readIcons(page);
  expect(survivor.jumpTo ?? "").toBe(""); // the survivor is the destination (it never carried a target)
});

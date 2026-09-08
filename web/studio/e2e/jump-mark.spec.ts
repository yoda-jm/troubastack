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

test("selecting one end selects the WHOLE pair and draws the dashed segment (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source
  // Switch to select and pick just ONE end…
  await page.getByTestId("tool-select").click();
  await clickAt(page, 300, 300);
  // …both ends are selected, and the segment is drawn between them.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);
  await expect(page.locator(".jump-segment").first()).toBeVisible();
});

/**
 * P206 — the Jump mark authoring tool (first cut). A jump is a PAIR of matching icon landmarks (same
 * glyph + colour): the destination is placed first, then the source, which carries jumpTo = the
 * destination's uuid. Reuses OBJECT_TYPE_ICON (no new type); the tool offers only landmark glyphs.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
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

/** Click at a FRACTION of the page box. Fixed pixels are a trap in the two-file layout: the canvas is
 *  shorter there, and a y that overshoots lands on the file strip below — which switches files silently
 *  instead of placing a mark, so the test fails somewhere else entirely. Keep the fractions in the upper
 *  part of the page: mouse.click takes VIEWPORT coordinates, so a point far down a tall canvas is below
 *  the fold and never reaches it. */
async function clickFrac(page: Page, fx: number, fy: number) {
  // Switching file tabs remounts the canvas, so wait for it before measuring — boundingBox() on a
  // detached element is null, which fails as an unrelated TypeError three lines later.
  const canvas = page.getByTestId("edit-canvas").first();
  await expect(canvas).toBeVisible();
  const cb = (await canvas.boundingBox())!;
  await page.mouse.click(cb.x + cb.width * fx, cb.y + cb.height * fy);
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

test("selecting ONE end selects only THAT end, and the link is shown (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source
  await page.getByTestId("tool-select").click();
  await clickAt(page, 300, 300);
  // VLL, 2026-09-08: "if one is selected the other is not selected, but we see the link". The pairing is
  // shown by the SEGMENT, never by selecting something the user did not pick.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.locator(".jump-segment").first()).toBeVisible();
});

// "if both are really selected they move together" (VLL) — genuinely selecting both (a marquee) makes an
// ordinary multi-selection, which group-moves like any other. Nothing special-cases a jump: this test
// exists to pin that the pair is NOT welded by selection and NOT exempted from grouping either.
test("when both ends are REALLY selected, they move together (VLL)", async ({ page }) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 520); // destination
  await clickAt(page, 300, 300); // source
  await page.getByTestId("tool-select").click();

  // Marquee from empty space around BOTH ends.
  const cb = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  await page.mouse.move(cb.x + 180, cb.y + 230);
  await page.mouse.down();
  await page.mouse.move(cb.x + 460, cb.y + 600, { steps: 8 });
  await page.mouse.up();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);

  const before = await readEnds(page);
  // Drag from INSIDE the selection (between the two marks) — a group move.
  await page.mouse.move(cb.x + 300, cb.y + 410);
  await page.mouse.down();
  await page.mouse.move(cb.x + 380, cb.y + 410, { steps: 8 });
  await page.mouse.up();
  await expect.poll(async () => (await readEnds(page)).source.x > before.source.x + 0.02).toBe(true);
  const after = await readEnds(page);
  const dSource = after.source.x - before.source.x;
  const dDest = after.dest.x - before.dest.x;
  expect(Math.abs(dDest - dSource)).toBeLessThan(0.005); // the SAME delta — they travelled together
  expect(Math.abs(after.dest.y - before.dest.y)).toBeLessThan(0.005);
});

test("deleting one end clears the survivor's dangling jumpTo; undo restores the pair whole (Fable)", async ({
  page,
}) => {
  await openEditorReady(page);
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickAt(page, 300, 540); // destination
  await clickAt(page, 300, 300); // source (carries jumpTo = destination)
  await page.getByTestId("tool-select").click();

  const destUuid = (await readIcons(page)).find((o) => !o.jumpTo)!.uuid;
  await clickAt(page, 300, 540); // grab the DESTINATION — the end the survivor points at
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await page.keyboard.press("Delete");

  // The source survives, and its pointer is gone — no reference to a deleted object is persisted.
  await expect.poll(async () => (await readIcons(page)).length).toBe(1);
  await expect.poll(async () => (await readIcons(page))[0].jumpTo ?? "").toBe("");

  // Undo restores BOTH: the destination comes back and the source points at it again.
  await page.keyboard.press("Control+z");
  await expect.poll(async () => (await readIcons(page)).length).toBe(2);
  await expect
    .poll(async () => (await readIcons(page)).find((o) => o.uuid !== destUuid)?.jumpTo ?? "")
    .toBe(destUuid);
});

// P206 ⟨D2⟩ (Fable, from VLL's "somewhere in the same pdf"): a jump's two ends must be on the SAME FILE —
// two files are two independent page spaces, and the bake drops the pair. The reason this needs a Studio
// flag at all is that the doomed pair is INDISTINGUISHABLE from a valid one while authoring: ends on two
// pages of one file draw no segment either. So the author sees nothing wrong until a bake warning names
// the song (not the mark), possibly days later.
test("a pair placed across two files is flagged in the page, and says so at placement (⟨D2⟩)", async ({
  page,
}) => {
  await register(page, `jx_${stamp()}`);
  await createBandAndOpen(page, `JxBand ${stamp()}`);
  await createSongAndOpen(page, `JxSong ${stamp()}`);
  // Two files on one song — the shared uploadPdf asserts exactly one row, so upload locally (T116).
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("my-files-edit").click();
    await page.getByTestId("file-input").setInputFiles(fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url)));
    await page.getByTestId("file-upload").click();
    await page.getByTestId("my-files-edit").click();
  }
  await page.reload();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await expect(page.getByTestId("file-tab")).toHaveCount(2);

  // Destination on file A…
  await page.getByTestId("file-tab").nth(0).click();
  await page.getByTestId("tool-jump").click();
  await page.getByTestId("jump-palette").getByRole("button", { name: "segno" }).click();
  await clickFrac(page, 0.5, 0.3);
  await expect(page.getByText(/now place the source/i)).toBeVisible();

  // …source on file B. It is PLACED (VLL prefers a flag over a refused gesture) and told to the author.
  await page.getByTestId("file-tab").nth(1).click();
  await clickFrac(page, 0.5, 0.3);
  await expect(page.getByText(/same part/i)).toBeVisible();
  // And it stays visible on the page as a red mark, with no selection needed to see it.
  await expect(page.getByTestId("jump-broken")).toHaveCount(1);

  // The valid case must NOT be flagged: a pair placed entirely on file B draws no flag. (Without this
  // half the test would pass on a component that flags EVERY jump.)
  await clickFrac(page, 0.22, 0.2);
  await expect(page.getByText(/now place the source/i)).toBeVisible();
  await clickFrac(page, 0.22, 0.38);
  await expect(page.getByTestId("file-tab").nth(1)).toHaveAttribute("aria-selected", "true"); // still on B
  await expect(page.getByTestId("jump-broken")).toHaveCount(1); // still only the cross-file source
});

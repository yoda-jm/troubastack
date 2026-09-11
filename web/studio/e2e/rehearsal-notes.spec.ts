/**
 * T170 — a rehearsal note reaches Studio as a REFERENCE UNDERLAY and never as an annotation.
 *
 * The one rule that shapes the whole feature (VLL): the note is a bitmap precisely so it cannot
 * be mistaken for a mark. You print it between the chart and your own layers and recopy what
 * still matters by hand. So the assertions here are about POSITION IN THE STACK and about the
 * absence of every affordance an object has.
 *
 * Occlusion is MEASURED, not asserted with toBeVisible(). That is the lesson of `d70fdb14`:
 * toBeVisible() reports an element that is fully covered by another as visible, which is how a
 * chip rendered underneath the toolbar shipped green. Where two things overlap, the only honest
 * question is which colour is at the pixel.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createBandAndOpen, createSongAndOpen, register, stamp, uploadPdf } from "./setup-helpers";

/** The fixture is a 1600×2261 RGBA PNG — the real note's dimensions — carrying one OPAQUE
 *  MAGENTA block on an otherwise fully transparent ground, covering x∈[0.100,0.563],
 *  y∈[0.071,0.310] of the page. That is the shape of a real note (a small opaque scribble on
 *  clear pixels).
 *
 *  Magenta is not a decorative choice. The first version of this fixture was crimson
 *  (220,20,60) and the default pen is #e11d48 (225,29,72) — within the tolerance, so the
 *  measurement counted the MARK as note ink and the whole occlusion test passed with the
 *  underlay deliberately moved to the top of the stack. Note ink and mark ink must be
 *  separable by colour or nothing below means anything. */
const NOTE_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));
function noteImage(): Buffer {
  return readFileSync(NOTE_PATH);
}

const NOTE_INK = { r: 255, g: 0, b: 255 }; // the fixture block
const MARK_INK = { r: 225, g: 29, b: 72 }; // DEFAULT_STYLE.color, #e11d48
/** The fixture block as a fraction of the page box, so a count can be restricted to where the
 *  note and the mark actually overlap. */
const BLOCK = { x0: 0.1, x1: 0.5625, y0: 0.0708, y1: 0.3096 };

/**
 * countColour decodes a screenshot INSIDE the browser (where a canvas exists) and counts pixels
 * near `want`, optionally only within a sub-rectangle. This is the honest answer to "what is on
 * screen here", which toBeVisible() cannot give for anything another element may cover — the
 * lesson of `d70fdb14`.
 */
async function countColour(
  page: Page,
  shot: Buffer,
  want: { r: number; g: number; b: number },
  rect?: { x0: number; x1: number; y0: number; y1: number },
): Promise<number> {
  return page.evaluate(
    async ([b64, w, box]: [string, typeof want, typeof rect]) => {
      const g = globalThis as any;
      const img = new g.Image();
      img.src = "data:image/png;base64," + b64;
      await img.decode();
      const c = g.document.createElement("canvas");
      c.width = img.naturalWidth;
      c.height = img.naturalHeight;
      const ctx = c.getContext("2d");
      ctx.drawImage(img, 0, 0);
      const x0 = Math.round((box ? box.x0 : 0) * c.width);
      const x1 = Math.round((box ? box.x1 : 1) * c.width);
      const y0 = Math.round((box ? box.y0 : 0) * c.height);
      const y1 = Math.round((box ? box.y1 : 1) * c.height);
      const d = ctx.getImageData(x0, y0, Math.max(1, x1 - x0), Math.max(1, y1 - y0)).data;
      let n = 0;
      for (let i = 0; i < d.length; i += 4) {
        if (Math.abs(d[i] - w.r) < 16 && Math.abs(d[i + 1] - w.g) < 16 && Math.abs(d[i + 2] - w.b) < 16) n++;
      }
      return n;
    },
    [shot.toString("base64"), want, rect] as [string, typeof want, typeof rect],
  );
}

/** waitInkCommitted waits until the stroke has left the WET canvas and landed on the dry
 *  annotation layer. Without it a screenshot catches the wet preview, which is the LAST child of
 *  the stack and therefore paints above everything — including an underlay that has been wrongly
 *  moved to the top. That is precisely how the first version of this test proved nothing. */
async function waitInkCommitted(page: Page) {
  await expect
    .poll(async () =>
      page.evaluate(() => {
        const doc = (globalThis as unknown as { document: any }).document;
        const ink = (sel: string) => {
          const c = doc.querySelector(sel);
          if (!c || !c.width) return 0;
          const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
          let n = 0;
          for (let i = 3; i < d.length; i += 4) if (d[i] > 20) n++;
          return n;
        };
        return { dry: ink('[data-testid="annotation-overlay"]'), wet: ink('[data-testid="edit-canvas"]') };
      }),
    )
    .toEqual(expect.objectContaining({ wet: 0 }));
  const dry = await page.evaluate(() => {
    const doc = (globalThis as unknown as { document: any }).document;
    const c = doc.querySelector('[data-testid="annotation-overlay"]');
    const d = c.getContext("2d").getImageData(0, 0, c.width, c.height).data;
    let n = 0;
    for (let i = 3; i < d.length; i += 4) if (d[i] > 20) n++;
    return n;
  });
  expect(dry, "the stroke never reached the dry annotation layer").toBeGreaterThan(0);
}

async function putNote(
  page: Page,
  bandId: string,
  songId: string,
  pageIndex: number,
  opts: { rasterHash?: string; overwrite?: boolean } = {},
) {
  const res = await page.request.fetch(
    `/api/bands/${bandId}/songs/${songId}/rehearsal-notes/${pageIndex}` +
      (opts.overwrite ? "?overwrite=1" : ""),
    {
      method: "PUT",
      multipart: {
        file: { name: "note.png", mimeType: "image/png", buffer: noteImage() },
        rasterHash: opts.rasterHash ?? "raster-hash-from-the-tablet",
        concertId: "concert-e2e",
        concertRev: "8",
        takenAs: "member-e2e",
        width: "1600",
        height: "2261",
      },
    },
  );
  return res;
}

async function openSongWithPdf(page: Page) {
  const who = stamp();
  await register(page, `notes${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);
  await uploadPdf(page);
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  return { bandId: band.id, songId };
}

test("no notes, no chip — the feature does not advertise itself to someone who never used it", async ({
  page,
}) => {
  await openSongWithPdf(page);
  await expect(page.getByTestId("rehearsal-notes-chip")).toHaveCount(0);
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(0);
});

test("the underlay sits between the chart raster and the annotation layer, on the same box", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();

  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();
  const underlay = page.getByTestId("rehearsal-underlay").first();
  await expect(underlay).toHaveCount(1);

  // DOM ORDER inside the SAME .pdf-page: raster, then underlay, then the annotation canvas.
  // This is the feature; anything else either hides the chart or hides the musician's marks.
  const order = await page.evaluate(() => {
    const doc = (globalThis as unknown as { document: any }).document;
    const stack = doc.querySelector('[data-testid="pdf-page"]');
    const kids = Array.from(stack.children) as any[];
    const idx = (pred: (el: any) => boolean) => kids.findIndex(pred);
    return {
      raster: idx((el) => el.classList.contains("pdf-canvas")),
      under: idx((el) => el.classList.contains("rehearsal-underlay")),
      overlay: idx((el) => el.classList.contains("annotation-overlay")),
    };
  });
  expect(order.raster, "the chart raster is first").toBeGreaterThanOrEqual(0);
  expect(order.under, "the underlay is inside the page stack").toBeGreaterThan(order.raster);
  expect(order.overlay, "the annotation canvas is ABOVE the underlay").toBeGreaterThan(order.under);

  // and it occupies the same box as the annotation canvas, within a pixel
  const u = (await underlay.boundingBox())!;
  const o = (await page.getByTestId("annotation-overlay").first().boundingBox())!;
  for (const k of ["x", "y", "width", "height"] as const) {
    expect(Math.abs(u[k] - o[k]), `underlay ${k} differs from the overlay by more than 1px`).toBeLessThanOrEqual(1);
  }
});

test("the underlay is above the chart and below a mark — measured on the pixels", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();
  await expect(page.getByTestId("rehearsal-underlay").first()).toHaveCount(1);

  // ---- ABOVE THE CHART. Screenshot the composited page box and count the note's own colour.
  // The chart raster is black-on-white, so any magenta on screen came from the note, and it only
  // reaches the screenshot if the note paints OVER the raster rather than under it.
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const withNote = await countColour(page, await pageEl.screenshot(), NOTE_INK);
  expect(withNote, "the note's ink is nowhere on the rendered page — it is behind the chart raster").toBeGreaterThan(1000);

  // NEGATIVE CONTROL: hide it and the same measurement must go to zero. Without this, a count
  // that was picking up something else entirely would read as a pass forever.
  await page.getByTestId("rehearsal-notes-chip").click();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(0);
  expect(
    await countColour(page, await pageEl.screenshot(), NOTE_INK),
    "the note's colour is on the page even with the underlay hidden — the measurement counts something else",
  ).toBe(0);
  await page.getByTestId("rehearsal-notes-chip").click();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(1);

  // ---- BELOW A MARK. Draw across the note's block, wait for the stroke to leave the WET canvas
  // (which is the last child and paints above everything, underlay included), then look INSIDE
  // the block: the mark's own colour must be there. If the underlay sat above the annotation
  // layer, the note would have covered the stroke and the musician's work would be hidden behind
  // the scribble they are copying.
  const box = (await pageEl.boundingBox())!;
  const midY = (BLOCK.y0 + BLOCK.y1) / 2;
  await page.getByTestId("tool-freehand").click();
  await page.mouse.move(box.x + box.width * (BLOCK.x0 + 0.03), box.y + box.height * midY);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * (BLOCK.x1 - 0.03), box.y + box.height * midY, { steps: 14 });
  await page.mouse.up();
  await waitInkCommitted(page);

  const markInsideNote = await countColour(page, await pageEl.screenshot(), MARK_INK, BLOCK);
  expect(
    markInsideNote,
    "no mark-coloured pixel inside the note's block — the note is painted OVER the annotation layer",
  ).toBeGreaterThan(100);
});

test("the underlay is not a thing you can pick up", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();
  await expect(page.getByTestId("rehearsal-underlay").first()).toHaveCount(1);

  // pointer-events must be off, so the click below genuinely reaches what is underneath
  const pe = await page
    .getByTestId("rehearsal-underlay")
    .first()
    .evaluate((el) => (globalThis as any).getComputedStyle(el).pointerEvents);
  expect(pe).toBe("none");

  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  await page.getByTestId("tool-select").click();
  await page.mouse.click(box.x + box.width * 0.25, box.y + box.height * 0.18);
  // nothing is selected: the selection toolbar never appears for a note
  await expect(page.getByTestId("sel-delete")).toHaveCount(0);
});

test("no bake, no verdict — an unchecked note is not labelled 'page changed'", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();
  await page.getByTestId("rehearsal-notes-more").click();
  await expect(page.getByTestId("rehearsal-note-row")).toHaveCount(1);
  // The e2e server has no bake for "concert-e2e", so pageChanged is null — unknown. Crying
  // "page changed" on every note of a server that never baked would make the tag worthless.
  await expect(page.getByTestId("rehearsal-note-changed")).toHaveCount(0);
});

test("a second send for the same page asks before it destroys the first", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(409);
  expect((await putNote(page, bandId, songId, 0, { overwrite: true })).status()).toBe(200);
});

test("Done, remove takes the row and the underlay, and it stays gone", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();
  await page.getByTestId("rehearsal-notes-more").click();
  await page.getByTestId("rehearsal-note-done").click();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(0);
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toHaveCount(0);
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(0);
});

test("the chip toggles the underlay without touching the note", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  await page.reload();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(1); // default ON
  await page.getByTestId("rehearsal-notes-chip").click();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(0);
  await page.getByTestId("rehearsal-notes-chip").click();
  await expect(page.getByTestId("rehearsal-underlay")).toHaveCount(1);
  // and the note itself is untouched — hiding is not removing
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();
});

test("another member of the same band sees no chip and no underlay", async ({ page, browser }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  const songUrl = page.url();

  // an invite link, accepted by a second user in a separate browser context (own cookie jar)
  const inv = await page.request.post(`/api/bands/${bandId}/invite-links`, {
    data: { role: "member" },
  });
  expect(inv.ok()).toBeTruthy();
  const token = (await inv.json()).token as string;

  const ctx = await browser.newContext();
  const other = await ctx.newPage();
  await register(other, `other${stamp()}`);
  const join = await other.request.post(`/api/invite-links/${token}/accept`, { data: {} });
  expect(join.ok()).toBeTruthy();

  await other.goto(songUrl);
  await expect(other.getByTestId("pdf-page").first()).toBeVisible();
  // A note is one person's reference. Another member must not learn that it exists, let alone
  // see it printed over the chart they are annotating.
  await expect(other.getByTestId("rehearsal-notes-chip")).toHaveCount(0);
  await expect(other.getByTestId("rehearsal-underlay")).toHaveCount(0);
  const mine = await other.request.get(`/api/bands/${bandId}/songs/${songId}/rehearsal-notes`);
  expect((await mine.json()).notes).toEqual([]);
  const bytes = await other.request.get(
    `/api/bands/${bandId}/songs/${songId}/rehearsal-notes/0`,
  );
  expect(bytes.status(), "someone else's note must be 404 — a 403 would confirm it exists").toBe(404);
  await ctx.close();
});

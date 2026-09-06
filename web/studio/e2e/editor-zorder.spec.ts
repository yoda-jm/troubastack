/**
 * T27 stage 2 — floating selection toolbar + per-object z-order + duplicate + color.
 *
 * The toolbar floats by the single, active-editable selection (no layout shift —
 * it's absolute over the canvas). It exposes colour · bring-to-front · send-to-back
 * · duplicate · delete, all driving the object mutations.
 *
 * z-order is asserted on the RENDERED overlay: two overlapping opaque rects (A red,
 * B blue, B drawn last → on top). We read the overlap pixel; bringing A to front
 * flips it red, send-to-back flips it back to blue. This proves the reorder mutation
 * + the within-layer render sort end to end.
 */
import { test, expect, type Page } from "@playwright/test";
import { scrollFracIntoBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

// T27 stage 3: this suite draws LARGE rects (~0.4 page-frac span) to test z-order.
// On the default 720px viewport the fullscreen fit-width page is taller than the
// clear band, so an endpoint runs under the bottom pill. A taller viewport (test
// infra only — no product/assertion change) enlarges the band so both endpoints of
// the big draws land on the canvas at fit-width.
test.use({ viewport: { width: 1280, height: 1100 } });

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // new-layer lives in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function pageXY(page: Page, fx: number, fy: number) {
  const box = await scrollFracIntoBand(page, fy);
  return { x: box.x + box.width * fx, y: box.y + box.height * fy };
}
async function dragRect(page: Page, f: { x0: number; y0: number; x1: number; y1: number }) {
  // Scroll ONCE so the rect's mid-Y is in the clear band (both endpoints share one box).
  const box = await scrollFracIntoBand(page, f.y0, f.y1);
  await page.mouse.move(box.x + box.width * f.x0, box.y + box.height * f.y0);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * f.x1, box.y + box.height * f.y1, { steps: 10 });
  await page.mouse.up();
}
async function clickPage(page: Page, fx: number, fy: number) {
  const p = await pageXY(page, fx, fy);
  await page.mouse.click(p.x, p.y);
}

/** RGB of one pixel of the first page's dry overlay, at page-relative (fx,fy). */
function overlayPixel(page: Page, fx: number, fy: number): Promise<[number, number, number]> {
  return page
    .getByTestId("pdf-page")
    .first()
    .locator(".annotation-overlay")
    .evaluate(
      (el, at) => {
        const c = el as HTMLCanvasElement;
        const ctx = c.getContext("2d")!;
        const x = Math.floor(at.fx * c.width);
        const y = Math.floor(at.fy * c.height);
        const d = ctx.getImageData(x, y, 1, 1).data;
        return [d[0], d[1], d[2]] as [number, number, number];
      },
      { fx, fy },
    );
}
const isReddish = ([r, , b]: [number, number, number]) => r > b + 30;
const isBluish = ([r, , b]: [number, number, number]) => b > r + 30;

const setSelColor = (page: Page, c: string) => page.getByTestId("sel-color").fill(c);

test("editor: selection toolbar reorders z (render), duplicates, recolors, deletes", async ({
  page,
}) => {
  // This is a legitimately heavy end-to-end test (register → band → song → upload →
  // reload → editor → layer → two large draws → six pixel/count polls → select /
  // bring-front / send-back / duplicate / recolour / delete). Every assertion below
  // stays strict; it just brushed the default 30s budget on a loaded CI runner
  // (reproduced red under 4-worker CPU contention: ~31s vs a 26–29s pass). test.slow()
  // triples the budget so contention has headroom — a timeout fix, not a loosened check.
  test.slow();
  await register(page, `zo_${stamp()}`);
  await createBandAndOpen(page, `ZOBand ${stamp()}`);
  await createSongAndOpen(page, `ZOSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // A writable layer + filled rect tool. Colour each rect right after drawing it — since T155 a
  // completed draw no longer selects, so each one is PICKED with the select tool first (the same
  // idiom this test already uses below). Labels stay unambiguous: A red under, B blue on top.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  // Dismiss the drawer so the wide draws / z-order drags aren't intercepted by it.
  await closeDrawer(page);
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("preset-box").click(); // fill + border

  await dragRect(page, { x0: 0.18, y0: 0.18, x1: 0.58, y1: 0.58 }); // A
  await expect.poll(() => objectCount(page)).toBe(1);
  // T155: pick A (nothing is selected after a draw). Re-arming a draw tool clears the selection,
  // so B below is drawn on a clean canvas.
  await page.getByTestId("tool-select").click();
  await clickPage(page, 0.22, 0.22);
  await expect(page.getByTestId("sel-toolbar")).toBeVisible();
  await setSelColor(page, "#ff0000");

  await page.getByTestId("tool-rect").click();
  await dragRect(page, { x0: 0.4, y0: 0.4, x1: 0.8, y1: 0.8 }); // B (drawn last → on top)
  await expect.poll(() => objectCount(page)).toBe(2);
  // T155: pick B at a point inside B ONLY (A spans 0.18–0.58, B spans 0.4–0.8), so the recolour
  // below cannot land on A.
  await page.getByTestId("tool-select").click();
  await clickPage(page, 0.7, 0.7);
  await expect(page.getByTestId("sel-toolbar")).toBeVisible();
  await setSelColor(page, "#0000ff");

  // Overlap centre (≈0.49,0.49): B (blue) is on top. The poll waits for the render
  // to settle on its own — no fixed sleep needed.
  await expect.poll(() => overlayPixel(page, 0.49, 0.49).then(isBluish)).toBe(true);

  // Select A (its own region), bring to front → overlap flips RED.
  await page.getByTestId("tool-select").click();
  await clickPage(page, 0.22, 0.22);
  await expect(page.getByTestId("sel-toolbar")).toBeVisible();
  await page.getByTestId("sel-front").click();
  await expect.poll(() => overlayPixel(page, 0.49, 0.49).then(isReddish)).toBe(true);

  // Send A back → overlap flips back to BLUE.
  await page.getByTestId("sel-back").click();
  await expect.poll(() => overlayPixel(page, 0.49, 0.49).then(isBluish)).toBe(true);

  // Duplicate A (still selected) → object count +1, the copy becomes the selection.
  await clickPage(page, 0.22, 0.22);
  await expect(page.getByTestId("sel-toolbar")).toBeVisible();
  await page.getByTestId("sel-duplicate").click();
  await expect.poll(() => objectCount(page)).toBe(3);

  // Recolour the current selection (the copy) green; the toolbar reflects it.
  await setSelColor(page, "#00cc00");
  await expect.poll(() => page.getByTestId("sel-color").inputValue()).toBe("#00cc00");

  // Delete via the toolbar → back to 2 objects.
  await page.getByTestId("sel-delete").click();
  await expect.poll(() => objectCount(page)).toBe(2);

  // Deselecting hides the toolbar (click an empty corner).
  await clickPage(page, 0.93, 0.06);
  await expect(page.getByTestId("sel-toolbar")).toHaveCount(0);
});

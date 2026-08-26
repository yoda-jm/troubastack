/**
 * T27 stage 1 — Ctrl/⌘-wheel zoom-to-cursor.
 *
 * The load-bearing invariant (Fable): a wheel/pinch zoom must DECOUPLE the
 * visual zoom from rasterization — the live zoom is a cheap CSS transform and
 * the crisp PDF re-raster is committed only ONCE, on wheel-settle. So a fast
 * burst of many Ctrl+wheel ticks must cost exactly ONE raster pass (one per
 * page), never one raster per tick.
 *
 * Two assertions:
 *  1. A burst of N Ctrl+wheel ticks changes the zoom but bumps `pdf-render-count`
 *     by exactly the page count (one pass) — not by N.
 *  2. Plain wheel (no ctrl/meta) does not zoom.
 *  3. After a zoom settles, a normal annotation edit STILL does not re-raster
 *     (the overlay-only edit path survives the new scale) — the no-flicker
 *     invariant holds post-zoom.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";
import { waitRenderStable } from "./render-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}

const renderCount = (page: Page) =>
  page.getByTestId("pdf-render-count").innerText().then((t) => parseInt(t, 10));
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

/** Fire N Ctrl+wheel ticks at the scroll column's centre, synchronously (well
 *  inside the settle window) so the whole burst commits as one raster. */
async function ctrlWheelBurst(page: Page, ticks: number, deltaY: number) {
  await page.getByTestId("viewer-scroll").evaluate(
    (el, { n, dy }) => {
      const r = el.getBoundingClientRect();
      const cx = r.left + r.width / 2;
      const cy = r.top + r.height / 2;
      for (let i = 0; i < n; i++) {
        el.dispatchEvent(
          new WheelEvent("wheel", {
            deltaY: dy,
            ctrlKey: true,
            clientX: cx,
            clientY: cy,
            bubbles: true,
            cancelable: true,
          }),
        );
      }
    },
    { n: ticks, dy: deltaY },
  );
}

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => top + bandH * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

test("editor: a Ctrl+wheel burst zooms but re-rasters exactly once (not per tick)", async ({
  page,
}) => {
  await register(page, `wz_${stamp()}`);
  await createBandAndOpen(page, `WZBand ${stamp()}`);
  await createSongAndOpen(page, `WZSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Pin an explicit zoom so the base scale is deterministic; wait for the 100% re-raster to settle
  // (T115) rather than sleeping, so `before` is captured on a quiesced count.
  const baseline = await renderCount(page);
  await page.getByTestId("zoom-mode").selectOption("100");
  await waitRenderStable(page, baseline);

  const pageCount = await page.getByTestId("pdf-page").count();
  const before = await renderCount(page);

  // A plain wheel (no ctrl) must NOT zoom.
  await page.getByTestId("viewer-scroll").evaluate((el) => {
    const r = el.getBoundingClientRect();
    el.dispatchEvent(
      new WheelEvent("wheel", {
        deltaY: -120,
        clientX: r.left + r.width / 2,
        clientY: r.top + r.height / 2,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await page.waitForTimeout(200);
  await expect(page.getByTestId("zoom-mode")).toHaveValue("100");
  expect(await renderCount(page)).toBe(before);

  // Now a fast Ctrl+wheel burst: many ticks, all inside the settle window.
  await ctrlWheelBurst(page, 8, -40);
  // Wait for the single re-raster to LAND and QUIESCE (T115) — not a fixed sleep. This is what
  // keeps the exactly-once teeth: a per-tick regression climbs past pageCount and only THEN settles,
  // so the delta below fails; the wait never returns mid-climb.
  const afterZoom = await waitRenderStable(page, before);

  // Zoom changed (zoomed IN → a percentage above 100).
  const zoomVal = await page.getByTestId("zoom-mode").inputValue();
  expect(parseInt(zoomVal, 10)).toBeGreaterThan(100);

  // …but the PDF re-rasterized exactly ONCE (one pass = one bump per page),
  // NOT once per wheel tick. This is the decouple-visual-zoom invariant.
  expect(afterZoom - before).toBe(pageCount);
});

test("editor: after a wheel-zoom settles, an annotation edit still does NOT re-raster", async ({
  page,
}) => {
  await register(page, `wz2_${stamp()}`);
  await createBandAndOpen(page, `WZBand ${stamp()}`);
  await createSongAndOpen(page, `WZSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const baseline = await renderCount(page);
  await page.getByTestId("zoom-mode").selectOption("100");
  await waitRenderStable(page, baseline);

  // Zoom in with a Ctrl+wheel burst; wait for the re-raster to land + quiesce (T115).
  const beforeBurst = await renderCount(page);
  await ctrlWheelBurst(page, 8, -40);
  const afterZoom = await waitRenderStable(page, beforeBurst);

  // Draw + move an annotation at the new scale.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(1);
  await page.getByTestId("tool-select").click();
  await dragOnPage(page, 0.4, 0.4, 0.5, 0.5, 10);
  await page.waitForTimeout(400);

  // The edit is overlay-only: no re-raster, even at the zoomed scale.
  expect(await renderCount(page)).toBe(afterZoom);
});

// A "spaced" burst: ticks fired in SEPARATE tasks via in-page setTimeout, `gapMs` apart, on the PAGE
// clock (deterministic — not CDP round-trip latency). ctrlWheelBurst fires all ticks in ONE
// synchronous task, which React batches, so a per-tick raster regression collapses to a single
// raster and is invisible to it; spacing the ticks lets a per-tick regression raster separately.
// gapMs stays INSIDE the 120ms WHEEL_SETTLE_MS window (usePdfDocument.ts), so the correct debounced
// impl still commits exactly ONE pass.
async function ctrlWheelSpaced(page: Page, ticks: number, deltaY: number, gapMs: number) {
  await page.getByTestId("viewer-scroll").evaluate(
    (el, { n, dy, gap }) => {
      const r = el.getBoundingClientRect();
      const cx = r.left + r.width / 2;
      const cy = r.top + r.height / 2;
      for (let i = 0; i < n; i++)
        setTimeout(
          () =>
            el.dispatchEvent(
              new WheelEvent("wheel", { deltaY: dy, ctrlKey: true, clientX: cx, clientY: cy, bubbles: true, cancelable: true }),
            ),
          i * gap,
        );
    },
    { n: ticks, dy: deltaY, gap: gapMs },
  );
  // No sleep to "let the ticks fire": the caller's waitRenderStable blocks until a raster lands
  // (~one debounce past the LAST scheduled tick), which is strictly after all ticks have dispatched.
}

test("editor: wheel ticks spread over time (separate tasks) re-raster exactly once, not per tick (T118)", async ({
  page,
}) => {
  // The exactly-once test above fires its burst SYNCHRONOUSLY, so React batches the per-tick
  // setZoomMode calls and a per-tick raster regression is indistinguishable from the debounced path
  // (verified: the per-tick mutation stays green on that test). This guards the "not once per tick"
  // half directly — 5 ticks 60ms apart, each in its own task but all inside the 120ms settle window:
  // the debounced impl commits ONE pass; a per-tick regression rasters ~5×, so the delta climbs past
  // pageCount and this reddens. (Teeth verified for T118: RED under the per-tick mutation.)
  await register(page, `wzs_${stamp()}`);
  await createBandAndOpen(page, `WZSBand ${stamp()}`);
  await createSongAndOpen(page, `WZSSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const baseline = await renderCount(page);
  await page.getByTestId("zoom-mode").selectOption("100");
  await waitRenderStable(page, baseline);

  const pageCount = await page.getByTestId("pdf-page").count();
  const before = await renderCount(page);

  await ctrlWheelSpaced(page, 5, -30, 60);
  const after = await waitRenderStable(page, before);

  // Zoom actually changed (a spaced burst still zooms in).
  expect(parseInt(await page.getByTestId("zoom-mode").inputValue(), 10)).toBeGreaterThan(100);
  // …and it was exactly ONE debounced pass, NOT one raster per tick.
  expect(after - before).toBe(pageCount);
});

/**
 * T115 — a shared primitive for the editor tests that assert an EXACT PDF raster count after a
 * zoom ("re-rasters exactly once, not once per tick"). Those tests previously slept a fixed amount
 * to let rasterization settle before reading `pdf-render-count`; this replaces the guesswork with a
 * wait that provably cannot return early — so the `=== pageCount` delta keeps its teeth.
 */
import { type Page, expect } from "@playwright/test";

/** The hidden probe: how many PDF page rasters have LANDED (increments once per page, per pass). */
export const renderCount = (page: Page): Promise<number> =>
  page.getByTestId("pdf-render-count").innerText().then((t) => parseInt(t, 10));

/**
 * Wait until PDF rasterization has QUIESCED after an action expected to render, then return the
 * settled count. Two guards make an early return impossible (the failure mode a deflake helper must
 * not have — a wait that is already true reads as rigour while proving nothing):
 *
 *  1. **A pass has landed.** `renderCount > since` — never returns at the pre-action baseline, which
 *     would race an unstarted raster (the settle debounce + the raster both still pending).
 *  2. **It then holds steady.** `samples` CONSECUTIVE equal reads spaced `holdMs` apart — i.e.
 *     `(samples - 1) · holdMs` of confirmed steadiness (default 3 × 200ms = 400ms). `holdMs` clears
 *     both the 120ms wheel-settle debounce (usePdfDocument.ts) and the worst measured intra-pass gap
 *     (~37ms between a 2-page pass's two increments; heavier zooms batch both pages into one commit,
 *     0 gap). So the window cannot land between a per-tick regression's extra passes and return mid-climb.
 *
 * The caller keeps its own exact-count assertion (`after - before === pageCount`); this only
 * replaces the blind settle sleep. Use ONLY where a render is expected — never to prove a NEGATIVE
 * ("no re-raster happened"): guard 1 would hang, and you cannot poll for nothing happening anyway.
 */
export async function waitRenderStable(
  page: Page,
  since: number,
  opts: { holdMs?: number; samples?: number; timeout?: number } = {},
): Promise<number> {
  const { holdMs = 200, samples = 3, timeout = 15_000 } = opts;
  await expect.poll(() => renderCount(page), { timeout }).toBeGreaterThan(since);
  const history: number[] = [];
  await expect
    .poll(
      async () => {
        const n = await renderCount(page);
        history.push(n);
        if (history.length > samples) history.shift();
        return history.length === samples && history.every((v) => v === history[0]);
      },
      { intervals: [holdMs], timeout },
    )
    .toBe(true);
  return renderCount(page);
}

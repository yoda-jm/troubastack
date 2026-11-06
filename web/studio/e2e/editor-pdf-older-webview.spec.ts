/**
 * T209 — a song's PDF opens on a WebView older than Chromium 119, which has no Promise.withResolvers.
 * pdfjs-dist 4.10's default build calls it (page side and worker), so the tablet's Studio couldn't open a song
 * on an un-updated WebView; the legacy build carries its own polyfill. This test removes it from BOTH globals:
 * the page (an init script, before Studio's scripts run) and pdf.js's worker (its script is served with the
 * same removal first), then opens a song and expects it to render.
 *
 * TEETH (measured): the default build → red; the legacy page with the default worker → red too, which is how
 * we know the vector reaches the worker.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";
import { renderCount } from "./render-helpers";

const REMOVE = "delete Promise.withResolvers;\n";

test("a song's PDF renders with no Promise.withResolvers, on the page and in pdf.js's worker", async ({ page }) => {
  await page.addInitScript(() => {
    delete (Promise as unknown as { withResolvers?: unknown }).withResolvers;
  });
  let workerServed = 0;
  await page.route(
    (url) => /pdf\.worker[^/]*\.mjs$/.test(url.pathname),
    async (route) => {
      const res = await route.fetch();
      workerServed++;
      await route.fulfill({ response: res, body: REMOVE + (await res.text()) });
    },
  );
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  page.on("console", (m) => {
    if (m.type() === "error") errors.push(m.text());
  });

  await register(page, `T209_${stamp()}`);
  await createBandAndOpen(page, `T209 Band ${stamp()}`);
  await createSongAndOpen(page, `T209 Song ${stamp()}`);
  // The removal took: nothing has loaded pdf.js yet (its legacy build polyfills it once loaded).
  expect(await page.evaluate(() => typeof (Promise as unknown as { withResolvers?: unknown }).withResolvers)).toBe(
    "undefined",
  );

  await uploadPdf(page);
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect.poll(() => renderCount(page), { timeout: 20_000 }).toBeGreaterThan(0);
  expect(workerServed, "pdf.js's worker was served with the removal").toBeGreaterThan(0);
  expect(errors.filter((e) => /withResolvers/.test(e))).toEqual([]);
});

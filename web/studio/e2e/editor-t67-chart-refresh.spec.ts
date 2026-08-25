/**
 * T67: saving an edited text chart re-renders it in the viewer WITHOUT a manual refresh.
 *
 * The bug: the server re-renders on save (same file id, revision++), but the viewer showed
 * the stale render even after F5 — because the file URL was revision-agnostic (/api/files/{id})
 * so the browser served cached bytes AND the viewer never refetched after save. The fix pins
 * the revision in the URL (?rev=), refetches the viewer on save, and caches by ETag. This
 * spec asserts the viewer fetches the file at the NEW revision in-session, no reload.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

test("editing a chart re-renders the viewer with no manual refresh (T67)", async ({ page }) => {
  await register(page, `t67_${stamp()}`);
  await createBandAndOpen(page, `T67Band ${stamp()}`);
  await createSongAndOpen(page, `T67Song ${stamp()}`);

  // Record every file-blob fetch the viewer issues (the render pulls /api/files/{id}?rev=…).
  const fetches: string[] = [];
  page.on("request", (r) => {
    if (/\/api\/files\/[^/?]+(\?|$)/.test(r.url())) fetches.push(r.url());
  });

  // Create a text chart — it enters the pool and (via the T67 refetch wiring) becomes the
  // viewer's open file.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("new-text-chart").click();
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await page.getByTestId("chart-source").fill("# Road Song\n\n## Verse 1\nG            D\nfirst render\n");
  await page.getByTestId("chart-save").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);

  // Close Details → the chart renders. The render fetch is revision-pinned (?rev=1) so a
  // hard refresh can never serve stale bytes for a later re-render.
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect
    .poll(() => fetches.some((u) => /\/api\/files\/[^/?]+\?rev=1\b/.test(u)), { timeout: 6000 })
    .toBe(true);

  const before = fetches.length;

  // Edit the source and Save chart → re-renders in place at revision 2.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-chart-edit").click(); // T104: one-click row control, no menu
  await expect(page.getByTestId("chart-source")).toHaveValue(/Road Song/);
  await page.getByTestId("chart-source").fill("# Road Song v2\n\n## Chorus\na different second render\n");
  await page.getByTestId("chart-save").click();

  // WITHOUT any reload, the viewer refetches the file at the NEW revision (?rev=2). Red-first:
  // today's revision-agnostic URL + no-refetch never produces a ?rev=2 request.
  await expect
    .poll(() => fetches.slice(before).some((u) => /\/api\/files\/[^/?]+\?rev=2\b/.test(u)), {
      timeout: 8000,
    })
    .toBe(true);
});

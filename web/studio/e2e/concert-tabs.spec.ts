/**
 * T206 — the concert page is "Running order" | "Sound sheet" as two route-based tabs under one shared
 * header. These drive the tab behaviour (deep-link, Back, active marking), prove each tab shows only its
 * own exports, and that the tab strip doesn't break the sheet's sticky-header offset under the topbar.
 */
import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// A concert with one song (the creator on lead vocal ⇒ an amplified channel so the sheet has content).
// Returns ids; leaves the page on the running-order tab.
async function makeConcert(page: Page): Promise<{ id: string; slId: string; name: string }> {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json();
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const song = await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json();
  await page.request.patch(`/api/bands/${id}/songs/${song.song.id}`, { data: { lineup: [{ musicianId: me, partIds: [lead.part.id] }] } });
  const name = `Gig ${stamp()}`;
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name } })).json()).setlist;
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: song.song.id } });
  await page.goto(`/bands/${id}/setlists/${sl.id}`);
  await expect(page.getByTestId("setlist-detail-title")).toContainText(name);
  return { id, slId: sl.id, name };
}

test("concert tabs: deep-link to the sound sheet, Back returns, active marked on both", async ({ page }) => {
  await register(page, `tabs${stamp()}`);
  const { id, slId } = await makeConcert(page);

  // The strip shows on the running-order tab; Running order is active, Sound sheet is not.
  await expect(page.getByTestId("concert-tab-order")).toBeVisible();
  await expect(page.getByTestId("concert-tab-sheet")).toBeVisible();
  await expect(page.getByTestId("concert-tab-order")).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("concert-tab-sheet")).not.toHaveAttribute("aria-current", "page");

  // Click "Sound sheet" → the URL becomes …/sheet and the tab marks active (a real navigation, not a swap).
  await page.getByTestId("concert-tab-sheet").click();
  await expect(page).toHaveURL(new RegExp(`/setlists/${slId}/sheet$`));
  await expect(page.getByTestId("concert-tab-sheet")).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("concert-tab-order")).not.toHaveAttribute("aria-current", "page");
  // The shared header rode across — same concert h1, no re-crumb.
  await expect(page.getByTestId("setlist-detail-title")).toBeVisible();

  // Back returns to the running-order tab (the browser history works because it's a route).
  await page.goBack();
  await expect(page).toHaveURL(new RegExp(`/setlists/${slId}$`));
  await expect(page.getByTestId("concert-tab-order")).toHaveAttribute("aria-current", "page");
});

test("concert tabs: direct load of /sheet shows the shared header + active Sound sheet tab", async ({ page }) => {
  await register(page, `tabsd${stamp()}`);
  const { id, slId, name } = await makeConcert(page);

  await page.goto(`/bands/${id}/setlists/${slId}/sheet`);
  await expect(page.getByTestId("setlist-detail-title")).toContainText(name); // the concert header, not a sheet-only h1
  await expect(page.getByTestId("concert-tab-sheet")).toHaveAttribute("aria-current", "page");
  await expect(page.getByTestId("concert-tab-order")).toBeVisible();
});

test("concert tabs: each tab shows only its own exports", async ({ page }) => {
  await register(page, `tabsx${stamp()}`);
  const { id: bandId, slId } = await makeConcert(page);

  // Running order tab: the running-order PDF; NOT the sheet's PDF/CSV.
  await expect(page.getByTestId("export-setlist-pdf")).toBeVisible();
  await expect(page.getByTestId("sheet-pdf")).toHaveCount(0);
  await expect(page.getByTestId("sheet-csv")).toHaveCount(0);

  // Sound sheet tab: its PDF + CSV; NOT the running-order PDF.
  await page.goto(`/bands/${bandId}/setlists/${slId}/sheet`);
  await expect(page.getByTestId("sheet-pdf")).toBeVisible();
  await expect(page.getByTestId("sheet-csv")).toBeVisible();
  await expect(page.getByTestId("export-setlist-pdf")).toHaveCount(0);
});

test("concert tabs: the tab strip scrolls away; the sheet grid header still pins under the topbar", async ({ page }) => {
  await register(page, `tabss${stamp()}`);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json();
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Long" } })).json()).setlist;
  for (let i = 0; i < 30; i++) {
    const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } })).json()).song;
    await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [{ musicianId: me, partIds: [lead.part.id] }] } });
    await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s.id } });
  }

  await page.goto(`/bands/${id}/setlists/${sl.id}/sheet`);
  await expect(page.getByTestId("setlist-detail-title")).toBeVisible();
  await expect(page.getByTestId("concert-tab-sheet")).toBeVisible();
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));

  // The sheet's sticky thead sits right under the topbar — the tab strip (normal flow) scrolled away and
  // did NOT add itself to the sticky offset (--topbar-h unchanged). Prove the tab strip is gone from view.
  await expect.poll(async () => {
    const topbar = await page.locator("header.topbar").boundingBox();
    const head = await page.locator('[data-testid="sheet-grid"] thead tr').first().boundingBox();
    if (!topbar || !head) return 999;
    return Math.abs(head.y - (topbar.y + topbar.height));
  }).toBeLessThan(2);
  const tab = await page.getByTestId("concert-tab-sheet").boundingBox();
  const topbar = await page.locator("header.topbar").boundingBox();
  // Binding 1 (Fable): assert the strip has scrolled OUT OF VIEW — its BOTTOM edge is at/above the topbar's
  // bottom. `tab.y < topbarBottom` is blind to a sticky strip: with `.concert-tabs{position:sticky;
  // top:--topbar-h}` the strip sticks under the topbar (tab.y 57 vs topbar bottom 57.375 — passes by
  // 0.375px) and covers the grid header, and thead.y is 57 either way so it can't tell them apart. The
  // bottom edge does: ≈ −408.7 clean, ≈ 96.2 stuck (~39px margin each way).
  expect(tab!.y + tab!.height).toBeLessThanOrEqual(topbar!.y + topbar!.height);
});

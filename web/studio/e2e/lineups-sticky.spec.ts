import { test, expect } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T203-FIX3 — the Line-ups grid: a sticky header under the topbar + a sticky song column, so a long/wide list
// never loses its headers or its song titles.
test("sticky header under the topbar + sticky song column, popup still anchored", async ({ page }) => {
  const u = `sticky${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  // Many musicians (wide) + many songs (tall).
  for (let i = 0; i < 8; i++) await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Guest ${i}` } });
  for (let i = 0; i < 45; i++) await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } });

  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await page.getByTestId("lineups-add-common").click(); // seeds voice parts → every musician gets a Voice column
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  // 1. Scroll to the bottom — the header row sits right under the topbar.
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
  await expect.poll(async () => {
    const topbar = await page.locator("header.topbar").boundingBox();
    const owner = await page.getByTestId("lineup-col-owner").first().boundingBox();
    if (!topbar || !owner) return 999;
    return Math.abs(owner.y - (topbar.y + topbar.height));
  }).toBeLessThan(2);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  // 2. Narrow viewport, scroll right — the song-title column stays at x ≈ 0.
  await page.setViewportSize({ width: 480, height: 800 });
  await page.evaluate(() => window.scrollTo(document.body.scrollWidth, 0));
  await expect.poll(async () => {
    const cell = await page.getByTestId("lineup-row").first().locator("td").first().boundingBox();
    return cell ? Math.abs(cell.x) : 999;
  }).toBeLessThan(2);

  // 3. The "+ part" popup still opens below its trigger.
  await page.evaluate(() => window.scrollTo(0, 0));
  await page.getByTestId("lineup-add-part").first().click();
  await expect(page.getByTestId("lineup-assign")).toBeVisible();
  const trig = await page.getByTestId("lineup-add-part").first().boundingBox();
  const pop = await page.getByTestId("lineup-assign").boundingBox();
  expect(pop!.y).toBeGreaterThanOrEqual(trig!.y - 1); // below (or level with) the trigger
});

// T203-FIX3 binding (Fable): the whole thead is one sticky block, NOT per-row offsets — so a taller header row
// (the sound sheet's 3 rows incl. the input row, a long owner name, 125% zoom) can't make the stuck rows
// overlap. The sound sheet is the worst case (3 header rows).
test("sound sheet: stuck header rows stay flush (no overlap) at 125% zoom", async ({ page }) => {
  const u = `sheetz${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json()).part;
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  // a long owner name — stresses the owner row's height
  const guest = (await (await page.request.post(`/api/bands/${id}/musicians`, { data: { name: "Maximiliana Featherstonehaugh-Montmorency" } })).json()).musician.id;
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Long Gig" } })).json()).setlist;
  for (let i = 0; i < 30; i++) {
    const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } })).json()).song;
    await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [{ musicianId: me, partIds: [lead.id] }, { musicianId: guest, partIds: [lead.id] }] } });
    await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s.id } });
  }

  await page.goto(`/bands/${id}/setlists/${sl.id}/sheet`);
  await expect(page.getByTestId("sheet-title")).toBeVisible();
  await expect(page.locator('[data-testid="sheet-grid"] thead tr')).toHaveCount(3);
  await page.evaluate(() => { (document.documentElement.style as unknown as { zoom: string }).zoom = "1.25"; });
  await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));

  // The header stays stuck (visible) after scrolling to the bottom...
  await expect(page.getByTestId("sheet-owner").first()).toBeVisible();
  // ...and the three stuck rows are flush — NO overlap — row k top == row k-1 bottom (±1px), the binding: the
  // whole thead is one sticky block, so the taller input row (and zoom) can't make a row ride over the one above.
  await expect.poll(async () => {
    const trs = page.locator('[data-testid="sheet-grid"] thead tr');
    const n = await trs.count();
    const boxes = [];
    for (let i = 0; i < n; i++) boxes.push(await trs.nth(i).boundingBox());
    if (boxes.some((b) => !b)) return 999;
    let worst = 0;
    for (let i = 1; i < boxes.length; i++) worst = Math.max(worst, Math.abs(boxes[i]!.y - (boxes[i - 1]!.y + boxes[i - 1]!.height)));
    return worst;
  }).toBeLessThan(2);
});

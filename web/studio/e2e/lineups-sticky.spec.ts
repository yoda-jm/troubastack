import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// Read an element's x through a poll. Right after setViewportSize / a scroll the grid re-renders and
// boundingBox() can momentarily return null (the node detaches); a bare `(await boundingBox())!.x` then throws
// "Cannot read properties of null" — the T203-FIX4 flake. Poll until the box is present.
async function boxX(page: Page, sel: string): Promise<number> {
  let x = NaN;
  await expect.poll(async () => {
    const b = await page.locator(sel).first().boundingBox();
    if (!b) return "pending";
    x = b.x;
    return "ok";
  }).toBe("ok");
  return x;
}

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

// T203-FIX4 — the rowSpan "Song" corner meant `thead th:first-child` also hit row 2's first musician header:
// it went white and stuck left over the corner. The corner + song column are now targeted by class.
test("T203-FIX4: first Voice header keeps the tint and scrolls away while the corner stays (grid)", async ({ page }) => {
  const u = `fix4${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  for (let i = 0; i < 8; i++) await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Guest ${i}` } });
  for (let i = 0; i < 6; i++) await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } });
  await page.getByTestId("nav-lineups").click();
  await page.getByTestId("lineups-add-common").click();
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  const grid = '[data-testid="lineup-grid"]';
  const firstSub = `${grid} thead tr:nth-child(2) th`;
  // 1. the first Voice sub-header carries the header tint — the SAME paint as the owner header above it (both
  //    thead cells), and not the body's transparent/surface white (the regression made this cell white). Polled:
  //    add-common seeds the parts asynchronously, so the grid re-renders and a node read too early reads "".
  await expect.poll(async () => {
    const a = await page.locator(firstSub).first().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    const o = await page.getByTestId("lineup-col-owner").first().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    if (!a || !o) return "pending";
    if (a === "rgba(0, 0, 0, 0)") return "transparent";
    return a === o ? "ok" : `mismatch a=${a} o=${o}`;
  }).toBe("ok");

  // 2. scrolled right, the Voice header scrolls away (not pinned) while the corner stays at x≈0.
  await page.setViewportSize({ width: 480, height: 800 });
  const x0 = await boxX(page, firstSub);
  await page.evaluate(() => window.scrollTo(document.body.scrollWidth, 0));
  await expect.poll(async () => {
    const b = await page.locator(`${grid} .lineup-corner`).boundingBox();
    return b ? b.x : 999;
  }).toBeLessThan(2);
  const x1 = await boxX(page, firstSub);
  expect(x1).toBeLessThan(x0 - 5); // it moved left with the content, it did not stick
});

test("T203-FIX4: sound sheet first channel header keeps the tint and scrolls away while the corner stays", async ({ page }) => {
  const u = `fix4s${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json()).part;
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const guests: string[] = [];
  for (let i = 0; i < 6; i++) guests.push((await (await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Guest ${i}` } })).json()).musician.id);
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Gig" } })).json()).setlist;
  const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song;
  await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [me, ...guests].map((mid) => ({ musicianId: mid, partIds: [lead.id] })) } });
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s.id } });

  await page.goto(`/bands/${id}/setlists/${sl.id}/sheet`);
  await expect(page.getByTestId("sheet-title")).toBeVisible();
  const grid = '[data-testid="sheet-grid"]';
  const firstChan = `${grid} [data-testid="sheet-channel"]`;
  // first channel header tinted like a sibling, not the body cell (polled against the render race — see the grid test)
  await expect.poll(async () => {
    const a = await page.locator(firstChan).nth(0).evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    const b = await page.locator(firstChan).nth(1).evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    const body = await page.locator(`${grid} .sheet-body-cell`).first().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    if (!a || !b) return "pending";
    if (a !== b) return `mismatch a=${a} b=${b}`;
    return a !== body ? "ok" : "same-as-body";
  }).toBe("ok");

  await page.setViewportSize({ width: 480, height: 800 });
  const x0 = await boxX(page, firstChan);
  await page.evaluate(() => window.scrollTo(document.body.scrollWidth, 0));
  await expect.poll(async () => {
    const b = await page.locator(`${grid} .lineup-corner`).boundingBox();
    return b ? b.x : 999;
  }).toBeLessThan(2);
  const x1 = await boxX(page, firstChan);
  expect(x1).toBeLessThan(x0 - 5);
});

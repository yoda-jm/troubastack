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

// T210: each line-up grid scrolls inside its own box (LineupScroll), so these tests scroll the BOX, not the
// window. boxEdge is the box's inner edges: where a stuck header row or song column must sit.
async function boxEdge(page: Page, grid: string) {
  return page.locator(grid).evaluate((g) => {
    const box = g.closest(".lineup-scroll") as HTMLElement;
    const r = box.getBoundingClientRect();
    return { top: r.top + box.clientTop, left: r.left + box.clientLeft };
  });
}
const scrollBox = (page: Page, grid: string, to: "top" | "bottom" | "left" | "right") =>
  page.locator(grid).evaluate((g, where) => {
    const box = g.closest(".lineup-scroll") as HTMLElement;
    if (where === "top") box.scrollTop = 0;
    else if (where === "bottom") box.scrollTop = box.scrollHeight;
    else if (where === "left") box.scrollLeft = 0;
    else box.scrollLeft = box.scrollWidth;
  }, to);

// The "+ part" popup is painted where it says it is, whole: inside the window, and the element at each of its
// inner corners is the popup itself (an overflow box clipping it, or anything covering it, fails this).
async function expectWholePopup(page: Page) {
  const pop = page.getByTestId("lineup-assign");
  await expect(pop).toBeVisible();
  const r = await pop.evaluate((el) => {
    const b = el.getBoundingClientRect();
    const hit = (x: number, y: number) => el.contains(document.elementFromPoint(x, y));
    return {
      inWindow: b.left >= 0 && b.top >= 0 && b.right <= document.documentElement.clientWidth && b.bottom <= window.innerHeight,
      corners: [hit(b.left + 4, b.top + 4), hit(b.right - 4, b.top + 4), hit(b.left + 4, b.bottom - 4), hit(b.right - 4, b.bottom - 4)],
    };
  });
  expect(r).toEqual({ inWindow: true, corners: [true, true, true, true] });
}

// 9 musicians (each a Voice column after "Add common parts") and `songs` songs: wide and tall.
async function seedWide(page: Page, songs: number) {
  await register(page, `sticky${stamp()}`);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  for (let i = 0; i < 8; i++) await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Guest ${i}` } });
  for (let i = 0; i < songs; i++) await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } });
  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await page.getByTestId("lineups-add-common").click(); // seeds voice parts → every musician gets a Voice column
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();
}

const GRID = '[data-testid="lineup-grid"]';

// T203-FIX3, inside T210's box: a header stuck at the box's top and a song column stuck at its left, so a long,
// wide list never loses its headers or its song titles. The "+ part" popup opens whole, at either end.
test("sticky header + song column inside the box, the popup whole at either end", async ({ page }) => {
  await seedWide(page, 45);

  // 1. Scroll the box to the bottom: the header row sits at the box's top.
  await scrollBox(page, GRID, "bottom");
  await expect.poll(async () => {
    const edge = await boxEdge(page, GRID);
    const owner = await page.getByTestId("lineup-col-owner").first().boundingBox();
    return owner ? Math.abs(owner.y - edge.top) : 999;
  }).toBeLessThan(2);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  // 2. Narrow viewport, scroll the box right: the song-title column stays at the box's left...
  await page.setViewportSize({ width: 480, height: 800 });
  await scrollBox(page, GRID, "right");
  await expect.poll(async () => {
    const edge = await boxEdge(page, GRID);
    const cell = await page.getByTestId("lineup-row").first().locator("td").first().boundingBox();
    return cell ? Math.abs(cell.x - edge.left) : 999;
  }).toBeLessThan(2);
  // ...and the page itself is no wider than the window.
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth)).toBe(true);

  // 3. The instruments popup opens below its ⋯ trigger, whole: at the right-hand column (box still scrolled
  //    right). T211: it opens from ⋯ → Edit musician…, placed from the ⋯'s rect (still fixed, never clipped).
  await scrollBox(page, GRID, "top");
  const lastMenu = page.getByTestId("lineup-musician-menu").last();
  await lastMenu.click();
  await page.getByTestId("lineup-musician-edit").click();
  const trig = (await lastMenu.boundingBox())!;
  const pop = (await page.getByTestId("lineup-assign").boundingBox())!;
  expect(pop.y).toBeGreaterThanOrEqual(trig.y - 1); // below (or level with) the trigger
  await expectWholePopup(page);
  await page.getByTestId("lineup-assign-close").click();
  await expect(page.getByTestId("lineup-assign")).toHaveCount(0);

  // ...and at the first column (the box scrolled back left).
  await scrollBox(page, GRID, "left");
  await page.getByTestId("lineup-musician-menu").first().click();
  await page.getByTestId("lineup-musician-edit").click();
  await expectWholePopup(page);
});

// T211 — the Line-ups page fills the window and the PAGE itself never scrolls (this replaces T210's "the box is
// no trap, the wheel goes on to the page": the page no longer scrolls past the box, by design). The box ends on
// screen, so both its scrollbars are reachable, and the wheel scrolls inside the box, never the page.
test("T211: the page never scrolls; the box fills the window and the wheel scrolls inside it", async ({ page }) => {
  await seedWide(page, 45);
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  // The page itself never scrolls, vertically or horizontally, even with 45 tall rows and 9 columns.
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= window.innerHeight)).toBe(true);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  // The box ends on screen (its horizontal scrollbar is reachable), at load.
  const b = (await page.getByTestId("lineup-scroll").boundingBox())!;
  expect(b.y + b.height).toBeLessThanOrEqual(720 + 1);

  // A wheel over the box scrolls the BOX, not the page.
  await scrollBox(page, GRID, "top");
  const boxTop = () => page.locator(GRID).evaluate((g) => (g.closest(".lineup-scroll") as HTMLElement).scrollTop);
  const y0 = await page.evaluate(() => window.scrollY);
  await page.mouse.move(b.x + b.width / 2, b.y + 120);
  await page.mouse.wheel(0, 400);
  await expect.poll(boxTop).toBeGreaterThan(50); // the box scrolled
  expect(await page.evaluate(() => window.scrollY)).toBe(y0); // the page did not move
});

// T210 + M4 — an arrow key onto a cell outside the box's view scrolls the box to it, and never leaves it under the
// stuck header rows or the stuck song column. Checked after EVERY key: focus() alone scrolls only a cell that is
// outside the box, so a middle row going up (or a column going left) stays hidden under them.
test("T210: arrow keys keep the focused cell in the box, clear of the header and song column", async ({ page }) => {
  await seedWide(page, 45);
  await page.setViewportSize({ width: 600, height: 700 });
  const clear = () =>
    page.evaluate(() => {
      const el = document.activeElement as HTMLElement;
      const td = el.closest("td")!.getBoundingClientRect();
      const g = document.querySelector('[data-testid="lineup-grid"]')!;
      const head = g.querySelector("thead")!.getBoundingClientRect();
      const corner = g.querySelector(".lineup-corner")!.getBoundingClientRect();
      const box = g.closest(".lineup-scroll") as HTMLElement;
      const b = box.getBoundingClientRect();
      const bottom = Math.min(b.top + box.clientTop + box.clientHeight, window.innerHeight);
      const right = b.left + box.clientLeft + box.clientWidth;
      const at = `row ${el.dataset.row} col ${el.dataset.col}`;
      if (td.top < head.bottom - 1) return `${at}: under the header (${Math.round(td.top)} < ${Math.round(head.bottom)})`;
      if (td.bottom > bottom + 1) return `${at}: below the box or window (${Math.round(td.bottom)} > ${Math.round(bottom)})`;
      if (td.left < corner.right - 1) return `${at}: under the song column (${Math.round(td.left)} < ${Math.round(corner.right)})`;
      if (td.right > right + 1) return `${at}: right of the box (${Math.round(td.right)} > ${Math.round(right)})`;
      return "clear";
    });
  const hidden: string[] = []; // every key that left the focused cell hidden, so a red run lists them all
  const press = async (key: string, times: number) => {
    for (let i = 0; i < times; i++) {
      await page.keyboard.press(key);
      const c = await clear();
      if (c !== "clear") hidden.push(`${key} #${i + 1}: ${c}`);
    }
  };
  await page.getByTestId("lineup-cell-voice").first().focus();
  const cols = await page.getByTestId("lineup-subcol").count();
  await press("ArrowDown", 44); // to the last row
  await press("ArrowUp", 44); // back to the first: the middle rows pass under the header
  await press("ArrowRight", cols - 1); // to the last column
  await press("ArrowLeft", cols - 1); // back to the first: the middle columns pass under the song column
  expect(hidden).toEqual([]);
});

// T210 — a musician column still drags to a new place inside the box.
test("T210: a musician column drags to a new place inside the box", async ({ page }) => {
  await seedWide(page, 3);
  const owners = page.getByTestId("lineup-col-owner");
  const names = async () => (await owners.allTextContents()).map((t) => t.split(/[◀+⋯]/)[0].trim());
  const before = await names();
  await owners.nth(2).dragTo(owners.nth(0));
  await expect.poll(names).not.toEqual(before);
  expect((await names())[0]).toBe(before[2]);
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
  await expect(page.getByTestId("setlist-detail-title")).toBeVisible(); // T206: shared concert header h1
  await expect(page.locator('[data-testid="sheet-grid"] thead tr')).toHaveCount(3);
  await page.evaluate(() => { (document.documentElement.style as unknown as { zoom: string }).zoom = "1.25"; });
  await scrollBox(page, '[data-testid="sheet-grid"]', "bottom");

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

  const grid = GRID;
  const firstSub = `${grid} thead tr:nth-child(2) th`;
  // 1. the first Voice sub-header carries a HEADER paint — not the body's transparent/surface white (the
  //    regression made this cell white). T211: the owner header is now the musician's colour while the sub
  //    header is the neutral header ground, so compare the sub header against a BODY cell (it must differ, and
  //    never be transparent) rather than against the owner. Polled: add-common seeds the parts asynchronously.
  const bodyCell = `${grid} tbody tr:first-child td`;
  await expect.poll(async () => {
    const a = await page.locator(firstSub).first().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    const body = await page.locator(bodyCell).last().evaluate((el) => getComputedStyle(el).backgroundColor).catch(() => "");
    if (!a || !body) return "pending";
    if (a === "rgba(0, 0, 0, 0)") return "transparent";
    return a !== body ? "ok" : "same-as-body";
  }).toBe("ok");

  // 2. scrolled right, the Voice header scrolls away (not pinned) while the corner stays at the box's left.
  await page.setViewportSize({ width: 480, height: 800 });
  const x0 = await boxX(page, firstSub);
  await scrollBox(page, grid, "right");
  await expect.poll(async () => {
    const edge = await boxEdge(page, grid);
    const b = await page.locator(`${grid} .lineup-corner`).boundingBox();
    return b ? Math.abs(b.x - edge.left) : 999;
  }).toBeLessThan(2);
  const cornerX = await boxX(page, `${grid} .lineup-corner`);
  const x1 = await boxX(page, firstSub);
  // It didn't just move — it scrolled PAST the corner's left edge. FIX6: a stuck-left header would sit AT the
  // corner (x1≈cornerX) and still satisfy x1<x0-5, so that check alone is blind to the bug.
  expect(x1).toBeLessThan(x0 - 5);
  expect(x1).toBeLessThan(cornerX);
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
  await expect(page.getByTestId("setlist-detail-title")).toBeVisible(); // T206: shared concert header h1
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
  await scrollBox(page, grid, "right");
  await expect.poll(async () => {
    const edge = await boxEdge(page, grid);
    const b = await page.locator(`${grid} .lineup-corner`).boundingBox();
    return b ? Math.abs(b.x - edge.left) : 999;
  }).toBeLessThan(2);
  const cornerX = await boxX(page, `${grid} .lineup-corner`);
  const x1 = await boxX(page, firstChan);
  // FIX6: scrolled PAST the corner's left edge, not merely moved (a stuck-left header sits at x1≈cornerX and
  // still passes x1<x0-5).
  expect(x1).toBeLessThan(x0 - 5);
  expect(x1).toBeLessThan(cornerX);
});

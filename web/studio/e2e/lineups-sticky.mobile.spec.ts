import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T210 — the line-up header and song column stay put under a FINGER on a touch engine. Runs in the "mobile"
// project only (isMobile + hasTouch, 800×1280 at DPR 2; playwright.config.ts). On a mobile engine a page wider
// than the screen widens its layout viewport to the content, so a drag only pans the visual viewport and the
// page-sticky header rides away (T203-FIX3 measured −745 px). The desktop spec scrolls with scrollTo and can't
// see it; this one drags with real touch events (CDP Input.dispatchTouchEvent), never scrollTo or the wheel.

const W = 800;

async function seed(page: Page) {
  await register(page, `touch${stamp()}`);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const post = async (path: string, data: object) => (await page.request.post(`/api/bands/${id}/${path}`, { data })).json();
  const voice = (await post("parts", { name: "Lead vocal", kind: "voice", short: "L" })).part.id;
  const guitar = (await post("parts", { name: "Guitar", kind: "instrument", short: "G" })).part.id;
  // 14 musicians × 2 sub-columns (voice + an instrument) and 80 songs: wide, and tall enough that each box has far
  // more than one drag to scroll (40 sheet rows were barely taller than the box).
  const mus: string[] = [(await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id];
  for (let i = 1; i < 14; i++) mus.push((await post("musicians", { name: `Player ${i}` })).musician.id);
  const sl = (await post("setlists", { name: "Gig" })).setlist.id;
  for (let i = 0; i < 80; i++) {
    const s = (await post("songs", { title: `Song ${i}` })).song.id;
    await page.request.patch(`/api/bands/${id}/songs/${s}`, { data: { lineup: mus.map((m) => ({ musicianId: m, partIds: [voice, guitar] })) } });
    await post(`setlists/${sl}/items`, { songId: s });
  }
  return { id, sl };
}

// One finger from (x0,y0) to (x1,y1) in small steps, then a still hold before lifting so no fling runs on.
async function drag(page: Page, x0: number, y0: number, x1: number, y1: number) {
  const cdp = await page.context().newCDPSession(page);
  const steps = 24;
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: x0, y: y0 }] });
  for (let i = 1; i <= steps; i++) {
    const x = x0 + ((x1 - x0) * i) / steps;
    const y = y0 + ((y1 - y0) * i) / steps;
    await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x, y }] });
    await page.waitForTimeout(16);
  }
  await page.waitForTimeout(150);
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await cdp.detach();
}

// Where things SHOW: getBoundingClientRect is in layout-viewport coordinates, so subtract the visual viewport's
// offset. The box is the grid's scroll box when there is one, else the page (what the sabotage leaves).
function read(page: Page, gridSel: string) {
  return page.evaluate((sel) => {
    const g = document.querySelector(sel)!;
    const vv = window.visualViewport!;
    const box = g.closest(".lineup-scroll");
    const boxRect = box ? box.getBoundingClientRect() : { top: 0, left: 0 };
    const head = g.querySelector("thead")!.getBoundingClientRect();
    const song = g.querySelector("tbody .lineup-songcell")!.getBoundingClientRect();
    return {
      innerWidth: window.innerWidth,
      boxTop: boxRect.top - vv.offsetTop + (box ? box.clientTop : 0),
      boxLeft: boxRect.left - vv.offsetLeft + (box ? box.clientLeft : 0),
      headTop: head.top - vv.offsetTop,
      songLeft: song.left - vv.offsetLeft,
      scrolledY: (box ? box.scrollTop : 0) + vv.pageTop,
      scrolledX: (box ? box.scrollLeft : 0) + vv.pageLeft,
      vvHeight: vv.height,
      topbarBottom: (document.querySelector("header.topbar")?.getBoundingClientRect().bottom ?? 0) - vv.offsetTop,
    };
  }, gridSel);
}

async function dragAndCheck(page: Page, gridSel: string) {
  // The grid fills the screen below the controls; start the finger well inside it.
  const g = (await page.locator(gridSel).boundingBox())!;
  const y = Math.min(g.y + g.height, 1280) - 120;
  const before = await read(page, gridSel);
  // Soft: a red run reports every check, so the sabotage shows where the header and the song column went.
  expect.soft(before.innerWidth, "the page itself must not widen to the table").toBe(W);

  // A vertical drag: up the screen, so the content scrolls down.
  await drag(page, W / 2, y, W / 2, y - 500);
  await expect.poll(async () => (await read(page, gridSel)).scrolledY, { message: "the drag scrolled something" }).toBeGreaterThan(before.scrolledY + 100);
  const v = await read(page, gridSel);
  expect.soft(v.innerWidth).toBe(W);
  expect.soft(v.headTop, "the header is inside the visible area").toBeGreaterThanOrEqual(-2);
  expect.soft(v.headTop).toBeLessThan(v.vvHeight);
  expect.soft(Math.abs(v.headTop - v.boxTop), `header at ${v.headTop}, box top at ${v.boxTop}`).toBeLessThanOrEqual(2);

  // A horizontal drag: leftwards, so the content scrolls right.
  await drag(page, W - 80, y - 200, 80, y - 200);
  await expect.poll(async () => (await read(page, gridSel)).scrolledX, { message: "the drag scrolled sideways" }).toBeGreaterThan(before.scrolledX + 100);
  const h = await read(page, gridSel);
  expect.soft(h.innerWidth).toBe(W);
  expect.soft(Math.abs(h.songLeft - h.boxLeft), `song cell at ${h.songLeft}, box left at ${h.boxLeft}`).toBeLessThanOrEqual(2);

  // Drag on until nothing moves: the box reaches its end. T211 — the Line-ups page itself never scrolls or pans,
  // so there is nothing past the box; the header stays at the box's top, which is below the topbar (and the
  // toolbar). The sound sheet keeps T210's bounded box, where the page can still scroll its bottom padding past.
  // Either way the header stays on screen, under the topbar.
  let last = -1;
  for (let i = 0; i < 12; i++) {
    await drag(page, W / 2, 1100, W / 2, 400);
    const now = await read(page, gridSel);
    const pageY = await page.evaluate(() => window.scrollY + window.visualViewport!.pageTop);
    if (now.scrolledY + pageY === last) break;
    last = now.scrolledY + pageY;
  }
  const end = await read(page, gridSel);
  expect.soft(end.headTop, `at the very end: header at ${end.headTop}, topbar ends at ${end.topbarBottom}`).toBeGreaterThanOrEqual(end.topbarBottom - 2);
}

test("T210: a finger drag keeps the Line-ups header and song column in place", async ({ page }) => {
  const { id } = await seed(page);
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineup-col-owner")).toHaveCount(14);
  await dragAndCheck(page, '[data-testid="lineup-grid"]');
});

test("T210: a finger drag keeps the Sound sheet header and song column in place", async ({ page }) => {
  const { id, sl } = await seed(page);
  await page.goto(`/bands/${id}/setlists/${sl}/sheet`);
  await expect(page.getByTestId("sheet-owner")).toHaveCount(14);
  await expect(page.getByTestId("sheet-row")).toHaveCount(80);
  await dragAndCheck(page, '[data-testid="sheet-grid"]');
});

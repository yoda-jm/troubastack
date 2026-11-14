import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T211 stage 1 — the Line-ups page fills the window: the page never scrolls, the grid box ends on screen so both
// its scrollbars are reachable, empty cells are blank (no "·"), more songs show at load than before, and when the
// grid overflows its box the edge shadows + "Jump to" appear. Placeholder names/titles only (public repo).

const GRID = '[data-testid="lineup-grid"]';
const LONG_TITLE = "A Long Song Title That Runs Well Past Forty-Six Characters Indeed"; // ≥ 46 chars

// Seed a band of a given shape over the API. Editor mode gives every VISIBLE musician a Voice channel (the band
// has voice parts) plus one channel per PERSISTED instrument, so the sub-column count is deterministic:
//   subcols = (1 + guests) + instruments.
// The measured shape is guests:3, instruments:11 → 4 visible musicians, 15 sub-columns, plus 1 hidden.
async function seedShape(
  page: Page,
  opts: { guests: number; instruments: number; songs: number; firstTitle?: string; hidden?: boolean },
): Promise<{ id: string }> {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const post = async (path: string, data: object) => (await page.request.post(`/api/bands/${id}/${path}`, { data })).json();
  const mkPart = async (name: string, kind: string, short: string) => (await post("parts", { name, kind, short })).part;
  await mkPart("Lead vocal", "voice", "LV");
  await mkPart("Backing vocal", "voice", "BV");
  const inst = [];
  for (let i = 0; i < opts.instruments; i++) inst.push(await mkPart(`Instrument ${i}`, "instrument", `I${i}`));

  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const vis = [me];
  for (let i = 0; i < opts.guests; i++) vis.push((await post("musicians", { name: `Player ${i}` })).musician.id);
  if (opts.hidden ?? true) {
    const h = (await post("musicians", { name: "Hidden One" })).musician.id;
    await page.request.patch(`/api/bands/${id}/musicians/${h}`, { data: { hidden: true } });
  }
  // Persist each instrument to one visible musician (round-robin) → a stable column per instrument.
  const ownerOf = (i: number) => vis[i % vis.length];
  for (let i = 0; i < inst.length; i++) await page.request.post(`/api/bands/${id}/musicians/${ownerOf(i)}/instruments`, { data: { partId: inst[i].id } });

  // 48 songs, 2–3 players a song, some playing a voice part and/or one of their instruments; one long title.
  for (let s = 0; s < opts.songs; s++) {
    const title = s === 0 ? (opts.firstTitle ?? LONG_TITLE) : `Song ${s}`;
    const song = (await post("songs", { title })).song;
    const n = 2 + (s % 2); // 2 or 3 players
    const lineup: { musicianId: string; partIds: string[] }[] = [];
    for (let k = 0; k < n; k++) {
      const vi = (s + k) % vis.length;
      const mid = vis[vi];
      const partIds: string[] = [];
      if (k === 0) partIds.push((await (await page.request.get(`/api/bands/${id}/parts`)).json()).parts.find((p: { kind: string }) => p.kind === "voice").id);
      const mine = inst.filter((_, ii) => ii % vis.length === vi);
      if (mine.length) partIds.push(mine[s % mine.length].id);
      if (partIds.length) lineup.push({ musicianId: mid, partIds });
    }
    if (lineup.length) await page.request.patch(`/api/bands/${id}/songs/${song.id}`, { data: { lineup } });
  }
  return { id };
}

const scrollBox = (page: Page, to: "top" | "bottom" | "left" | "right") =>
  page.locator(GRID).evaluate((g, where) => {
    const box = g.closest(".lineup-scroll") as HTMLElement;
    if (where === "top") box.scrollTop = 0;
    else if (where === "bottom") box.scrollTop = box.scrollHeight;
    else if (where === "left") box.scrollLeft = 0;
    else box.scrollLeft = box.scrollWidth;
  }, to);

const boxFits = (page: Page) =>
  page.locator(GRID).evaluate((g) => { const b = g.closest(".lineup-scroll") as HTMLElement; return b.scrollWidth <= b.clientWidth; });
const boxOverflows = (page: Page) =>
  page.locator(GRID).evaluate((g) => { const b = g.closest(".lineup-scroll") as HTMLElement; return b.scrollWidth > b.clientWidth + 1; });
// How many body rows fit in the box's visible area at load (header excluded).
const rowsVisible = (page: Page) =>
  page.locator(GRID).evaluate((g) => {
    const box = g.closest(".lineup-scroll") as HTMLElement;
    const head = g.querySelector("thead")!.getBoundingClientRect().height;
    const row = g.querySelector("tbody tr")!.getBoundingClientRect().height;
    return Math.floor((box.clientHeight - head) / row);
  });
const cornerWidth = (page: Page) => page.locator(`${GRID} .lineup-corner`).evaluate((e) => e.getBoundingClientRect().width);

test("the page fills the window and never scrolls; the box ends on screen (measured shape)", async ({ page }) => {
  await register(page, `frame${stamp()}`);
  const { id } = await seedShape(page, { guests: 3, instruments: 11, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  for (const [w, h] of [[1280, 720], [1440, 900], [1920, 1080]] as const) {
    await page.setViewportSize({ width: w, height: h });
    await expect.poll(() => page.evaluate(() => document.documentElement.scrollHeight <= window.innerHeight)).toBe(true);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    const b = (await page.getByTestId("lineup-scroll").boundingBox())!;
    expect(b.y + b.height, `box bottom at ${w}×${h}`).toBeLessThanOrEqual(h + 1);
  }
});

test("the measured shape fits without sideways scroll (1440, 1920), shows more rows, and hides Jump to", async ({ page }) => {
  await register(page, `fits${stamp()}`);
  const { id } = await seedShape(page, { guests: 3, instruments: 11, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  await page.setViewportSize({ width: 1440, height: 900 });
  await expect.poll(() => boxFits(page)).toBe(true); // SABOTAGE: putting --maxw back on the route reds this
  expect(await rowsVisible(page), "rows at 1440×900 (today 12)").toBeGreaterThanOrEqual(20);
  await expect(page.getByTestId("lineups-jump")).toHaveCount(0); // SABOTAGE: showing Jump to unconditionally reds this

  await page.setViewportSize({ width: 1920, height: 1080 });
  await expect.poll(() => boxFits(page)).toBe(true);

  await page.setViewportSize({ width: 1280, height: 720 });
  expect(await rowsVisible(page), "rows at 1280×720 (today 7)").toBeGreaterThanOrEqual(15);
});

test("a single long title never sets the song column width (46- vs 10-char)", async ({ page }) => {
  await register(page, `songcol${stamp()}`);
  await page.setViewportSize({ width: 1440, height: 900 });
  const long = await seedShape(page, { guests: 3, instruments: 3, songs: 3, firstTitle: LONG_TITLE });
  await page.goto(`/bands/${long.id}/lineups`);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();
  const wLong = await cornerWidth(page);

  await page.goto("/bands"); // createBandAndOpen (inside seedShape) drives the bands list
  const short = await seedShape(page, { guests: 3, instruments: 3, songs: 3, firstTitle: "Short" });
  await page.goto(`/bands/${short.id}/lineups`);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();
  const wShort = await cornerWidth(page);

  expect(Math.abs(wLong - wShort), `long=${wLong} short=${wShort}`).toBeLessThanOrEqual(2);
});

test("empty cells are blank — no '·' in the grid", async ({ page }) => {
  await register(page, `empty${stamp()}`);
  const { id } = await seedShape(page, { guests: 3, instruments: 11, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();
  const texts = await page.locator(`${GRID} .lineup-cell`).allTextContents();
  expect(texts.some((t) => t.trim() === "·"), "no cell shows '·'").toBe(false); // SABOTAGE: rendering "·" reds this
  expect(texts.some((t) => t.trim() !== ""), "positive control: some cells are filled").toBe(true);
});

test("wide band: sideways scroll, right shadow, Jump to visible, and Jump to the last musician", async ({ page }) => {
  await register(page, `wide${stamp()}`);
  const { id } = await seedShape(page, { guests: 6, instruments: 18, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  await expect.poll(() => boxOverflows(page)).toBe(true);
  await expect(page.getByTestId("lineup-edge-r")).toHaveClass(/on/); // right shadow on while columns remain
  await expect(page.getByTestId("lineups-jump")).toBeVisible();

  const jumps = page.getByTestId("lineups-jump-item");
  // Jump to a musician aligns its header's left edge to the song column's right edge (±2 px). Checked on the
  // SECOND musician, which has a screen of columns to its right so the box can reach that scroll position.
  await jumps.nth(1).click();
  await expect.poll(() =>
    page.locator(GRID).evaluate((g) => {
      const corner = g.querySelector(".lineup-corner") as HTMLElement;
      const second = g.querySelectorAll('[data-testid="lineup-col-owner"][data-mus-id]')[1] as HTMLElement;
      return Math.abs(second.getBoundingClientRect().left - corner.getBoundingClientRect().right);
    }),
  ).toBeLessThanOrEqual(2);

  // Jump to the LAST musician scrolls the box as far toward it as it can. It is the trailing column, so there is
  // not a screen of grid to its right to pull it all the way to the song column; the box reaches its maximum
  // scroll with that musician fully in view at the right (its left edge at the song column is reached only when
  // geometry allows). The jumpTo target math is the same — proven aligned on the second musician above.
  await jumps.nth((await jumps.count()) - 1).click();
  await expect.poll(() =>
    page.locator(GRID).evaluate((g) => {
      const box = g.closest(".lineup-scroll") as HTMLElement;
      const corner = g.querySelector(".lineup-corner") as HTMLElement;
      const last = [...g.querySelectorAll('[data-testid="lineup-col-owner"][data-mus-id]')].pop() as HTMLElement;
      const songRight = corner.getBoundingClientRect().right;
      const lastRect = last.getBoundingClientRect();
      const atSong = Math.abs(lastRect.left - songRight) <= 2;
      const maxed = Math.abs(box.scrollLeft - (box.scrollWidth - box.clientWidth)) <= 2;
      const visible = lastRect.right <= box.getBoundingClientRect().right + 2 && lastRect.left >= songRight - 2;
      return atSong || (maxed && visible) ? "ok" : `left=${Math.round(lastRect.left)} songRight=${Math.round(songRight)} maxed=${maxed}`;
    }),
  ).toBe("ok");
});

test("edge shadows track the scroll", async ({ page }) => {
  await register(page, `edges${stamp()}`);
  const { id } = await seedShape(page, { guests: 6, instruments: 18, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await page.setViewportSize({ width: 1440, height: 900 });
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();
  const L = page.getByTestId("lineup-edge-l"), R = page.getByTestId("lineup-edge-r"), T = page.getByTestId("lineup-edge-t");

  await scrollBox(page, "left");
  await expect(L).not.toHaveClass(/on/); // nothing to the left yet
  await expect(R).toHaveClass(/on/);

  await scrollBox(page, "right");
  await expect(L).toHaveClass(/on/); // SABOTAGE: dropping the scrollLeft>0 condition keeps L off here → reds
  await expect(R).not.toHaveClass(/on/); // at the far right, nothing more to the right

  await scrollBox(page, "top");
  await expect(T).not.toHaveClass(/on/);
  await scrollBox(page, "bottom");
  await expect(T).toHaveClass(/on/);
});

test("the page does not scroll when the filter is focused and typed (soft-keyboard proxy; 1d checks the app)", async ({ page }) => {
  await register(page, `kbd${stamp()}`);
  const { id } = await seedShape(page, { guests: 3, instruments: 11, songs: 48 });
  await page.goto(`/bands/${id}/lineups`);
  await page.setViewportSize({ width: 1280, height: 720 });
  await expect(page.getByTestId("lineup-col-owner").first()).toBeVisible();

  const filter = page.getByTestId("lineups-filter");
  await filter.click();
  await filter.fill("Player");
  // The filter and its suggestions are at the top and stay visible; the page itself does not scroll.
  await expect(filter).toBeVisible();
  await expect(page.getByTestId("lineups-suggest")).toBeVisible();
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= window.innerHeight)).toBe(true);
});

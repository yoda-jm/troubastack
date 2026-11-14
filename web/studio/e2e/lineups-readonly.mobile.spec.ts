import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T211 — on a touch engine, a tap and a drag across the cells in read-only change nothing (the cells are plain
// text, not buttons). Runs in the `mobile` project (isMobile + hasTouch). Placeholder names only.

async function seed(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "LV" } })).json()).part;
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  // A few songs so there are several voice cells to tap and drag across.
  const songs: string[] = [];
  for (let i = 0; i < 6; i++) songs.push((await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: `Song ${i}` } })).json()).song.id);
  return { id, me, songs, lead: lead.id as string };
}

async function storedCount(page: Page, id: string): Promise<number> {
  const songs = (await (await page.request.get(`/api/bands/${id}/songs`)).json()).songs as { lineup?: { partIds: string[] }[] }[];
  return songs.reduce((n, s) => n + (s.lineup ?? []).reduce((m, e) => m + e.partIds.length, 0), 0);
}

test("read-only on touch: a tap and a drag across cells change nothing", async ({ page }) => {
  await register(page, `rotouch${stamp()}`);
  const { id } = await seed(page);
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await expect(page.getByTestId("lineups-edit-toggle")).toHaveText("Edit");
  expect(await storedCount(page, id)).toBe(0);

  // A tap on a voice cell changes nothing.
  const cell = page.getByTestId("lineup-cell-voice").first();
  await expect(cell).toBeVisible();
  const b = (await cell.boundingBox())!;
  const cdp = await page.context().newCDPSession(page);
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: b.x + b.width / 2, y: b.y + b.height / 2 }] });
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await page.waitForTimeout(200);

  // A drag down across several cells changes nothing (it only scrolls, if anything).
  const x = b.x + b.width / 2;
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x, y: b.y + b.height / 2 }] });
  for (let i = 1; i <= 10; i++) await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x, y: b.y + b.height / 2 + i * 20 }] });
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  await cdp.detach();
  await page.waitForTimeout(250);

  expect(await storedCount(page, id)).toBe(0); // nothing was written
});

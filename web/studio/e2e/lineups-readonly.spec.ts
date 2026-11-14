import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp, enableEdit } from "./setup-helpers";

// T211 — the Line-ups page opens READ-ONLY on every visit (VLL: "ro by default … to avoid modification while
// scrolling"); an Edit/Done toggle turns editing on. In read-only the cells are plain text (a click changes
// nothing), and ✎ / ⋯ are hidden. Also: the /lineups/ trailing-slash route must still fill the window.

async function seed(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "LV" } });
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const song = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song;
  return { id, me, song: song.id as string };
}

// The stored part ids for (song, musician), read from the API — the actual persisted state.
async function storedParts(page: Page, id: string, song: string, mid: string): Promise<string[]> {
  const songs = (await (await page.request.get(`/api/bands/${id}/songs`)).json()).songs;
  const lu = (songs.find((s: { id: string }) => s.id === song)?.lineup ?? []) as { musicianId: string; partIds: string[] }[];
  return lu.find((e) => e.musicianId === mid)?.partIds ?? [];
}

test("read-only by default: the toggle reads Edit, a cell click changes nothing; Edit then it does", async ({ page }) => {
  await register(page, `ro${stamp()}`);
  const { id, me, song } = await seed(page);
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();

  // On load: the toggle reads Edit, not pressed. SABOTAGE (start in edit mode) reds this.
  const toggle = page.getByTestId("lineups-edit-toggle");
  await expect(toggle).toHaveText("Edit");
  await expect(toggle).toHaveAttribute("aria-pressed", "false");

  // ✎ and ⋯ are absent in read-only.
  await expect(page.getByTestId("lineup-song-edit")).toHaveCount(0);
  await expect(page.getByTestId("lineup-musician-menu")).toHaveCount(0);

  // The voice cell is a plain SPAN, not a button (not focusable, no cell interaction).
  const cell = page.getByTestId("lineup-cell-voice").first();
  await expect(cell).toBeVisible();
  expect(await cell.evaluate((el) => el.tagName)).toBe("SPAN");

  // A click on it leaves the stored line-up unchanged.
  await cell.click({ force: true });
  await page.waitForTimeout(250);
  expect(await storedParts(page, id, song, me)).toEqual([]);

  // Positive control: after Edit, the toggle reads Done, ✎/⋯ appear, and the SAME click sets the voice part.
  await enableEdit(page);
  await expect(toggle).toHaveText("Done");
  await expect(page.getByTestId("lineup-song-edit").first()).toBeVisible();
  await expect(page.getByTestId("lineup-musician-menu").first()).toBeVisible();
  await page.getByTestId("lineup-cell-voice").first().click();
  await expect.poll(() => storedParts(page, id, song, me)).not.toEqual([]);
});

test("read-only survives a reload", async ({ page }) => {
  await register(page, `ror${stamp()}`);
  const { id } = await seed(page);
  await page.goto(`/bands/${id}/lineups`);
  await enableEdit(page);
  await expect(page.getByTestId("lineups-edit-toggle")).toHaveText("Done");
  await page.reload();
  // A reload is read-only again (the mode is not remembered).
  await expect(page.getByTestId("lineups-edit-toggle")).toHaveText("Edit");
  await expect(page.getByTestId("lineups-edit-toggle")).toHaveAttribute("aria-pressed", "false");
});

test("the /lineups/ trailing-slash route still fills the window (box > 0, page doesn't scroll)", async ({ page }) => {
  await register(page, `slash${stamp()}`);
  const { id } = await seed(page);
  await page.goto(`/bands/${id}/lineups/`); // trailing slash — React Router still routes to Lineups
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  // The grid box fills the frame (not 0 px). SABOTAGE (endsWith("/lineups") back in Shell) reds this.
  const box = (await page.getByTestId("lineup-scroll").boundingBox())!;
  expect(box.height, "grid box height on /lineups/").toBeGreaterThan(100);
  expect(await page.evaluate(() => document.documentElement.scrollHeight <= window.innerHeight)).toBe(true);
});

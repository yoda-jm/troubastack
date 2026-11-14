import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp, enableEdit } from "./setup-helpers";

// T211 stage 2 — the ✎ song-detail dialog. ✎ (edit mode) opens a <dialog> with one row per visible musician:
// a voice chip ‹ value › (arrows step; the middle opens a list) and the musician's instrument switches. Every
// change writes through the same path as the grid, so the grid behind updates at once.

async function seed(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const mk = async (name: string, kind: string, short: string) =>
    (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name, kind, short } })).json()).part;
  await mk("Lead vocal", "voice", "L");
  await mk("Backing vocal", "voice", "B");
  const gtr = await mk("Guitar", "instrument", "G");
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  await page.request.post(`/api/bands/${id}/musicians/${me}/instruments`, { data: { partId: gtr.id } });
  const songs: string[] = [];
  for (const t of ["Song A", "Song B", "Song C"]) songs.push((await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: t } })).json()).song.id);
  return { id, me, gtr: gtr.id as string, songs };
}

async function storedParts(page: Page, id: string, song: string, mid: string): Promise<string[]> {
  const list = (await (await page.request.get(`/api/bands/${id}/songs`)).json()).songs;
  const lu = (list.find((s: { id: string }) => s.id === song)?.lineup ?? []) as { musicianId: string; partIds: string[] }[];
  return lu.find((e) => e.musicianId === mid)?.partIds ?? [];
}

async function open(page: Page): Promise<{ id: string; me: string; gtr: string; songs: string[] }> {
  const s = await seed(page);
  await page.goto(`/bands/${s.id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  return s;
}

test("✎ opens the right song; the right arrow sets the voice to Lead and the grid cell reads L", async ({ page }) => {
  await register(page, `dlg${stamp()}`);
  const { id, me, songs } = await open(page);

  await page.getByTestId("lineup-song-edit").first().click();
  await expect(page.getByTestId("song-dialog")).toBeVisible();
  await expect(page.getByTestId("song-dialog-pos")).toHaveText("Song 1 of 3");
  await expect(page.getByTestId("song-dialog-title")).toHaveText("Song A");
  await expect(page.getByTestId("song-dialog-voice-value")).toHaveText("None");

  // The right arrow steps None → Lead. SABOTAGE (swap the arrows) sends it the other way → reds this.
  await page.getByTestId("song-dialog-voice-next").click();
  await expect(page.getByTestId("song-dialog-voice-value")).toHaveText("Lead");
  await expect.poll(() => storedParts(page, id, songs[0], me)).toHaveLength(1);

  // Close; the grid's voice cell now reads L.
  await page.getByTestId("song-dialog-close").click();
  await expect(page.getByTestId("song-dialog")).toBeHidden();
  await expect(page.getByTestId("lineup-cell-voice").first()).toHaveText("L");
});

test("the middle opens the list; choosing Both makes the grid cell L+B", async ({ page }) => {
  await register(page, `dlgm${stamp()}`);
  await open(page);
  await page.getByTestId("lineup-song-edit").first().click();

  // The list opens from the middle and is a CHILD of the dialog, so its options receive clicks. SABOTAGE
  // (mount it outside the modal dialog) makes it inert → this reds.
  await page.getByTestId("song-dialog-voice-value").click();
  await expect(page.getByTestId("song-dialog-voice-list")).toBeVisible();
  await page.getByTestId("song-dialog-voice-option").filter({ hasText: "Both" }).click();
  await expect(page.getByTestId("song-dialog-voice-value")).toHaveText("Both");
  // 1d: after choosing, focus returns to that chip's middle button (it falls to <body> without the fix).
  await expect(page.getByTestId("song-dialog-voice-value")).toBeFocused();
  await page.getByTestId("song-dialog-close").click();
  await expect(page.getByTestId("lineup-cell-voice").first()).toHaveText("L+B");
});

test("an instrument switch round-trips through GET /songs", async ({ page }) => {
  await register(page, `dlgi${stamp()}`);
  const { id, me, gtr, songs } = await open(page);
  await page.getByTestId("lineup-song-edit").first().click();

  const tog = page.getByTestId("song-dialog-instrument").filter({ hasText: "Guitar" });
  await expect(tog).toHaveAttribute("aria-pressed", "false");
  await tog.click();
  await expect(tog).toHaveAttribute("aria-pressed", "true");
  await expect.poll(() => storedParts(page, id, songs[0], me)).toContain(gtr);
  await tog.click();
  await expect.poll(() => storedParts(page, id, songs[0], me)).not.toContain(gtr);
});

test("Next lands on the next filtered song and resets the body's scroll", async ({ page }) => {
  await register(page, `dlgn${stamp()}`);
  // Many musicians so the dialog body scrolls — then Next must reset it to the top (1d).
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } });
  for (let i = 0; i < 8; i++) await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Player ${i}` } });
  for (const t of ["Song A", "Song B"]) await page.request.post(`/api/bands/${id}/songs`, { data: { title: t } });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  await page.getByTestId("lineup-song-edit").first().click();
  await expect(page.getByTestId("song-dialog-pos")).toHaveText("Song 1 of 2");

  // Scroll the body down, confirm it moved, then Next resets it to 0.
  const bodyScroll = () => page.locator(".lineup-dlg-body").evaluate((el) => el.scrollTop);
  await page.locator(".lineup-dlg-body").evaluate((el) => { el.scrollTop = el.scrollHeight; });
  expect(await bodyScroll()).toBeGreaterThan(0);
  await page.getByTestId("song-dialog-next").click();
  await expect(page.getByTestId("song-dialog-pos")).toHaveText("Song 2 of 2");
  await expect(page.getByTestId("song-dialog-title")).toHaveText("Song B");
  expect(await bodyScroll(), "body scroll reset on song change").toBe(0); // SABOTAGE (drop the reset) reds this
});

test("Escape closes the dialog and returns focus to the ✎ that opened it", async ({ page }) => {
  await register(page, `dlge${stamp()}`);
  await open(page);
  const pencil = page.getByTestId("lineup-song-edit").first();
  await pencil.click();
  await expect(page.getByTestId("song-dialog")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("song-dialog")).toBeHidden();
  await expect(pencil).toBeFocused();
});

// Discriminating check (Fable): with one musician, nothing pins that a row writes to its OWN musician — a
// sabotage that sends every row to musicianColumns[0] stays green. Seed a SECOND musician with their own
// instrument; edit the second's row; the second gets both parts AND the first is left untouched.
test("each dialog row writes to its own musician (the second musician's edits leave the first unchanged)", async ({ page }) => {
  await register(page, `dlg2${stamp()}`);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const mk = async (name: string, kind: string, short: string) =>
    (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name, kind, short } })).json()).part;
  const lead = await mk("Lead vocal", "voice", "L");
  await mk("Backing vocal", "voice", "B");
  const gtr = await mk("Guitar", "instrument", "G");
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const guest = (await (await page.request.post(`/api/bands/${id}/musicians`, { data: { name: "Zed" } })).json()).musician.id;
  await page.request.post(`/api/bands/${id}/musicians/${guest}/instruments`, { data: { partId: gtr.id } }); // the guest's OWN instrument
  const song = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song.id;
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  await page.getByTestId("lineup-song-edit").first().click();

  // Edit the SECOND musician's row: voice None → Lead, and switch their Guitar on.
  const guestRow = page.locator(`[data-testid="song-dialog-musician"][data-mus-id="${guest}"]`);
  await guestRow.getByTestId("song-dialog-voice-next").click();
  await guestRow.getByTestId("song-dialog-instrument").filter({ hasText: "Guitar" }).click();

  // The guest's entry has BOTH parts (SABOTAGE: every row → musicianColumns[0] leaves the guest empty → reds here)...
  await expect.poll(async () => (await storedParts(page, id, song, guest)).length).toBe(2);
  expect(await storedParts(page, id, song, guest)).toEqual(expect.arrayContaining([lead.id, gtr.id]));
  // ...and the FIRST musician's entry is unchanged (SABOTAGE sends the guest's edits here → reds).
  expect(await storedParts(page, id, song, me)).toEqual([]);
});

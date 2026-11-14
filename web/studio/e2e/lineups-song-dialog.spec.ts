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

test("Next lands on the next filtered song", async ({ page }) => {
  await register(page, `dlgn${stamp()}`);
  await open(page);
  await page.getByTestId("lineup-song-edit").first().click();
  await expect(page.getByTestId("song-dialog-pos")).toHaveText("Song 1 of 3");
  await page.getByTestId("song-dialog-next").click();
  // SABOTAGE (write to the wrong song index) would still move here, but the voice edits would hit the wrong song;
  // this check pins the navigation itself.
  await expect(page.getByTestId("song-dialog-pos")).toHaveText("Song 2 of 3");
  await expect(page.getByTestId("song-dialog-title")).toHaveText("Song B");
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

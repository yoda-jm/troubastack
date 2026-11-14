import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp, lineupMusicianAction, enableEdit } from "./setup-helpers";

// T210 part B, Studio half — hiding a player, "for everybody, one click to bring them back", in two scopes:
// band-wide (the Line-ups header "hide") and per-concert ("not here" on a sound-sheet owner). The filter itself
// is pinned by the shared projection vectors; these specs prove the Studio wiring reaches every surface the
// spec names (grid, sheet, CSV) and that the ↺ bars restore, with a second concert left alone.
//
// Seed: two amplified owners — the admin on lead vocal (mic) and a guest "Zed" on guitar (DI) — one song with
// both, and TWO concerts holding that song. Placeholder names only (public repo).
async function seed(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json()).part;
  const gtr = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Guitar", kind: "instrument", short: "Gtr" } })).json()).part;
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const guest = (await (await page.request.post(`/api/bands/${id}/musicians`, { data: { name: "Zed" } })).json()).musician.id;
  const song = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song;
  await page.request.patch(`/api/bands/${id}/songs/${song.id}`, {
    data: { lineup: [{ musicianId: me, partIds: [lead.id] }, { musicianId: guest, partIds: [gtr.id] }] },
  });
  const mk = async (name: string) => {
    const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name } })).json()).setlist;
    await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: song.id } });
    return sl.id as string;
  };
  const sl1 = await mk("Gig One");
  const sl2 = await mk("Gig Two");
  return { id, me, guest, sl1, sl2 };
}

const gridOwner = (page: Page, name: string) => page.getByTestId("lineup-col-owner").filter({ hasText: name });
const sheetOwner = (page: Page, name: string) => page.getByTestId("sheet-owner").filter({ hasText: name });
const csvText = (page: Page, id: string, sl: string) =>
  page.request.get(`/api/bands/${id}/setlists/${sl}/sheet.csv?order=stage`).then((r) => r.text());

test("T210 band scope: hide from Line-ups — gone on the grid, the sheet and the CSV; one ↺ restores everywhere", async ({ page }) => {
  await register(page, `hide${stamp()}`);
  const { id, sl1 } = await seed(page);

  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  await expect(gridOwner(page, "Zed")).toHaveCount(1);
  await expect(page.getByTestId("lineups-hidden-bar")).toHaveCount(0); // nobody hidden yet

  // Hide the guest from his own column header.
  await lineupMusicianAction(page, "hide", gridOwner(page, "Zed")); // T211: hide moved into the ⋯ menu
  await expect(gridOwner(page, "Zed")).toHaveCount(0); // column gone on the grid
  await expect(page.getByTestId("lineups-hidden-bar")).toContainText("Zed"); // the bar names him

  // Gone on the concert's sound sheet, and the band-scope restore bar explains why.
  await page.goto(`/bands/${id}/setlists/${sl1}/sheet`);
  await expect(page.getByTestId("sheet-grid")).toBeVisible();
  await expect(sheetOwner(page, "Zed")).toHaveCount(0);
  await expect(page.getByTestId("sheet-bandhidden-bar")).toContainText("Zed");

  // Gone in the CSV too.
  expect(await csvText(page, id, sl1)).not.toContain("Zed");

  // One ↺ (from the sheet's band-hidden bar) brings him back everywhere.
  await page.getByTestId("sheet-bandhidden-restore").click();
  await expect(sheetOwner(page, "Zed")).toHaveCount(1);
  expect(await csvText(page, id, sl1)).toContain("Zed"); // positive control: "Zed" IS the CSV token we looked for
  await expect(page.getByTestId("sheet-bandhidden-bar")).toHaveCount(0);
});

test("T210 concert scope: 'not here' drops a player for that sheet only; a second concert is untouched; ↺ restores", async ({ page }) => {
  await register(page, `absent${stamp()}`);
  const { id, sl1, sl2 } = await seed(page);

  await page.goto(`/bands/${id}/setlists/${sl1}/sheet`);
  await expect(page.getByTestId("sheet-grid")).toBeVisible();
  await expect(sheetOwner(page, "Zed")).toHaveCount(1);

  // Mark him "not at this concert" from his owner header.
  await sheetOwner(page, "Zed").getByTestId("sheet-owner-absent").click();
  await expect(sheetOwner(page, "Zed")).toHaveCount(0); // gone on THIS sheet
  await expect(page.getByTestId("sheet-absent-bar")).toContainText("Zed");
  expect(await csvText(page, id, sl1)).not.toContain("Zed");

  // The band Line-ups grid is unaffected — this scope is the concert only.
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  await expect(gridOwner(page, "Zed")).toHaveCount(1);
  await expect(page.getByTestId("lineups-hidden-bar")).toHaveCount(0);

  // The second concert, holding the same song, still shows him.
  await page.goto(`/bands/${id}/setlists/${sl2}/sheet`);
  await expect(page.getByTestId("sheet-grid")).toBeVisible();
  await expect(sheetOwner(page, "Zed")).toHaveCount(1);
  await expect(page.getByTestId("sheet-absent-bar")).toHaveCount(0);
  expect(await csvText(page, id, sl2)).toContain("Zed"); // positive control

  // ↺ on the first concert restores him there.
  await page.goto(`/bands/${id}/setlists/${sl1}/sheet`);
  await expect(page.getByTestId("sheet-grid")).toBeVisible();
  await page.getByTestId("sheet-absent-restore").click();
  await expect(sheetOwner(page, "Zed")).toHaveCount(1);
  await expect(page.getByTestId("sheet-absent-bar")).toHaveCount(0);
});

test("T210 binding 2: a hidden player drops out of search suggestions, but a chip added earlier still filters", async ({ page }) => {
  await register(page, `sug${stamp()}`);
  const { id } = await seed(page);

  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  const filter = page.getByTestId("lineups-filter");
  const zedSuggest = page.getByTestId("lineups-suggest-item").filter({ hasText: "Zed" });

  // Before hiding: typing the name suggests him (the musician chip and the "Zed · Guitar" pair). Add the first
  // — the chip reads the stored line-ups, not this view.
  await filter.fill("Zed");
  await expect.poll(() => zedSuggest.count()).toBeGreaterThan(0);
  await zedSuggest.first().click();
  await expect(page.getByTestId("lineup-chip").filter({ hasText: "Zed" })).toHaveCount(1);
  await expect(page.getByTestId("lineup-row")).toHaveCount(1); // the one song he plays

  // Hide him (his column is still here because the chip filters rows, not columns).
  await lineupMusicianAction(page, "hide", gridOwner(page, "Zed")); // T211: hide moved into the ⋯ menu
  await expect(gridOwner(page, "Zed")).toHaveCount(0);

  // The chip still matches the stored data — the row stays.
  await expect(page.getByTestId("lineup-chip").filter({ hasText: "Zed" })).toHaveCount(1);
  await expect(page.getByTestId("lineup-row")).toHaveCount(1);

  // But a fresh search no longer suggests him ("as if he doesn't exist").
  await filter.fill("Zed");
  await expect(page.getByTestId("lineups-suggest-item").filter({ hasText: "Zed" })).toHaveCount(0);
});

// Seed one song where the guest "Zed" plays ONLY a shared part (Congas), and the admin is on lead vocal. A
// shared-only player has no personal column on the sheet — they appear in the shared part's own span.
async function seedSharedOnly(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json()).part;
  const congas = (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Congas", kind: "instrument", short: "Cg", shared: true } })).json()).part;
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const guest = (await (await page.request.post(`/api/bands/${id}/musicians`, { data: { name: "Zed" } })).json()).musician.id;
  const song = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song;
  await page.request.patch(`/api/bands/${id}/songs/${song.id}`, {
    data: { lineup: [{ musicianId: me, partIds: [lead.id] }, { musicianId: guest, partIds: [congas.id] }] },
  });
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Gig" } })).json()).setlist;
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: song.id } });
  return { id, me, guest, congas: congas.id, song: song.id, sl: sl.id as string };
}

// The one stored line-up for `song`, as {musicianId → partIds} — read from the API, the actual persisted state.
async function storedLineup(page: Page, id: string, song: string): Promise<Record<string, string[]>> {
  const songs = (await (await page.request.get(`/api/bands/${id}/songs`)).json()).songs;
  const lu = (songs.find((s: { id: string }) => s.id === song)?.lineup ?? []) as { musicianId: string; partIds: string[] }[];
  return Object.fromEntries(lu.map((e) => [e.musicianId, e.partIds]));
}

// Blocking 2 (Fable): the PAGE must pass the hidden set to assignSharedPart — the unit tests prove the function,
// not the wiring. Reassign a shared part while its holder is hidden, then read the stored line-up: the new holder
// has it AND the hidden holder still does. The sabotage (passing an empty set in setShared) strips the hidden
// holder → this reds, while "the admin holds it" stays green as the positive control.
test("T210 binding 1 on the page: taking a shared part while its holder is hidden leaves the hidden entry intact", async ({ page }) => {
  await register(page, `b1${stamp()}`);
  const { id, me, guest, congas, song } = await seedSharedOnly(page);

  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);

  // Hide Zed; the shared Congas cell now shows nobody (he's filtered out of the view).
  await lineupMusicianAction(page, "hide", gridOwner(page, "Zed")); // T211: hide moved into the ⋯ menu
  await expect(gridOwner(page, "Zed")).toHaveCount(0);

  // The admin takes the Congas through the shared-cell select (Zed isn't offered — he's hidden).
  const shared = page.getByTestId("lineup-cell-shared");
  await expect(shared).toHaveCount(1);
  await shared.selectOption(me);

  await expect.poll(async () => {
    const lu = await storedLineup(page, id, song);
    return { admin: (lu[me] ?? []).includes(congas), zed: (lu[guest] ?? []).includes(congas) };
  }).toEqual({ admin: true /* positive control */, zed: true /* the binding: Zed's entry untouched */ });
});

// Blocking 1 (Fable): a shared-only player has no personal owner header, so the header "not here" can't reach
// them (and must NOT render on a shared span, where ownerId is a part id). The "Not at this concert…" select is
// the way. Mark him absent through it → he leaves the shared header and the CSV; ↺ restores him.
test("T210 concert scope: a shared-only player is marked absent through the select; no button on a shared span", async ({ page }) => {
  await register(page, `sh${stamp()}`);
  const { id, guest, sl } = await seedSharedOnly(page);

  await page.goto(`/bands/${id}/setlists/${sl}/sheet`);
  await expect(page.getByTestId("sheet-grid")).toBeVisible();
  // Zed appears only in the shared "Congas" span — and that span carries NO "not here" button (fix a).
  await expect(sheetOwner(page, "Congas")).toHaveCount(1);
  await expect(sheetOwner(page, "Congas").getByTestId("sheet-owner-absent")).toHaveCount(0);

  // Mark him absent through the select (the only route for a shared-only player).
  await page.getByTestId("sheet-absent-add").selectOption(guest);
  await expect(sheetOwner(page, "Congas")).toHaveCount(0); // the shared column drops (nobody plays it now)
  await expect(page.getByTestId("sheet-absent-bar")).toContainText("Zed");
  expect(await csvText(page, id, sl)).not.toContain("Zed");

  // ↺ restores him and his shared column.
  await page.getByTestId("sheet-absent-restore").click();
  await expect(sheetOwner(page, "Congas")).toHaveCount(1);
  expect(await csvText(page, id, sl)).toContain("Zed"); // positive control
  await expect(page.getByTestId("sheet-absent-bar")).toHaveCount(0);
});

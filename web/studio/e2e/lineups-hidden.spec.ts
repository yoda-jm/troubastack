import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

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
  await expect(gridOwner(page, "Zed")).toHaveCount(1);
  await expect(page.getByTestId("lineups-hidden-bar")).toHaveCount(0); // nobody hidden yet

  // Hide the guest from his own column header.
  await gridOwner(page, "Zed").getByTestId("lineup-hide").click();
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
  await gridOwner(page, "Zed").getByTestId("lineup-hide").click();
  await expect(gridOwner(page, "Zed")).toHaveCount(0);

  // The chip still matches the stored data — the row stays.
  await expect(page.getByTestId("lineup-chip").filter({ hasText: "Zed" })).toHaveCount(1);
  await expect(page.getByTestId("lineup-row")).toHaveCount(1);

  // But a fresh search no longer suggests him ("as if he doesn't exist").
  await filter.fill("Zed");
  await expect(page.getByTestId("lineups-suggest-item").filter({ hasText: "Zed" })).toHaveCount(0);
});

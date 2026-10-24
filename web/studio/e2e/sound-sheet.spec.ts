import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// Set up a concert (one song, the creator on lead vocal + guitar = two amplified channels) and open its sound
// sheet. Returns nothing; the page is left on the sheet.
async function openSheet(page: Page, u: string) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json();
  const gtr = await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Guitar", kind: "instrument", short: "Gtr" } })).json();
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const song = await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json();
  await page.request.patch(`/api/bands/${id}/songs/${song.song.id}`, { data: { lineup: [{ musicianId: me, partIds: [lead.part.id, gtr.part.id] }] } });
  const sl = await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "The Gig" } })).json();
  await page.request.post(`/api/bands/${id}/setlists/${sl.setlist.id}/items`, { data: { songId: song.song.id } });
  await page.goto(`/bands/${id}/setlists/${sl.setlist.id}`);
  await page.getByTestId("sound-sheet-link").click();
  await expect(page.getByTestId("sheet-title")).toContainText("The Gig");
}

test("sound sheet: render, patch inline, order toggle, input list", async ({ page }) => {
  const u = `sheet${stamp()}`;
  await register(page, u);
  await openSheet(page, u);
  await expect(page.getByTestId("sheet-owner").filter({ hasText: `Display ${u}` })).toBeVisible();
  await expect(page.getByTestId("sheet-channel").filter({ hasText: "Voice" })).toBeVisible();
  await expect(page.getByTestId("sheet-row")).toHaveCount(1);

  await page.getByTestId("sheet-in").first().fill("1");
  await page.getByTestId("sheet-strip").first().fill("LEAD V");
  await page.getByTestId("sheet-save-patch").click();
  await expect(page.getByTestId("sheet-inputlist")).toContainText("In 1");
  await expect(page.getByTestId("sheet-inputlist")).toContainText("LEAD V");

  await expect(page.getByTestId("sheet-pdf")).toHaveAttribute("href", /sheet\.pdf/);
  await expect(page.getByTestId("sheet-csv")).toHaveAttribute("href", /sheet\.csv/);
});

// Binding 1: the order toggle must NOT discard unsaved patch edits.
test("sound sheet: unsaved patch edits survive an order toggle, then persist", async ({ page }) => {
  const u = `sheetd${stamp()}`;
  await register(page, u);
  await openSheet(page, u);

  await page.getByTestId("sheet-in").first().fill("7");
  await page.getByTestId("sheet-order").click(); // re-fetches
  await expect(page.getByTestId("sheet-order")).toHaveText("Order: patch");
  await expect(page.getByTestId("sheet-in").first()).toHaveValue("7"); // kept across the toggle
  await page.getByTestId("sheet-save-patch").click();
  await expect(page.getByTestId("sheet-inputlist")).toContainText("In 7");
});

// Binding 2: two channels on the same input are highlighted and the save is blocked.
test("sound sheet: duplicate inputs are flagged and block the save", async ({ page }) => {
  const u = `sheetx${stamp()}`;
  await register(page, u);
  await openSheet(page, u);

  await page.getByTestId("sheet-in").nth(0).fill("3");
  await page.getByTestId("sheet-in").nth(1).fill("3");
  await expect(page.getByTestId("sheet-in").nth(0)).toHaveAttribute("data-dup", "1");
  await expect(page.getByTestId("sheet-in").nth(1)).toHaveAttribute("data-dup", "1");
  await page.getByTestId("sheet-save-patch").click();
  await expect(page.getByText(/share input 3/)).toBeVisible(); // blocked with a message
  await expect(page.getByTestId("sheet-inputlist")).toHaveCount(0); // nothing saved
});

// T203-FIX: running numbers (not raw position), an intermission separator row, and an on-call section.
test("sound sheet: numbering, intermission separator, on-call section", async ({ page }) => {
  const u = `sheetn${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const lead = await (await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } })).json();
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const mkSong = async (title: string) => {
    const s = await (await page.request.post(`/api/bands/${id}/songs`, { data: { title } })).json();
    await page.request.patch(`/api/bands/${id}/songs/${s.song.id}`, { data: { lineup: [{ musicianId: me, partIds: [lead.part.id] }] } });
    return s.song.id;
  };
  const s1 = await mkSong("First");
  const s2 = await mkSong("Second");
  const bench = await mkSong("Bench");
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Show" } })).json()).setlist;
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s1 } });
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s2 } });
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { kind: "intermission", label: "Interval" } });
  const ocItem = await (await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: bench } })).json();
  await page.request.patch(`/api/bands/${id}/setlists/${sl.id}/items/${ocItem.item.id}`, { data: { onCall: true } });

  await page.goto(`/bands/${id}/setlists/${sl.id}/sheet`);
  await expect(page.getByTestId("sheet-title")).toContainText("Show");
  // main songs number 1, 2
  const rows = page.getByTestId("sheet-row");
  await expect(rows.nth(0)).toContainText("1 · First");
  await expect(rows.nth(1)).toContainText("2 · Second");
  // the intermission separator + its label
  await expect(page.getByTestId("sheet-intermission")).toContainText("Interval");
  // the on-call section heading, and its song carries no number
  await expect(page.getByTestId("sheet-oncall-head")).toBeVisible();
  const onCallRow = page.locator('[data-testid="sheet-row"][data-section="oncall"]');
  await expect(onCallRow).toContainText("Bench");
  await expect(onCallRow).not.toContainText("· Bench"); // unnumbered (no "N · ")
});

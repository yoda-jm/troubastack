import { test, expect } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T204 slice 2 — the Parts-manager "Track" UI: create a track, assign parts, the grid shows the track name, a
// mixed-input warning, delete-with-confirm reverts, and a merge report is never silent.

async function setup(page: import("@playwright/test").Page, inputs: { gtr: string; ah: string }) {
  const u = `trk${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const mkPart = async (name: string, short: string, input: string) =>
    (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name, kind: "instrument", short, input } })).json()).part;
  const gtr = await mkPart("Guitar", "Gtr", inputs.gtr);
  const ah = await mkPart("Autoharp", "AH", inputs.ah);
  const owner = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0];
  return { id, gtr, ah, me: owner.id, ownerName: owner.displayName as string };
}

test("create a track, assign two parts → one named grid column that cycles; mixed-input warning; delete reverts", async ({ page }) => {
  const { id, gtr, ah, me } = await setup(page, { gtr: "di", ah: "mic" });
  const mkSong = async (title: string, partId: string) => {
    const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title } })).json()).song;
    await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [{ musicianId: me, partIds: [partId] }] } });
  };
  await mkSong("One", gtr.id);
  await mkSong("Two", ah.id);

  await page.getByTestId("nav-lineups").click();
  await page.getByTestId("lineups-parts-toggle").click();

  // Create the "DI" track inline on Guitar, no prompt().
  await page.getByLabel("Track for Guitar").selectOption("__new__");
  await page.getByTestId("newtrack-name").fill("DI");
  await page.getByTestId("newtrack-create").click();
  // Assign Autoharp to the same track.
  await page.getByLabel("Track for Autoharp").selectOption({ label: "DI" });

  // The grid: ONE column, headed with the track NAME (not "Gtr/AH"), carrying the played part; click cycles.
  await expect(page.getByTestId("lineup-subcol")).toHaveCount(1);
  await expect(page.getByTestId("lineup-subcol")).toHaveText("DI");
  const cells = page.getByTestId("lineup-cell-inst");
  await expect(cells.nth(0)).toHaveText("Gtr");
  await expect(cells.nth(1)).toHaveText("AH");
  await cells.nth(0).click();
  await expect(page.getByTestId("lineup-cell-inst").nth(0)).toHaveText("AH");

  // The Tracks section flags the mixed input (Gtr=DI, AH=mic), non-blocking.
  await expect(page.getByTestId("track-warning")).toContainText("mixes mic and DI");

  // Delete the track → in-page confirm naming the count → revert; the two parts return to their own tracks.
  await page.getByTestId("track-delete").click();
  await expect(page.getByTestId("track-delete-confirm")).toContainText("Revert 2 parts");
  await page.getByTestId("track-delete-confirm-yes").click();
  await expect(page.getByTestId("lineups-tracks")).toHaveCount(0); // no tracks left
  // The parts are back on their own tracks — the column is headed by the part's short again, never "DI".
  // (The cycle above left this musician playing Autoharp in both songs, so one column remains.)
  await expect(page.getByTestId("lineup-subcol")).toHaveText("AH");
});

test("assigning two already-patched parts to one track merges patches, surfaces a notice, sheet shows the lower input", async ({ page }) => {
  const { id, gtr, ah, me, ownerName } = await setup(page, { gtr: "di", ah: "di" });
  const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } })).json()).song;
  await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [{ musicianId: me, partIds: [gtr.id, ah.id] }] } });
  const sl = (await (await page.request.post(`/api/bands/${id}/setlists`, { data: { name: "Gig" } })).json()).setlist;
  await page.request.post(`/api/bands/${id}/setlists/${sl.id}/items`, { data: { songId: s.id } });
  // Patch the two per-part channels on different inputs (4 and 6).
  await page.request.put(`/api/bands/${id}/setlists/${sl.id}/patch`, {
    data: { patch: [{ channel: `ch:${me}:p:${gtr.id}`, input: 4 }, { channel: `ch:${me}:p:${ah.id}`, input: 6 }] },
  });

  await page.getByTestId("nav-lineups").click();
  await page.getByTestId("lineups-parts-toggle").click();
  await page.getByLabel("Track for Guitar").selectOption("__new__");
  await page.getByTestId("newtrack-name").fill("DI");
  await page.getByTestId("newtrack-create").click();
  await page.getByLabel("Track for Autoharp").selectOption({ label: "DI" }); // this merge keeps the lower input

  // The merge notice is shown, and names the concert, WHOSE track (owner), WHICH track (DI), the kept input, and
  // links to that concert's sheet.
  await expect(page.getByTestId("merge-notice")).toBeVisible();
  await expect(page.getByTestId("merge-notice")).toContainText("Gig");
  await expect(page.getByTestId("merge-notice")).toContainText(ownerName); // whose
  await expect(page.getByTestId("merge-notice")).toContainText("DI"); // which track
  await expect(page.getByTestId("merge-notice")).toContainText("kept In 4");
  await expect(page.getByTestId("merge-notice-link")).toHaveAttribute("href", new RegExp(`/setlists/${sl.id}/sheet$`));

  // The sheet now shows ONE channel patched on the lower input (4), not two.
  await page.goto(`/bands/${id}/setlists/${sl.id}/sheet`);
  await expect(page.getByTestId("sheet-channel")).toHaveCount(1);
  await expect(page.getByTestId("sheet-in").first()).toHaveValue("4");
});

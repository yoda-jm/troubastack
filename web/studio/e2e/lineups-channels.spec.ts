import { test, expect } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T204 slice 1b — two parts assigned to ONE channel render as ONE cycling column in the line-up editor (not two
// per-part columns), and the cell carries whichever of the channel's parts the musician plays that song.
test("a channel groups its parts into one cycling column", async ({ page }) => {
  const u = `ch${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);

  const mkPart = async (name: string, short: string) =>
    (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name, kind: "instrument", short } })).json()).part;
  const gtr = await mkPart("Guitar", "Gtr");
  const ah = await mkPart("Autoharp", "AH");
  // One channel; both instruments join it.
  const chan = (await (await page.request.post(`/api/bands/${id}/channels`, { data: { name: "DI" } })).json()).channel;
  await page.request.patch(`/api/bands/${id}/parts/${gtr.id}/channel`, { data: { channelId: chan.id } });
  await page.request.patch(`/api/bands/${id}/parts/${ah.id}/channel`, { data: { channelId: chan.id } });

  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  const mkSong = async (title: string, partId: string) => {
    const s = (await (await page.request.post(`/api/bands/${id}/songs`, { data: { title } })).json()).song;
    await page.request.patch(`/api/bands/${id}/songs/${s.id}`, { data: { lineup: [{ musicianId: me, partIds: [partId] }] } });
    return s;
  };
  await mkSong("One", gtr.id); // the musician plays guitar here...
  await mkSong("Two", ah.id); // ...and autoharp here — same channel

  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();

  // Grouping: the two instruments are ONE sub-column, not two (no voice parts, one musician → exactly one).
  await expect(page.getByTestId("lineup-subcol")).toHaveCount(1);
  // The one column carries whichever channel part is played that song.
  const cells = page.getByTestId("lineup-cell-inst");
  await expect(cells).toHaveCount(2); // one per song row
  await expect(cells.nth(0)).toHaveText("Gtr");
  await expect(cells.nth(1)).toHaveText("AH");

  // Clicking the cell cycles through the channel's parts: Gtr → AH.
  await cells.nth(0).click();
  await expect(page.getByTestId("lineup-cell-inst").nth(0)).toHaveText("AH");
});

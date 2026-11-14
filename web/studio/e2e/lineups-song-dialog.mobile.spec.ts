import { test, expect, type Page } from "@playwright/test";
import { register, createBandAndOpen, stamp, enableEdit } from "./setup-helpers";

// T211 stage 2 — on a touch engine (the `mobile` project) the song dialog is a bottom sheet with ≥44px voice
// arrows, and (1d's binding) the ⋯ button and its menu items are ≥44px touch targets.

async function seed(page: Page) {
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  const mk = async (name: string, kind: string, short: string) =>
    (await (await page.request.post(`/api/bands/${id}/parts`, { data: { name, kind, short } })).json()).part;
  await mk("Lead vocal", "voice", "L");
  await mk("Backing vocal", "voice", "B");
  const gtr = await mk("Guitar", "instrument", "G");
  const me = (await (await page.request.get(`/api/bands/${id}/musicians`)).json()).musicians[0].id;
  await page.request.post(`/api/bands/${id}/musicians/${me}/instruments`, { data: { partId: gtr.id } });
  for (const t of ["Song A", "Song B"]) await page.request.post(`/api/bands/${id}/songs`, { data: { title: t } });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  return { id };
}

test("the song dialog is a bottom sheet with ≥44px voice arrows", async ({ page }) => {
  await register(page, `mdlg${stamp()}`);
  await seed(page);
  await page.getByTestId("lineup-song-edit").first().click();
  const dlg = page.getByTestId("song-dialog");
  await expect(dlg).toBeVisible();
  const box = (await dlg.boundingBox())!;
  const vp = page.viewportSize()!;
  // Bottom sheet: it spans the width and sits at the bottom of the screen.
  expect(box.width, "dialog width").toBeGreaterThan(vp.width - 4);
  expect(box.y + box.height, "dialog bottom").toBeGreaterThanOrEqual(vp.height - 2);
  // The voice arrows are ≥44px touch targets.
  for (const t of ["song-dialog-voice-prev", "song-dialog-voice-next"]) {
    const b = (await page.getByTestId(t).first().boundingBox())!;
    expect(b.width, `${t} width`).toBeGreaterThanOrEqual(44);
    expect(b.height, `${t} height`).toBeGreaterThanOrEqual(44);
  }
});

test("the ⋯ button and its menu items are ≥44px on touch (1d binding)", async ({ page }) => {
  await register(page, `mmore${stamp()}`);
  await seed(page);
  const more = page.getByTestId("lineup-musician-menu").first();
  const mb = (await more.boundingBox())!;
  expect(mb.width, "⋯ width").toBeGreaterThanOrEqual(44);
  expect(mb.height, "⋯ height").toBeGreaterThanOrEqual(44);
  await more.click();
  for (const t of ["lineup-musician-edit", "lineup-musician-left", "lineup-musician-right", "lineup-hide"]) {
    const b = (await page.getByTestId(t).boundingBox())!;
    expect(b.height, `${t} height`).toBeGreaterThanOrEqual(44);
  }
});

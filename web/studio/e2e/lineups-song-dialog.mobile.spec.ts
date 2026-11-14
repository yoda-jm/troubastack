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
  // 1d: the dialog's Close is a ≥44px touch target (was 55×34).
  const close = (await page.getByTestId("song-dialog-close").boundingBox())!;
  expect(close.height, "Close height").toBeGreaterThanOrEqual(44);
});

test("the last musician's voice list opens fully inside the sheet body (not clipped)", async ({ page }) => {
  await register(page, `mclip${stamp()}`);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Lead vocal", kind: "voice", short: "L" } });
  await page.request.post(`/api/bands/${id}/parts`, { data: { name: "Backing vocal", kind: "voice", short: "B" } });
  for (let i = 0; i < 8; i++) await page.request.post(`/api/bands/${id}/musicians`, { data: { name: `Player ${i}` } });
  await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Opener" } });
  await page.goto(`/bands/${id}/lineups`);
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  await enableEdit(page);
  await page.getByTestId("lineup-song-edit").first().click();

  // Open the LAST musician's voice list — the one that would be clipped below the scrolling body.
  const rows = page.getByTestId("song-dialog-musician");
  await rows.last().getByTestId("song-dialog-voice-value").click();
  await expect(page.getByTestId("song-dialog-voice-list")).toBeVisible();

  // Every option's rect is inside the sheet body's visible rect. SABOTAGE (don't flip/scroll it in) reds this.
  const body = (await page.locator(".lineup-dlg-body").boundingBox())!;
  const opts = page.getByTestId("song-dialog-voice-option");
  const n = await opts.count();
  for (let i = 0; i < n; i++) {
    const b = (await opts.nth(i).boundingBox())!;
    expect(b.y, `option ${i} top`).toBeGreaterThanOrEqual(body.y - 1);
    expect(b.y + b.height, `option ${i} bottom`).toBeLessThanOrEqual(body.y + body.height + 1);
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

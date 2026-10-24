import { test, expect } from "@playwright/test";
import { register, createBandAndOpen, stamp } from "./setup-helpers";

// T203 stage 2 — the band-level Line-ups grid. Seed the vocabulary, add a PERSISTED instrument, edit cells,
// prove the column survives toggling off its last usage (ruling 1), persist across reload, filter, by-part.
test("line-up grid: common parts, persisted instrument, edit + persist, filter, by-part", async ({ page }) => {
  const u = `lineup${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);

  for (const title of ["Alpha song", "Bravo song"]) {
    const r = await page.request.post(`/api/bands/${id}/songs`, { data: { title } });
    expect(r.ok()).toBeTruthy();
  }

  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  // Empty band: the creator shows as a musician column; the empty-state seed offer is present.
  await expect(page.getByTestId("lineup-col-owner").filter({ hasText: `Display ${u}` })).toBeVisible();
  await expect(page.getByTestId("lineup-row")).toHaveCount(2);

  // One-click seed the common vocabulary (voice + instruments). After this a Voice channel exists.
  await page.getByTestId("lineups-add-common").click();
  await expect(page.getByTestId("lineup-cell-voice").first()).toBeVisible();

  // Cycle the Voice channel on the first row (empty → a voice part, server-derived short).
  const voiceCell = page.getByTestId("lineup-cell-voice").first();
  await expect(voiceCell).toHaveText("·");
  await voiceCell.click();
  await expect(page.getByTestId("lineup-cell-voice").first()).not.toHaveText("·");

  // Give the musician a PERSISTED guitar via "+ part" → "+ Guitar". The Gtr channel appears; its cell is empty.
  await page.getByTestId("lineup-add-part").first().click();
  await page.getByTestId("lineup-assign-opt").filter({ hasText: "Guitar" }).click();
  const guitarCell = page.getByTestId("lineup-cell-inst").first();
  await expect(guitarCell).toHaveText("·"); // persisted column, no usage yet
  await expect(page.getByTestId("lineup-subcol").filter({ hasText: "Gui" })).toBeVisible();

  // Toggle the guitar ON on the first row, reload → it persists.
  await guitarCell.click();
  await expect(page.getByTestId("lineup-cell-inst").first()).toHaveText("Gui");
  await page.reload();
  await expect(page.getByTestId("lineup-cell-inst").first()).toHaveText("Gui");

  // RULING 1: toggle off the ONLY usage — the cell empties but the column DOES NOT vanish (persisted).
  await page.getByTestId("lineup-cell-inst").first().click();
  await expect(page.getByTestId("lineup-cell-inst").first()).toHaveText("·");
  await expect(page.getByTestId("lineup-subcol").filter({ hasText: "Gui" })).toBeVisible();

  // Filter by the musician (they sing on row 1 only) → a suggestion chip → one row.
  await page.getByTestId("lineups-filter").fill("Display");
  await page.getByTestId("lineups-suggest-item").first().click();
  await expect(page.getByTestId("lineup-chip")).toHaveCount(1);
  await expect(page.getByTestId("lineup-row")).toHaveCount(1);

  // By-part projection renders.
  await page.getByTestId("lineups-bypart").click();
  await expect(page.getByTestId("lineup-grid-bypart")).toBeVisible();
});

// T203-FIX2: an inline "+ part" named "Voix" is inferred as a VOICE part (not an instrument), so the Voice
// cell can then cycle it — VLL's "lead vs backing switch cannot be made".
test("inline part 'Voix' is inferred as voice and the Voice cell cycles it", async ({ page }) => {
  const u = `voix${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Alpha" } });
  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();

  // Create "Voix" inline on the musician's "+ part" — it should be inferred as voice.
  await page.getByTestId("lineup-add-part").first().click();
  await page.getByTestId("lineup-newinst").fill("Voix");
  await expect(page.getByTestId("lineup-infer-kind")).toContainText("voice");
  await page.getByTestId("lineup-newinst-add").click();

  // A Voice channel now exists; cycling the cell sets the voice part (no longer "·").
  const voice = page.getByTestId("lineup-cell-voice").first();
  await expect(voice).toBeVisible();
  await expect(voice).toHaveText("·");
  await voice.click();
  await expect(page.getByTestId("lineup-cell-voice").first()).not.toHaveText("·");
});

// T203-FIX2: the "+ part" popup must be closeable — ✕, Escape (focus back to the trigger), and outside click.
test("the + part popup closes via Escape, outside click, and ✕", async ({ page }) => {
  const u = `pop${stamp()}`;
  await register(page, u);
  const { id } = await createBandAndOpen(page, `Band ${stamp()}`);
  await page.request.post(`/api/bands/${id}/songs`, { data: { title: "Alpha" } });
  await page.getByTestId("nav-lineups").click();
  await expect(page.getByTestId("lineups-title")).toBeVisible();
  const trigger = page.getByTestId("lineup-add-part").first();

  // Escape closes and returns focus to the trigger.
  await trigger.click();
  await expect(page.getByTestId("lineup-assign")).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("lineup-assign")).toHaveCount(0);
  await expect(trigger).toBeFocused();

  // An outside click closes it.
  await trigger.click();
  await expect(page.getByTestId("lineup-assign")).toBeVisible();
  await page.getByTestId("lineups-title").click();
  await expect(page.getByTestId("lineup-assign")).toHaveCount(0);

  // The ✕ closes it.
  await trigger.click();
  await expect(page.getByTestId("lineup-assign")).toBeVisible();
  await page.getByTestId("lineup-assign-close").click();
  await expect(page.getByTestId("lineup-assign")).toHaveCount(0);
});

/**
 * T50 guard: personal song cues (icons + colors) in the editor's Details panel, and
 * the tinted chips they produce on setlist rows. A member picks a color + a glyph
 * ("red guitar-electric"), adds a neutral one ("mic"), and the pair persists across a
 * reload and renders tinted — self-only, ≤4. Red-first: the My-cues panel + the
 * setlist row cues don't exist pre-fix.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const RED = "rgb(225, 29, 72)"; // #e11d48

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function openCues(page: Page) {
  await page.getByTestId("my-files-edit").click(); // open the Details panel (T36)
  const panel = page.getByTestId("my-cues-panel");
  await expect(panel).toBeVisible();
  return panel;
}
// Computed CSS color of a chip's glyph (the applied tint).
function chipColor(page: Page, icon: string) {
  return page
    .locator(`[data-testid="cue-chip"][data-icon="${icon}"] svg.cue-glyph`)
    .evaluate((el) => getComputedStyle(el).color);
}

test("set red guitar + neutral mic, persists across reload, tinted (T50)", async ({ page }) => {
  await register(page, `cue_${stamp()}`);
  await createBandAndOpen(page, `CueBand ${stamp()}`);
  await createSongAndOpen(page, "Wonderwall");
  const panel = await openCues(page);

  // Pick red, then add the electric guitar → one tinted chip.
  await panel.getByTestId("cue-addcolor").filter({ has: page.locator('[data-color="#e11d48"]') });
  await panel.locator('[data-testid="cue-addcolor"][data-color="#e11d48"]').click();
  await panel.getByTestId("cue-add-guitar-electric").click();
  await expect(page.locator('[data-testid="cue-chip"][data-icon="guitar-electric"]')).toBeVisible();
  expect(await chipColor(page, "guitar-electric")).toBe(RED);

  // Back to neutral, add the mic → two chips, count 2/4.
  await panel.locator('[data-testid="cue-addcolor"][data-color=""]').click();
  await panel.getByTestId("cue-add-mic").click();
  await expect(page.getByTestId("cue-chip")).toHaveCount(2);
  await expect(panel.getByTestId("my-cues-count")).toHaveText("2/4");

  // Reload → the pair persists, order kept, guitar still red.
  await page.reload();
  const panel2 = await openCues(page);
  await expect(page.getByTestId("cue-chip")).toHaveCount(2);
  await expect(panel2.getByTestId("cue-chip").first()).toHaveAttribute("data-icon", "guitar-electric");
  await expect(panel2.getByTestId("cue-chip").nth(1)).toHaveAttribute("data-icon", "mic");
  expect(await chipColor(page, "guitar-electric")).toBe(RED);
});

test("the ≤4 cap disables the picker; remove frees a slot (T50)", async ({ page }) => {
  await register(page, `cap_${stamp()}`);
  await createBandAndOpen(page, `CapBand ${stamp()}`);
  await createSongAndOpen(page, "Champagne Supernova");
  const panel = await openCues(page);

  for (const icon of ["mic", "bass", "cajon", "shaker"]) {
    await panel.getByTestId(`cue-add-${icon}`).click();
  }
  await expect(page.getByTestId("cue-chip")).toHaveCount(4);
  await expect(panel.getByTestId("my-cues-count")).toHaveText("4/4");
  await expect(panel.getByTestId("my-cues-full")).toBeVisible();
  await expect(panel.getByTestId("cue-add-note")).toBeDisabled();

  // Remove one → picker re-enables, count drops.
  await panel.getByTestId("cue-remove").first().click();
  await expect(page.getByTestId("cue-chip")).toHaveCount(3);
  await expect(panel.getByTestId("cue-add-note")).toBeEnabled();
});

test("an unknown icon id falls back to the note glyph, never an error (T50)", async ({ page }) => {
  await register(page, `unk_${stamp()}`);
  await createBandAndOpen(page, `UnkBand ${stamp()}`);
  await createSongAndOpen(page, "Little By Little");
  const m = page.url().match(/\/bands\/([^/]+)\/songs\/([^/]+)$/)!;
  const [bandId, songId] = [m[1], m[2]];

  // A future/unknown icon id, set through the REAL API (shares the page's session).
  const res = await page.request.put(`/api/bands/${bandId}/songs/${songId}/my-cues`, {
    data: { cues: [{ icon: "theremin-2099", color: "#00ffcc" }] },
  });
  expect(res.ok()).toBeTruthy(); // server accepts it (additive; no icon allow-list)

  // On the setlist row it renders as the `note` fallback, tinted — no crash.
  await page.goto(`/bands/${bandId}/setlists`);
  await page.getByTestId("setlist-name").fill("Gig");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  await page.getByTestId("add-item-song").selectOption({ label: "Little By Little" });
  await page.getByTestId("add-item").click();

  const row = page.getByTestId("item-row").filter({ hasText: "Little By Little" });
  await expect(row.getByTestId("item-cues").locator('svg.cue-glyph[data-icon="note"]')).toHaveCount(1);
});

test("a song's cues show as tinted chips on its setlist row (T50)", async ({ page }) => {
  await register(page, `slc_${stamp()}`);
  await createBandAndOpen(page, `SlcBand ${stamp()}`);
  await createSongAndOpen(page, "Slide Away");
  const panel = await openCues(page);
  await panel.locator('[data-testid="cue-addcolor"][data-color="#e11d48"]').click();
  await panel.getByTestId("cue-add-guitar-electric").click();
  await panel.getByTestId("cue-add-mic").click();
  await expect(page.getByTestId("cue-chip")).toHaveCount(2);

  // The band's setlists live under /bands/{bandId}/setlists.
  const bandId = page.url().match(/\/bands\/([^/]+)\/songs\//)![1];
  await page.goto(`/bands/${bandId}/setlists`);
  await page.getByTestId("setlist-name").fill("Gig");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  await page.getByTestId("add-item-song").selectOption({ label: "Slide Away" });
  await page.getByTestId("add-item").click();

  const row = page.getByTestId("item-row").filter({ hasText: "Slide Away" });
  await expect(row.getByTestId("item-cues")).toBeVisible();
  await expect(row.getByTestId("item-cues").locator("svg.cue-glyph")).toHaveCount(2);
  // The guitar glyph carries the red tint on the row too.
  expect(
    await row.locator('svg.cue-glyph[data-icon="guitar-electric"]').evaluate((el) => getComputedStyle(el).color),
  ).toBe(RED);
});

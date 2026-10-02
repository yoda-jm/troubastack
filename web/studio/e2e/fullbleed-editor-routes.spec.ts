/**
 * T184 — the song editor is full-bleed on BOTH routes into it (flat and via a setlist). The regression:
 * T175 ⟨D5⟩ added the setlist route, a path regex in Shell only knew the flat one, so the editor opened from
 * a setlist kept the app navbar and its top/bottom pills scrolled off the viewport. Parametrised over both
 * routes so they can never drift again; it asserts the PILL POSITIONS (the symptom VLL reported), not only
 * the class (the mechanism).
 */
import { test, expect, type Page } from "@playwright/test";
import {
  createBandAndOpen,
  createSetlist,
  createSongAndOpen,
  register,
  stamp,
  uploadPdf,
} from "./setup-helpers";

async function setup(page: Page) {
  const who = stamp();
  await register(page, `fb${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songTitle = `Song ${who}`;
  const songId = await createSongAndOpen(page, songTitle);
  await uploadPdf(page); // real page content, so the broken layout would actually scroll
  await page.goto(`${band.url}/setlists`);
  await createSetlist(page, `Concert ${who}`);
  await page.getByTestId("setlist-link").filter({ hasText: `Concert ${who}` }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  const setlistId = page.url().split("/setlists/")[1];
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-title-link")).toHaveCount(1);
  return { bandId: band.id, songId, setlistId };
}

type Ids = Awaited<ReturnType<typeof setup>>;
const ROUTES: { name: string; url: (i: Ids) => string }[] = [
  { name: "flat", url: (i) => `/bands/${i.bandId}/songs/${i.songId}` },
  { name: "setlist", url: (i) => `/bands/${i.bandId}/setlists/${i.setlistId}/songs/${i.songId}` },
];

for (const r of ROUTES) {
  test(`editor via the ${r.name} route is full-bleed: navbar gone, bars anchored after a scroll`, async ({
    page,
  }) => {
    const ids = await setup(page);
    await page.goto(r.url(ids));
    // Gate on the editor's own chrome, which mounts with SongEditor — not pdf-page, whose pdf.js render is
    // slow under load and gates nothing the full-bleed decision depends on.
    await expect(page.getByTestId("viewer-chrome")).toBeVisible();

    // the mechanism…
    await expect(page.locator(".shell-fullbleed")).toHaveCount(1);
    await expect(page.locator(".topbar")).toHaveCount(0); // the app navbar is not drawn

    // …and the symptom: scroll the page and the chrome must stay put.
    await page.mouse.move(600, 400);
    await page.mouse.wheel(0, 2000);
    await page.waitForTimeout(200);
    expect(await page.evaluate(() => window.scrollY)).toBe(0); // the document is height-locked
    const chrome = await page.getByTestId("viewer-chrome").boundingBox();
    expect(chrome, "the editor's top chrome must exist").not.toBeNull();
    expect(chrome!.y).toBeGreaterThanOrEqual(0); // anchored, not drifted off to -1483 like the regression
  });
}

test("a non-editor page (the band) is NOT full-bleed — the navbar is back", async ({ page }) => {
  const ids = await setup(page);
  await page.goto(`/bands/${ids.bandId}`);
  await expect(page.getByTestId("band-detail").or(page.locator(".topbar"))).toBeVisible();
  await expect(page.locator(".shell-fullbleed")).toHaveCount(0);
  await expect(page.locator(".topbar")).toHaveCount(1);
});

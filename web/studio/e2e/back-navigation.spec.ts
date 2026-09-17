/**
 * T175 — Back returns where you came from.
 *
 * VLL: "when going from setlist to song, the back is not the setlist, this is not practical". A song has
 * several parents and the arrow used to name the band in all of them. Every row of §6 is new coverage:
 * `tb-back` appeared in the suite only as a visibility assertion, so nothing said where it went.
 */
import { test, expect, type Page } from "@playwright/test";
import { createBandAndOpen, createSetlist, createSongAndOpen, register, stamp } from "./setup-helpers";

/** A band with one song and one setlist containing it. Returns the ids and the setlist's URL. */
async function bandWithSetlist(page: Page) {
  const who = stamp();
  await register(page, `nav${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songTitle = `Song ${who}`;
  const songId = await createSongAndOpen(page, songTitle);

  await page.goto(`${band.url}/setlists`);
  await createSetlist(page, `Concert ${who}`);
  await page.getByTestId("setlist-link").filter({ hasText: `Concert ${who}` }).click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);
  const setlistUrl = page.url();
  const setlistId = setlistUrl.split("/setlists/")[1];

  // put the song in the setlist (the real controls, not invented ones — the first draft of this spec
  // used testids that exist nowhere)
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-title-link")).toHaveCount(1);

  return { band, songId, songTitle, setlistUrl, setlistId };
}

test("arriving from a setlist, Back returns to that setlist — not the band", async ({ page }) => {
  const { setlistUrl, setlistId } = await bandWithSetlist(page);

  await page.getByTestId("item-title-link").click();
  await expect(page).toHaveURL(new RegExp(`/setlists/${setlistId}/songs/`));
  await expect(page.getByTestId("pdf-page").first().or(page.getByTestId("viewer-chrome"))).toBeVisible();

  const back = page.getByTestId("tb-back");
  // ⟨D3⟩ the label and the destination must agree — assert BOTH, or a right destination under a wrong
  // word passes and the reader is told they are going somewhere they are not.
  await expect(back).toHaveAttribute("title", "Back to setlist");
  await back.click();
  await expect(page).toHaveURL(setlistUrl);
});

test("reloading first does not lose the origin — the row that rules out link state", async ({ page }) => {
  const { setlistUrl } = await bandWithSetlist(page);
  await page.getByTestId("item-title-link").click();
  await expect(page.getByTestId("tb-back")).toHaveAttribute("title", "Back to setlist");

  // Router link state dies here; an address survives. The row is nearly trivial now that the origin is
  // in the route — which is the point, and why it stays: it documents WHY the mechanism is an address
  // rather than something carried alongside one.
  await page.reload();
  const back = page.getByTestId("tb-back");
  await expect(back).toHaveAttribute("title", "Back to setlist");
  await back.click();
  await expect(page).toHaveURL(setlistUrl);
});

test("arriving from the band list, Back returns to the band", async ({ page }) => {
  const who = stamp();
  await register(page, `nav2${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  await createSongAndOpen(page, `Song ${who}`);

  const back = page.getByTestId("tb-back");
  await expect(back).toHaveAttribute("title", "Back to band");
  await back.click();
  await expect(page).toHaveURL(band.url);
});

test("a bare song URL falls back to the band, with no error", async ({ page }) => {
  const who = stamp();
  await register(page, `nav3${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);

  await page.goto(`/bands/${band.id}/songs/${songId}`);
  const back = page.getByTestId("tb-back");
  await expect(back).toHaveAttribute("title", "Back to band");
  await expect(page.getByTestId("error-banner")).toHaveCount(0);
  await back.click();
  await expect(page).toHaveURL(band.url);
});

test("a setlist that is gone takes you there anyway — the page says so, the arrow does not hide it", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `nav4${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);

  // ⟨D3⟩ as corrected: NOT silently retargeted to the band. The reader was in that setlist minutes ago;
  // quietly landing them elsewhere hides that something they were using is gone, and the setlist page
  // already owns its own not-found and its own authorisation.
  const ghost = "11111111-2222-3333-4444-555555555555";
  await page.goto(`/bands/${band.id}/setlists/${ghost}/songs/${songId}`);
  const back = page.getByTestId("tb-back");
  await expect(back).toHaveAttribute("title", "Back to setlist");
  await back.click();
  await expect(page).toHaveURL(`${band.url}/setlists/${ghost}`);

  // and the hostile input is not validated away — it cannot be expressed. A path segment holds no "/",
  // so this is simply a different (unmatched) route and never a setlist id.
  await page.goto(`/bands/${band.id}/setlists/..%2F..%2Fbands%2Felsewhere/songs/${songId}`);
  await expect(page.getByTestId("tb-back")).toHaveCount(0);
});

/**
 * T71 — the UI half of the lyrics.ovh search path. The dialog now leads with "Search by song"
 * (Artist, Title, Search), prefilled from the song's metadata, with the URL row demoted below it.
 * Search posts {artist,title} to the SAME /lyrics-import endpoint; every non-ok outcome degrades to
 * paste (the existing contract), showing the server's curated reason.
 *
 * Network-free by construction: `page.route` intercepts /lyrics-import so CI never depends on
 * lyrics.ovh being reachable (a hard requirement). Red-first: the search row doesn't exist pre-fix.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

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
async function createSongAndOpen(page: Page, title: string, artist: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  if (artist) await page.getByTestId("song-artist").fill(artist);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function openLyricsDialog(page: Page) {
  await page.getByTestId("my-files-edit").click();
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("new-lyrics-chart").click();
  await expect(panel.getByTestId("lyrics-dialog")).toBeVisible();
  return panel;
}

test("search by song: prefilled from metadata, sends {artist,title}, ok fills the textarea (T71)", async ({
  page,
}) => {
  await register(page, `ls_${stamp()}`);
  await createBandAndOpen(page, `LSBand ${stamp()}`);
  await createSongAndOpen(page, "The Open Road", "Trouba Test");

  let sentBody: unknown = null;
  await page.route("**/api/bands/*/lyrics-import", (route) => {
    sentBody = route.request().postDataJSON();
    return route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "ok", text: "found verse one\n\nfound verse two" }),
    });
  });

  const panel = await openLyricsDialog(page);
  // Prefilled from the song's metadata — the common case is open → click Search.
  await expect(panel.getByTestId("lyrics-title")).toHaveValue("The Open Road");
  await expect(panel.getByTestId("lyrics-artist")).toHaveValue("Trouba Test");

  await panel.getByTestId("lyrics-search").click();

  // ok fills the paste box for review (same affordance as the URL path).
  await expect(panel.getByTestId("lyrics-text")).toHaveValue(/found verse one[\s\S]*found verse two/);
  expect(sentBody).toEqual({ artist: "Trouba Test", title: "The Open Road" });

  // review → create still runs the normalize/section path.
  await panel.getByTestId("lyrics-create").click();
  await expect(panel.getByTestId("chart-source")).toBeVisible();
});

test("search is disabled until both artist and title are filled (T71)", async ({ page }) => {
  await register(page, `ld_${stamp()}`);
  await createBandAndOpen(page, `LDBand ${stamp()}`);
  await createSongAndOpen(page, "Solo Title", ""); // no artist → artist box prefills empty

  const panel = await openLyricsDialog(page);
  await expect(panel.getByTestId("lyrics-title")).toHaveValue("Solo Title");
  await expect(panel.getByTestId("lyrics-search")).toBeDisabled(); // artist blank
  await panel.getByTestId("lyrics-artist").fill("Someone");
  await expect(panel.getByTestId("lyrics-search")).toBeEnabled();
});

test("a disabled/error search shows the server's reason and leaves paste + create working (T71)", async ({
  page,
}) => {
  await register(page, `le_${stamp()}`);
  await createBandAndOpen(page, `LEBand ${stamp()}`);
  await createSongAndOpen(page, "Blocked Song", "The Band");
  await page.route("**/api/bands/*/lyrics-import", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "error", reason: "lyrics search is disabled on this server" }),
    }),
  );

  const panel = await openLyricsDialog(page);
  await panel.getByTestId("lyrics-search").click();

  // The server's curated reason is shown, and the user is not dead-ended.
  await expect(panel.getByTestId("lyrics-fetch-msg")).toContainText("lyrics search is disabled on this server");
  await expect(panel.getByTestId("lyrics-text")).toBeFocused();

  // Paste + create still works.
  await panel.getByTestId("lyrics-text").fill("pasted after a disabled search");
  await expect(panel.getByTestId("lyrics-create")).toBeEnabled();
  await panel.getByTestId("lyrics-create").click();
  await expect(panel.getByTestId("chart-source")).toBeVisible();
});

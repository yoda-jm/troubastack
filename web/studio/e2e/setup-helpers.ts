/**
 * Shared e2e SETUP primitives (T108). 77 of 81 specs hand-rolled `register`, and many also copied
 * `createBandAndOpen`, `createSongAndOpen`, and a PDF-upload helper — so a change to any of those flows
 * was a 77-file edit. This module is their single home.
 *
 * Like fullscreen-helpers, these change only HOW a spec reaches the state it needs, never WHAT it asserts.
 * A spec that is ABOUT one of these flows (registration itself) keeps driving it inline; the helpers are
 * for the many specs that merely NEED a logged-in user / a band / a song to exist.
 */
import { type Page, expect } from "@playwright/test";
import { fileURLToPath } from "node:url";

/** A unique-enough suffix so names don't collide across parallel specs. */
export const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

/** The committed sample PDF used by the upload helper. */
export const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

/** Register a logged-in user over the API — the fast path (T114) for the many specs that merely NEED a
 *  user, not the ones testing the sign-up flow (those use registerViaUi). The endpoints mirror the UI:
 *  register (201, creates the user) then login (200, sets the session cookie the page's /api/me auth
 *  check reads). Then land on /bands and wait for the authenticated first render — no cookie-only
 *  half-logged-in state, and no sleep. `page.request` shares the page's cookie jar, so the page is
 *  authenticated afterward exactly as the UI flow left it. */
export async function register(page: Page, username: string, password = "secret123") {
  const reg = await page.request.post("/api/auth/register", {
    data: { username, displayName: `Display ${username}`, password },
  });
  if (!reg.ok()) throw new Error(`register: ${reg.status()} ${await reg.text()}`);
  const login = await page.request.post("/api/auth/login", { data: { username, password } });
  if (!login.ok()) throw new Error(`login: ${login.status()} ${await login.text()}`);
  await page.goto("/bands");
  await expect(page).toHaveURL(/\/bands$/);
  await expect(page.getByTestId("new-band-btn")).toBeVisible(); // authenticated + first render settled
}

/** Register through the UI — for the ONE spec that is ABOUT the sign-up flow (flows §1). Kept so the
 *  register FORM keeps a UI walker after `register` moved to the API (T114). */
export async function registerViaUi(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

/** Create a band and open it. Returns the band URL + id; callers that only need the state ignore it. */
export async function createBandAndOpen(page: Page, name: string): Promise<{ url: string; id: string }> {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(name);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("band-title")).toHaveText(name);
  const url = page.url();
  return { url, id: url.split("/bands/")[1] };
}

/** Create a song (optionally with an artist) and open its editor. Returns the songId; callers that only
 *  need the state ignore it. */
export async function createSongAndOpen(page: Page, title: string, artist?: string): Promise<string> {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  if (artist) await page.getByTestId("song-artist").fill(artist);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  return page.url().split("/songs/")[1];
}

/** Upload the sample PDF into the open song's file pool, then close the details panel again — the common
 *  form used by the specs that need a file present but aren't testing the upload flow itself. */
export async function uploadPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}

/** Create a concert (setlist) via the T127 progressive-disclosure popup: reveal the form, fill it,
 *  submit. Assumes you are on the band's /setlists page. One home, so a future change to the create
 *  flow is one edit, not fifteen. */
export async function createSetlist(
  page: Page,
  name: string,
  opts: { date?: string; venue?: string } = {},
): Promise<void> {
  await page.getByTestId("new-setlist-btn").click();
  await page.getByTestId("setlist-name").fill(name);
  if (opts.date) await page.getByTestId("setlist-eventDate").fill(opts.date);
  if (opts.venue) await page.getByTestId("setlist-venue").fill(opts.venue);
  await page.getByTestId("create-setlist").click();
}

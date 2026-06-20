/**
 * End-to-end flows against the live stack (Go core + Vite SPA). Covers:
 *  1. register → land on /bands (empty)
 *  2. create a band → it appears → open it → I'm admin
 *  3. admin invites a second username; second user (separate context) registers,
 *     sees the pending invite, accepts, then can open the band / see it in /bands
 *  4. create a song → it appears → clicking shows the "editor coming soon" page
 *  5. logout → redirected to /login; visiting /bands while logged out → /login
 *
 * Each top-level user gets a unique username so runs stay independent even if a
 * backend is ever reused.
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

test("1. register lands on an empty /bands", async ({ page }) => {
  await register(page, `solo_${stamp()}`);
  await expect(page.getByTestId("bands-empty")).toBeVisible();
});

test("2. create a band, open it, I am admin", async ({ page }) => {
  await register(page, `owner_${stamp()}`);
  const bandName = `Band ${stamp()}`;

  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();

  const link = page.getByTestId("band-link").filter({ hasText: bandName });
  await expect(link).toBeVisible();
  await link.click();

  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  await expect(page.getByTestId("my-role")).toHaveText("admin");
});

test("3. invite a second user; they accept and gain access", async ({ browser }) => {
  const adminCtx = await browser.newContext();
  const inviteeCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const inviteePage = await inviteeCtx.newPage();

  const inviteeName = `invitee_${stamp()}`;
  const bandName = `Shared ${stamp()}`;

  // Admin registers, creates a band, opens it.
  await register(adminPage, `admin_${stamp()}`);
  await adminPage.getByTestId("band-name").fill(bandName);
  await adminPage.getByTestId("create-band").click();
  await adminPage.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(adminPage.getByTestId("my-role")).toHaveText("admin");

  // Admin invites the second user by username.
  await adminPage.getByTestId("invite-identifier").fill(inviteeName);
  await adminPage.getByTestId("invite-kind").selectOption("username");
  await adminPage.getByTestId("invite-submit").click();
  await expect(adminPage.getByTestId("invite-notice")).toBeVisible();

  // Second user registers (separate cookie jar), sees the pending invite.
  await register(inviteePage, inviteeName);
  await inviteePage.getByTestId("nav-invites").click();
  await expect(inviteePage).toHaveURL(/\/invites$/);
  const row = inviteePage.getByTestId("invite-row");
  await expect(row).toHaveCount(1);

  // Accept it.
  await inviteePage.getByTestId("invite-accept").click();
  await expect(inviteePage.getByTestId("invites-empty")).toBeVisible();

  // Now the invitee sees the band in /bands and can open it.
  await inviteePage.goto("/bands");
  const bandLink = inviteePage.getByTestId("band-link").filter({ hasText: bandName });
  await expect(bandLink).toBeVisible();
  await bandLink.click();
  await expect(inviteePage.getByTestId("band-title")).toHaveText(bandName);
  // Invitee is a plain member, not admin.
  await expect(inviteePage.getByTestId("my-role")).toHaveText("member");

  await adminCtx.close();
  await inviteeCtx.close();
});

test("4. create a song; clicking it shows the editor placeholder", async ({ page }) => {
  await register(page, `songwriter_${stamp()}`);
  const bandName = `Songs ${stamp()}`;
  const songTitle = `Tune ${stamp()}`;

  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();

  await page.getByTestId("song-title").fill(songTitle);
  await page.getByTestId("song-artist").fill("The Authors");
  await page.getByTestId("create-song").click();

  const songLink = page.getByTestId("song-link").filter({ hasText: songTitle });
  await expect(songLink).toBeVisible();
  await songLink.click();

  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  await expect(page.getByTestId("editor-placeholder")).toBeVisible();
  await expect(page.getByTestId("editor-placeholder")).toContainText("Editor coming soon");
});

test("5. logout redirects to /login; guarded routes redirect when logged out", async ({ page }) => {
  await register(page, `leaver_${stamp()}`);

  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login$/);

  // Visiting a guarded route while logged out → bounced to /login.
  await page.goto("/bands");
  await expect(page).toHaveURL(/\/login$/);
});

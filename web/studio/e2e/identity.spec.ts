/**
 * Identity & onboarding e2e against the live alt-port stack (core :8090 / Vite
 * :5183). Covers: profile edit persistence, password change + re-login, band
 * invite-link create → second user joins via /join/<token>, avatar silhouettes
 * in the members list, and the logged-out /join → login → return flow.
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

test("a. profile edit persists across reload", async ({ page }) => {
  await register(page, `prof_${stamp()}`);
  await page.getByTestId("nav-profile").click();
  await expect(page).toHaveURL(/\/me$/);

  const newName = `Edited ${stamp()}`;
  await page.getByTestId("profile-displayname").fill(newName);
  await page.getByTestId("profile-email").fill(`p${stamp()}@example.com`);
  await page.getByTestId("profile-avatar-woman").check();
  await page.getByTestId("profile-save").click();
  await expect(page.getByTestId("profile-notice")).toBeVisible();

  // Personal QR rendered.
  await expect(page.getByTestId("profile-qr").locator("svg")).toBeVisible();

  await page.reload();
  await expect(page.getByTestId("profile-displayname")).toHaveValue(newName);
  await expect(page.getByTestId("profile-avatar-woman")).toBeChecked();
  // Topbar reflects the new display name.
  await expect(page.getByTestId("current-user")).toHaveText(newName);
});

test("b. change password, logout, login with the new password", async ({ page }) => {
  const username = `pw_${stamp()}`;
  await register(page, username, "originalpw");
  await page.getByTestId("nav-profile").click();

  await page.getByTestId("pw-current").fill("originalpw");
  await page.getByTestId("pw-new").fill("changedpw");
  await page.getByTestId("pw-confirm").fill("changedpw");
  await page.getByTestId("pw-save").click();
  await expect(page.getByTestId("pw-notice")).toBeVisible();

  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login/);

  // Old password fails, new one succeeds.
  await page.getByTestId("username").fill(username);
  await page.getByTestId("password").fill("originalpw");
  await page.getByTestId("submit").click();
  await expect(page.getByTestId("error")).toBeVisible();

  await page.getByTestId("password").fill("changedpw");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
});

test("c. admin creates an invite link; second user joins via /join/<token>", async ({
  browser,
}) => {
  const adminCtx = await browser.newContext();
  const joinerCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const joinerPage = await joinerCtx.newPage();

  const bandName = `LinkBand ${stamp()}`;
  await register(adminPage, `linkadmin_${stamp()}`);
  await createBandAndOpen(adminPage, bandName);
  const bandUrl = adminPage.url();

  // Open settings, create a member invite link, read its url.
  await adminPage.goto(bandUrl + "/settings");
  await adminPage.getByTestId("invite-link-role").selectOption("member");
  await adminPage.getByTestId("create-invite-link").click();
  await expect(adminPage.getByTestId("invite-link-row")).toHaveCount(1);
  await expect(adminPage.getByTestId("invite-link-qr").locator("svg")).toBeVisible();
  const url = await adminPage.getByTestId("invite-link-url").inputValue();
  const token = url.split("/join/")[1];
  expect(token).toBeTruthy();

  // Second user registers, visits the join page, joins.
  await register(joinerPage, `joiner_${stamp()}`);
  await joinerPage.goto(`/join/${token}`);
  await expect(joinerPage.getByTestId("join-band-name")).toHaveText(bandName);
  await expect(joinerPage.getByTestId("join-role")).toHaveText("member");
  await joinerPage.getByTestId("join-accept").click();

  // Lands on the band detail, member role.
  await expect(joinerPage.getByTestId("band-title")).toHaveText(bandName);
  await expect(joinerPage.getByTestId("my-role")).toHaveText("member");

  // The band now appears in /bands.
  await joinerPage.goto("/bands");
  await expect(joinerPage.getByTestId("band-link").filter({ hasText: bandName })).toBeVisible();

  await adminCtx.close();
  await joinerCtx.close();
});

test("d. avatar silhouette renders in the members list", async ({ page }) => {
  await register(page, `av_${stamp()}`);
  await createBandAndOpen(page, `AvatarBand ${stamp()}`);
  const row = page.getByTestId("member-row").first();
  await expect(row.getByTestId("avatar").locator("svg")).toBeVisible();
});

test("e. logged-out /join → login → returns to join and can join", async ({ browser }) => {
  const adminCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const bandName = `GuardBand ${stamp()}`;
  await register(adminPage, `guardadmin_${stamp()}`);
  await createBandAndOpen(adminPage, bandName);
  const bandUrl = adminPage.url();
  await adminPage.goto(bandUrl + "/settings");
  await adminPage.getByTestId("invite-link-role").selectOption("member");
  await adminPage.getByTestId("create-invite-link").click();
  const url = await adminPage.getByTestId("invite-link-url").inputValue();
  const token = url.split("/join/")[1];
  await adminCtx.close();

  // Fresh (logged-out) context registers a user up front, logs OUT, then hits /join.
  const userCtx = await browser.newContext();
  const userPage = await userCtx.newPage();
  const uname = `guarduser_${stamp()}`;
  await register(userPage, uname, "joinpw1");
  await userPage.getByTestId("logout").click();
  await expect(userPage).toHaveURL(/\/login/);

  // Visit the protected join URL while logged out → bounced to /login?next=...
  await userPage.goto(`/join/${token}`);
  await expect(userPage).toHaveURL(/\/login\?next=/);

  // Log in → returns to the join page.
  await userPage.getByTestId("username").fill(uname);
  await userPage.getByTestId("password").fill("joinpw1");
  await userPage.getByTestId("submit").click();
  await expect(userPage).toHaveURL(new RegExp(`/join/${token}$`));
  await expect(userPage.getByTestId("join-band-name")).toHaveText(bandName);

  // Can join.
  await userPage.getByTestId("join-accept").click();
  await expect(userPage.getByTestId("band-title")).toHaveText(bandName);
  await expect(userPage.getByTestId("my-role")).toHaveText("member");

  await userCtx.close();
});

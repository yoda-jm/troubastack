/**
 * T21: admin-assisted password reset.
 * An admin issues a one-time reset link for a band member; the member opens the
 * link (the token is the credential — no session), sets a new password, and is
 * signed out everywhere: the old session is dead, the old password fails, and
 * the new password logs in.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

test("admin issues a reset link; member sets a new password; old session + password die", async ({
  browser,
}) => {
  const adminCtx = await browser.newContext();
  const memberCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const memberPage = await memberCtx.newPage();

  const bandName = `ResetBand ${stamp()}`;
  await register(adminPage, `resetadmin_${stamp()}`);
  await createBandAndOpen(adminPage, bandName);
  const bandUrl = adminPage.url();

  // Admin mints a member invite link so a second real member exists.
  await adminPage.goto(bandUrl + "/settings");
  await adminPage.getByTestId("invite-link-role").selectOption("member");
  await adminPage.getByTestId("create-invite-link").click();
  const inviteUrl = await adminPage.getByTestId("invite-link-url").inputValue();
  const inviteToken = inviteUrl.split("/join/")[1];
  expect(inviteToken).toBeTruthy();

  // Bob registers (old password), joins the band.
  const bob = `resetbob_${stamp()}`;
  await register(memberPage, bob, "oldpassword");
  await memberPage.goto(`/join/${inviteToken}`);
  await memberPage.getByTestId("join-accept").click();
  await expect(memberPage.getByTestId("my-role")).toHaveText("member");

  // Admin opens the band, finds Bob's row, issues a reset, reads the link.
  await adminPage.goto(bandUrl);
  const bobRow = adminPage.getByTestId("member-row").filter({ hasText: bob });
  await bobRow.getByTestId("reset-password").click();
  const resetUrl = await bobRow.getByTestId("reset-link").inputValue();
  const resetPath = new URL(resetUrl).pathname;
  expect(resetPath).toMatch(/^\/reset-password\/.+/);

  // Bob opens the link (his stale session cookie is still in this context) and
  // sets a new password.
  await memberPage.goto(resetPath);
  await expect(memberPage.getByTestId("reset-target")).toHaveText(`@${bob}`);
  await memberPage.getByTestId("reset-new-password").fill("brandnewpw");
  await memberPage.getByTestId("reset-submit").click();
  await expect(memberPage.getByTestId("reset-done")).toBeVisible();

  // Old session is dead: hitting an authed page bounces to login.
  await memberPage.goto("/bands");
  await expect(memberPage).toHaveURL(/\/login/);

  // Old password fails; the new one logs in.
  await memberPage.goto("/login");
  await memberPage.getByTestId("username").fill(bob);
  await memberPage.getByTestId("password").fill("oldpassword");
  await memberPage.getByTestId("submit").click();
  await expect(memberPage.getByTestId("error")).toBeVisible();

  await memberPage.getByTestId("password").fill("brandnewpw");
  await memberPage.getByTestId("submit").click();
  await expect(memberPage).toHaveURL(/\/bands$/);

  await adminCtx.close();
  await memberCtx.close();
});

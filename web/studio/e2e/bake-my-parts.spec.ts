/**
 * B07: a band MEMBER (not admin) bakes their personal parts.
 * The member sees "Bake my parts" on the setlist (but not the admin-only band
 * bake), bakes their own variant, and a download link for it appears. Uses an
 * empty setlist so no poppler/web-bake toolchain is needed.
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

test("a member bakes their own parts → variant download appears", async ({ browser }) => {
  const adminCtx = await browser.newContext();
  const memberCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const memberPage = await memberCtx.newPage();

  // Admin: band + a member invite link + a setlist.
  const bandName = `PartsBand ${stamp()}`;
  await register(adminPage, `partsadmin_${stamp()}`);
  await adminPage.getByTestId("new-band-btn").click();
  await adminPage.getByTestId("band-name").fill(bandName);
  await adminPage.getByTestId("create-band").click();
  await adminPage.getByTestId("band-link").filter({ hasText: bandName }).click();
  const bandUrl = adminPage.url();

  await adminPage.goto(bandUrl + "/settings");
  await adminPage.getByTestId("invite-link-role").selectOption("member");
  await adminPage.getByTestId("create-invite-link").click();
  const inviteUrl = await adminPage.getByTestId("invite-link-url").inputValue();
  const token = inviteUrl.split("/join/")[1];

  await adminPage.goto(bandUrl);
  await adminPage.getByTestId("nav-setlists").click();
  await adminPage.getByTestId("setlist-name").fill("Gig");
  await adminPage.getByTestId("create-setlist").click();

  // Member joins the band.
  await register(memberPage, `partsmember_${stamp()}`);
  await memberPage.goto(`/join/${token}`);
  await memberPage.getByTestId("join-accept").click();
  await expect(memberPage.getByTestId("my-role")).toHaveText("member");

  // Member opens the setlist.
  await memberPage.goto(bandUrl);
  await memberPage.getByTestId("nav-setlists").click();
  await memberPage.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  await expect(memberPage.getByTestId("bake-card")).toBeVisible();

  // The member sees "Bake my parts" but NOT the admin-only band bake.
  await expect(memberPage.getByTestId("bake-mine")).toBeVisible();
  await expect(memberPage.getByTestId("bake-setlist")).toHaveCount(0);

  // Bake my parts → the personal download link appears.
  await expect(memberPage.getByTestId("bake-mine-download")).toHaveCount(0);
  await memberPage.getByTestId("bake-mine").click();
  await memberPage.getByTestId("bake-dialog-confirm").click(); // P205 bake dialog
  await expect(memberPage.getByTestId("bake-mine-download")).toBeVisible();
  await expect(memberPage.getByTestId("bake-history-row").filter({ hasText: "Mine" })).toHaveCount(1); // T56: audience tag replaced "My parts"

  await adminCtx.close();
  await memberCtx.close();
});

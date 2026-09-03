/**
 * T131: fast actions on the concert row — bake/re-bake (reusing the detail page's bake dialog +
 * T103 kick-and-poll), and PDF + bundle links that appear only once a bake exists. Verified by an
 * ACTUAL bake (a song with no PDF needs no poppler/web-bake toolchain), not a click-handler unit test.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, createSetlist } from "./setup-helpers";

test("concert row: bake from the row, then PDF + bundle appear; an empty setlist can't bake", async ({
  page,
}) => {
  await register(page, `rowb_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `RowBand ${stamp()}`);
  const songTitle = `Open Road ${stamp()}`;
  await createSongAndOpen(page, songTitle); // no PDF → real bake needs no toolchain

  await page.goto(`/bands/${bandId}/setlists`);
  const emptyName = `Empty ${stamp()}`;
  await createSetlist(page, emptyName);
  const gigName = `Gig ${stamp()}`;
  await createSetlist(page, gigName);

  // Put the song on the Gig setlist (from its detail page), then return to the list.
  await page.getByTestId("setlist-link").filter({ hasText: gigName }).click();
  await page.getByTestId("add-item-song").selectOption({ label: songTitle });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toContainText(songTitle);
  await page.goto(`/bands/${bandId}/setlists`);

  const row = (name: string) =>
    page.locator("li", { has: page.getByTestId("setlist-link").filter({ hasText: name }) });

  // Empty setlist: the row's bake action is DISABLED, with the detail page's exact guard wording.
  await row(emptyName).getByTestId("setlist-menu").click();
  const emptyBake = page.getByTestId("setlist-rebake");
  await expect(emptyBake).toBeDisabled();
  await expect(emptyBake).toHaveAttribute("title", /Add at least one song/);
  await page.keyboard.press("Escape");

  // Gig setlist, not yet baked: bake action enabled, but NO PDF/bundle items (never a 404 link).
  await row(gigName).getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-rebake")).toBeEnabled();
  await expect(page.getByTestId("setlist-pdf")).toHaveCount(0);
  await expect(page.getByTestId("setlist-bundle")).toHaveCount(0);

  // Bake it from the row: confirm (naming the concert) → the SAME bake dialog → terminal.
  await page.getByTestId("setlist-rebake").click();
  await page.getByTestId("app-dialog-confirm").click(); // "Bake “Gig …”?"
  await page.getByTestId("bake-dialog-confirm").click();
  await expect(page.getByTestId("bake-dialog")).toBeHidden();

  // After baking, the list refreshes and the row now offers PDF + bundle.
  await row(gigName).getByTestId("setlist-menu").click();
  await expect(page.getByTestId("setlist-pdf")).toBeVisible();
  await expect(page.getByTestId("setlist-bundle")).toBeVisible();
});

test("non-admin sees only Duplicate on a concert row — no bake, delete, PDF or bundle (T131)", async ({
  browser,
}) => {
  const adminCtx = await browser.newContext();
  const memberCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const memberPage = await memberCtx.newPage();

  const memberName = `rowmember_${stamp()}`;
  await register(adminPage, `rowadmin_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(adminPage, `RowRole ${stamp()}`);

  // Admin invites the member; the member accepts → a non-admin member of the band.
  await adminPage.getByTestId("invite-toggle").click();
  await adminPage.getByTestId("invite-identifier").fill(memberName);
  await adminPage.getByTestId("invite-submit").click();
  await expect(adminPage.getByTestId("invite-notice")).toBeVisible();
  await register(memberPage, memberName);
  await memberPage.getByTestId("nav-invites").click();
  await memberPage.getByTestId("invite-accept").click();
  await expect(memberPage.getByTestId("invites-empty")).toBeVisible();

  // Admin makes an (unbaked) setlist — so PDF/bundle are absent for everyone regardless of role.
  await adminPage.goto(`/bands/${bandId}/setlists`);
  const name = `RoleGig ${stamp()}`;
  await createSetlist(adminPage, name);

  // The member opens that row's menu: Duplicate ONLY — admin actions (bake, delete) are gone, and the
  // never-baked links are absent. Checked with a real non-admin account, not by reasoning about the gate.
  await memberPage.goto(`/bands/${bandId}/setlists`);
  await memberPage
    .locator("li", { has: memberPage.getByTestId("setlist-link").filter({ hasText: name }) })
    .getByTestId("setlist-menu")
    .click();
  await expect(memberPage.getByTestId("setlist-duplicate")).toBeVisible();
  await expect(memberPage.getByTestId("setlist-rebake")).toHaveCount(0);
  await expect(memberPage.getByTestId("setlist-delete")).toHaveCount(0);
  await expect(memberPage.getByTestId("setlist-live-toggle")).toHaveCount(0); // T132: admin-only
  await expect(memberPage.getByTestId("setlist-pdf")).toHaveCount(0);
  await expect(memberPage.getByTestId("setlist-bundle")).toHaveCount(0);

  await adminCtx.close();
  await memberCtx.close();
});

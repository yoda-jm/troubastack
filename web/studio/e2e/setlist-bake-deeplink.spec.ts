/**
 * T189 — the app's concert ⋯ "Bake" (A83) deep-links to the concert page with `?bake=1`, which opens
 * Studio's OWN bake dialog once and then strips the parameter. Reuses the Bake button's dialog, never a
 * second path. Silently ignored (but still stripped) for a non-admin or an empty concert; other params
 * (the app's `?embedded=1`) survive the strip; a reload or back/forward never reopens it.
 */
import { test, expect, type Page } from "@playwright/test";
import {
  stamp,
  register,
  createBandAndOpen,
  createSongAndOpen,
  createSetlist,
  revealedInviteUrl,
} from "./setup-helpers";

/** A band owned by the freshly-registered admin, with a "Gig" concert that holds one song (unless
 *  `withSong` is false — the empty-concert case). Leaves the page on the concert detail; returns its ids. */
async function adminBandWithSetlist(page: Page, withSong = true) {
  await register(page, `bake_${stamp()}`);
  const band = await createBandAndOpen(page, `BakeBand ${stamp()}`);
  if (withSong) await createSongAndOpen(page, "The Open Road");
  await page.goto(`/bands/${band.id}/setlists`);
  await createSetlist(page, "Gig");
  await page.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  if (withSong) {
    await page.getByTestId("add-item-song").selectOption({ label: "The Open Road" });
    await page.getByTestId("add-item").click();
    await expect(page.getByTestId("bake-setlist")).toBeEnabled();
  }
  const setlistId = page.url().split("/setlists/")[1].split("?")[0];
  return { bandId: band.id, setlistId };
}

const hasBake = (page: Page) => new URL(page.url()).searchParams.has("bake");

test("admin: ?bake=1 opens the dialog, strips the param, and a reload does not reopen it", async ({
  page,
}) => {
  const { bandId, setlistId } = await adminBandWithSetlist(page);
  await page.goto(`/bands/${bandId}/setlists/${setlistId}?bake=1`);

  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  await expect.poll(() => hasBake(page)).toBe(false); // the parameter is gone

  await page.reload();
  await expect(page.getByTestId("bake-card")).toBeVisible();
  await expect(page.getByTestId("bake-dialog")).toHaveCount(0); // not reopened
});

test("embedded: the dialog opens and ?embedded=1 survives the strip", async ({ page }) => {
  const { bandId, setlistId } = await adminBandWithSetlist(page);
  await page.goto(`/bands/${bandId}/setlists/${setlistId}?embedded=1&bake=1`);

  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  await expect
    .poll(() => {
      const sp = new URL(page.url()).searchParams;
      return { bake: sp.has("bake"), embedded: sp.get("embedded") };
    })
    .toEqual({ bake: false, embedded: "1" });
});

test("non-admin member: no dialog, no error banner, and bake is stripped", async ({ browser }) => {
  const adminCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const { bandId, setlistId } = await adminBandWithSetlist(adminPage);

  // A member (non-admin) invite link.
  await adminPage.goto(`/bands/${bandId}/settings`);
  await adminPage.getByTestId("invite-link-role").selectOption("member");
  await adminPage.getByTestId("create-invite-link").click();
  const token = (await revealedInviteUrl(adminPage)).split("/join/")[1];

  const memberCtx = await browser.newContext();
  const memberPage = await memberCtx.newPage();
  await register(memberPage, `mbr_${stamp()}`);
  await memberPage.goto(`/join/${token}`);
  await memberPage.getByTestId("join-accept").click();
  await expect(memberPage.getByTestId("my-role")).toHaveText("member");

  await memberPage.goto(`/bands/${bandId}/setlists/${setlistId}?bake=1`);
  await expect(memberPage.getByTestId("bake-card")).toBeVisible();
  await expect(memberPage.getByTestId("bake-dialog")).toHaveCount(0);
  await expect(memberPage.getByTestId("error")).toHaveCount(0); // no error banner
  await expect.poll(() => hasBake(memberPage)).toBe(false);

  await adminCtx.close();
  await memberCtx.close();
});

test("empty concert, admin: no dialog, and bake is stripped", async ({ page }) => {
  const { bandId, setlistId } = await adminBandWithSetlist(page, false);
  await page.goto(`/bands/${bandId}/setlists/${setlistId}?bake=1`);

  await expect(page.getByTestId("bake-card")).toBeVisible();
  await expect(page.getByTestId("bake-dialog")).toHaveCount(0);
  await expect.poll(() => hasBake(page)).toBe(false);
});

test("any other value (bake=yes) is ignored and stripped", async ({ page }) => {
  const { bandId, setlistId } = await adminBandWithSetlist(page);
  await page.goto(`/bands/${bandId}/setlists/${setlistId}?bake=yes`);

  await expect(page.getByTestId("bake-card")).toBeVisible();
  await expect(page.getByTestId("bake-dialog")).toHaveCount(0);
  await expect.poll(() => hasBake(page)).toBe(false);
});

test("cancelling the opened dialog leaves the user on the concert page", async ({ page }) => {
  const { bandId, setlistId } = await adminBandWithSetlist(page);
  await page.goto(`/bands/${bandId}/setlists/${setlistId}?bake=1`);

  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  await page.getByTestId("bake-dialog-cancel").click();
  await expect(page.getByTestId("bake-dialog")).toHaveCount(0);
  await expect(page).toHaveURL(new RegExp(`/bands/${bandId}/setlists/${setlistId}`));
  await expect(page.getByTestId("bake-card")).toBeVisible();
});

/**
 * T89 — the Details panel (fullscreen editor) must have a way OUT beyond the one top-bar pill,
 * which on a phone is an unlabelled icon in a horizontal-scroll strip. Adds a ✕ in the sticky
 * tabs row, Escape, and outside-click — each asserted at a phone viewport; the ✕ must survive
 * scrolling the (long) panel body; both admin and non-admin tab layouts.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function createBand(page: Page, name: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(name);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("band-title")).toHaveText(name);
}
async function createSong(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}

test("editor: Details closes via ✕, Escape, and outside-click on a phone; ✕ survives scroll (T89)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 620 }); // short phone → the panel body overflows
  await register(page, `t89_${stamp()}`);
  await createBand(page, `T89Band ${stamp()}`);
  await createSong(page, `T89Song ${stamp()}`);

  const panel = page.getByTestId("details-panel");
  const close = page.getByTestId("details-close");
  const open = () => page.getByTestId("my-files-edit").click();

  // 1) the ✕ closes it
  await open();
  await expect(panel).toBeVisible();
  await expect(close).toBeVisible();
  await close.click();
  await expect(panel).toHaveCount(0);

  // 2) Escape closes it
  await open();
  await expect(panel).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(panel).toHaveCount(0);

  // 3) a click OUTSIDE the panel closes it — the panel is centred (14px side margin on a 390px
  //    phone), so a click at x=4 lands in the viewer behind it, outside the panel and the pill.
  await open();
  await expect(panel).toBeVisible();
  await page.mouse.click(4, 300);
  await expect(panel).toHaveCount(0);

  // 4) the ✕ survives scrolling the panel body — the actual failure mode
  await open();
  await expect(panel).toBeVisible();
  await panel.evaluate((el) => (el.scrollTop = el.scrollHeight)); // scroll to the bottom
  await expect(close).toBeVisible();
  const box = (await close.boundingBox())!;
  const vh = page.viewportSize()!.height;
  expect(box.y).toBeGreaterThanOrEqual(0);
  expect(box.y + box.height).toBeLessThanOrEqual(vh); // still fully on-screen (sticky tabs row)
  expect(box.y).toBeLessThan(200); // stayed near the top, not scrolled away
  await close.click();
  await expect(panel).toHaveCount(0);
});

test("editor: the pill still toggles Details on desktop (unchanged) (T89)", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 800 });
  await register(page, `t89d_${stamp()}`);
  await createBand(page, `T89dBand ${stamp()}`);
  await createSong(page, `T89dSong ${stamp()}`);
  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await page.getByTestId("my-files-edit").click(); // pill toggles closed
  await expect(panel).toHaveCount(0);
});

test("editor: a non-admin member also gets a working ✕ close (T89)", async ({ browser }) => {
  const adminCtx = await browser.newContext();
  const memberCtx = await browser.newContext();
  const admin = await adminCtx.newPage();
  const member = await memberCtx.newPage();
  const memberName = `t89mem_${stamp()}`;
  const bandName = `T89Shared ${stamp()}`;
  const songTitle = `T89MemSong ${stamp()}`;

  await register(admin, `t89adm_${stamp()}`);
  await createBand(admin, bandName);
  await createSong(admin, songTitle);
  await admin.goto(admin.url().replace(/\/songs\/.*$/, "")); // back to band page
  await admin.getByTestId("invite-toggle").click();
  await admin.getByTestId("invite-identifier").fill(memberName);
  await admin.getByTestId("invite-submit").click();
  await expect(admin.getByTestId("invite-notice")).toBeVisible();

  await register(member, memberName);
  await member.getByTestId("nav-invites").click();
  await member.getByTestId("invite-accept").click();
  await member.goto("/bands");
  await member.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(member.getByTestId("my-role")).toHaveText("member");
  await member.getByTestId("song-link").filter({ hasText: songTitle }).click();
  await expect(member).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);

  await member.setViewportSize({ width: 390, height: 620 });
  const panel = member.getByTestId("details-panel");
  await member.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await expect(member.getByTestId("details-tab-admin")).toHaveCount(0); // non-admin: no Admin tab
  const close = member.getByTestId("details-close");
  const box = (await close.boundingBox())!;
  const pbox = (await panel.boundingBox())!;
  expect(box.x).toBeGreaterThan(pbox.x + pbox.width / 2); // right-aligned (push-right, no Admin tab)
  await close.click();
  await expect(panel).toHaveCount(0);

  await adminCtx.close();
  await memberCtx.close();
});

test("editor: opening a file's ⋯ menu and clicking an item leaves Details open (T89)", async ({ page }) => {
  // The ⋯ menu (RowMenu) portals to <body> but lives inside the Details panel; T89's outside-click
  // must treat it as inside. This is the regression Fable's two-branch probe caught. T91's in-app
  // rename dialog is also portalled + [data-portal], so it too must not close Details.
  await register(page, `t89p_${stamp()}`);
  await createBand(page, `T89PBand ${stamp()}`);
  await createSong(page, `T89PSong ${stamp()}`);
  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel).toBeVisible();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-rename").click();
  await expect(page.getByTestId("app-dialog")).toBeVisible(); // in-app rename prompt opened
  await expect(panel).toBeVisible(); // portalled ⋯ item + [data-portal] dialog did NOT close Details
});

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

  await page.getByTestId("new-band-btn").click();
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
  await adminPage.getByTestId("new-band-btn").click();
  await adminPage.getByTestId("band-name").fill(bandName);
  await adminPage.getByTestId("create-band").click();
  await adminPage.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(adminPage.getByTestId("my-role")).toHaveText("admin");

  // Admin invites the second user by username.
  await adminPage.getByTestId("invite-toggle").click();
  await adminPage.getByTestId("invite-identifier").fill(inviteeName);
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

test("4. create a song; clicking it opens the annotation editor", async ({ page }) => {
  await register(page, `songwriter_${stamp()}`);
  const bandName = `Songs ${stamp()}`;
  const songTitle = `Tune ${stamp()}`;

  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();

  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(songTitle);
  await page.getByTestId("song-artist").fill("The Authors");
  await page.getByTestId("create-song").click();

  const songLink = page.getByTestId("song-link").filter({ hasText: songTitle });
  await expect(songLink).toBeVisible();
  await songLink.click();

  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  // The song page is the annotation editor: the tools palette is present.
  await expect(page.getByTestId("song-viewer")).toBeVisible();
  await expect(page.getByTestId("tool-rect")).toBeVisible();
});

test("5. logout redirects to /login; guarded routes redirect when logged out", async ({ page }) => {
  await register(page, `leaver_${stamp()}`);

  await page.getByTestId("account-trigger").click(); // T58: logout is a menu entry now
  await page.getByTestId("logout").click();
  await expect(page).toHaveURL(/\/login$/);

  // Visiting a guarded route while logged out → bounced to /login.
  await page.goto("/bands");
  await expect(page).toHaveURL(/\/login$/);
});

// ---- new backend features (non-canvas) ----------------------------------

/** Register, create a band, open it. Returns the band's detail URL. */
async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}

async function createSong(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}

test("6. edit song metadata persists across reload", async ({ page }) => {
  await register(page, `meta_${stamp()}`);
  await createBandAndOpen(page, `MetaBand ${stamp()}`);
  await createSong(page, `MetaSong ${stamp()}`);

  // Song metadata now lives in the editor's Details panel (opened via the Details pill);
  // it was previously a clipped-off-screen section (fix: reachable via the pill).
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("meta-key").fill("G#m");
  await page.getByTestId("meta-tempo").fill("128");
  await page.getByTestId("meta-tags").fill("rock, encore");
  await page.getByTestId("meta-notes").fill("Capo 2");
  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();

  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("meta-key")).toHaveValue("G#m");
  await expect(page.getByTestId("meta-tempo")).toHaveValue("128");
  await expect(page.getByTestId("meta-tags")).toHaveValue("rock, encore");
  await expect(page.getByTestId("meta-notes")).toHaveValue("Capo 2");
});

test("7. files list renders (empty) on the song page", async ({ page }) => {
  await register(page, `files_${stamp()}`);
  await createBandAndOpen(page, `FilesBand ${stamp()}`);
  await createSong(page, `FilesSong ${stamp()}`);
  await page.getByTestId("my-files-edit").click(); // files live in the Details panel now (T36)
  await expect(page.getByTestId("files-empty")).toBeVisible();
  await expect(page.getByTestId("file-upload-form")).toBeVisible();
});

test("8. setlist: create, add two songs, reorder, key override persists", async ({ page }) => {
  await register(page, `setlist_${stamp()}`);
  await createBandAndOpen(page, `SetBand ${stamp()}`);
  const bandUrl = page.url();

  const songA = `Alpha ${stamp()}`;
  const songB = `Bravo ${stamp()}`;
  for (const t of [songA, songB]) {
    await page.goto(bandUrl);
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill(t);
    await page.getByTestId("create-song").click();
    await expect(page.getByTestId("song-link").filter({ hasText: t })).toBeVisible();
  }

  // Go to setlists, create one, open it.
  await page.goto(bandUrl);
  await page.getByTestId("nav-setlists").click();
  await expect(page).toHaveURL(/\/setlists$/);
  await page.getByTestId("setlist-name").fill(`Gig ${stamp()}`);
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").first().click();
  await expect(page).toHaveURL(/\/setlists\/[^/]+$/);

  // Add both songs.
  await page.getByTestId("add-item-song").selectOption({ label: songA });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(1);
  await page.getByTestId("add-item-song").selectOption({ label: songB });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(2);

  // Order is A, B. Reorder → B first.
  await expect(page.getByTestId("item-title").first()).toContainText(songA);
  await page.getByTestId("item-down").first().click();
  await expect(page.getByTestId("item-title").first()).toContainText(songB);

  // Set a key override on the now-first item (songB). Per-item overrides open in
  // an inline editor (redesign) — expand it, then fill + save.
  await page.getByTestId("item-edit").first().click();
  await page.getByTestId("item-key").first().fill("Eb");
  await page.getByTestId("item-save").first().click();

  // Reload: order and override persist (re-open the editor to read the value back).
  await page.reload();
  await expect(page.getByTestId("item-title").first()).toContainText(songB);
  await page.getByTestId("item-edit").first().click();
  await expect(page.getByTestId("item-key").first()).toHaveValue("Eb");
});

test("9. band settings: admin changes a member's role; non-admin sees no controls", async ({
  browser,
}) => {
  const adminCtx = await browser.newContext();
  const memberCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();
  const memberPage = await memberCtx.newPage();

  const memberName = `member_${stamp()}`;
  const bandName = `RoleBand ${stamp()}`;

  await register(adminPage, `roleadmin_${stamp()}`);
  await createBandAndOpen(adminPage, bandName);
  const bandUrl = adminPage.url();

  // Invite the second user, they accept.
  await adminPage.getByTestId("invite-toggle").click();
  await adminPage.getByTestId("invite-identifier").fill(memberName);
  await adminPage.getByTestId("invite-submit").click();
  await expect(adminPage.getByTestId("invite-notice")).toBeVisible();

  await register(memberPage, memberName);
  await memberPage.getByTestId("nav-invites").click();
  await memberPage.getByTestId("invite-accept").click();
  await expect(memberPage.getByTestId("invites-empty")).toBeVisible();

  // Admin opens settings, promotes the member to conductor.
  await adminPage.goto(bandUrl);
  await adminPage.getByTestId("nav-settings").click();
  await expect(adminPage).toHaveURL(/\/settings$/);
  const memberRow = adminPage
    .getByTestId("settings-member-row")
    .filter({ hasText: `@${memberName}` });
  await memberRow.getByTestId("member-role-select").selectOption("conductor");

  // Reload: role persisted.
  await adminPage.reload();
  const memberRowAfter = adminPage
    .getByTestId("settings-member-row")
    .filter({ hasText: `@${memberName}` });
  await expect(memberRowAfter.getByTestId("member-role-select")).toHaveValue("conductor");

  // The member (non-admin) sees no settings link and no role controls.
  await memberPage.goto(bandUrl);
  await expect(memberPage.getByTestId("nav-settings")).toHaveCount(0);
  // Visiting settings directly: no role selects (non-admin).
  await memberPage.goto(bandUrl + "/settings");
  await expect(memberPage.getByTestId("settings-title")).toBeVisible();
  await expect(memberPage.getByTestId("member-role-select")).toHaveCount(0);

  await adminCtx.close();
  await memberCtx.close();
});

test("10. admin changes a conductor's role; member-list order stays stable", async ({
  browser,
}) => {
  const adminCtx = await browser.newContext();
  const adminPage = await adminCtx.newPage();

  const bandName = `OrderBand ${stamp()}`;
  await register(adminPage, `orderadmin_${stamp()}`);
  await createBandAndOpen(adminPage, bandName);
  const bandUrl = adminPage.url();

  // Invite three members, each accepts (separate contexts).
  const names = [`leo_${stamp()}`, `anya_${stamp()}`, `bob_${stamp()}`];
  const ctxs = [];
  for (const name of names) {
    await adminPage.goto(bandUrl);
    await adminPage.getByTestId("invite-toggle").click();
    await adminPage.getByTestId("invite-identifier").fill(name);
    await adminPage.getByTestId("invite-submit").click();
    await expect(adminPage.getByTestId("invite-notice")).toBeVisible();

    const ctx = await browser.newContext();
    const page = await ctx.newPage();
    await register(page, name);
    await page.getByTestId("nav-invites").click();
    await page.getByTestId("invite-accept").click();
    await expect(page.getByTestId("invites-empty")).toBeVisible();
    ctxs.push(ctx);
  }

  await adminPage.goto(bandUrl);
  await adminPage.getByTestId("nav-settings").click();
  await expect(adminPage).toHaveURL(/\/settings$/);

  // Capture the rendered member order (by @username), then read it again after
  // each role change — it must never reorder (#6).
  // Wait for the full members list (admin + 3) to render before reading order.
  await expect(adminPage.getByTestId("settings-member-row")).toHaveCount(4);

  const orderUsernames = async () => {
    const rows = adminPage.getByTestId("settings-member-row");
    const count = await rows.count();
    const out: string[] = [];
    for (let i = 0; i < count; i++) {
      out.push((await rows.nth(i).innerText()).match(/@\S+/)?.[0] ?? "");
    }
    return out;
  };

  const before = await orderUsernames();
  expect(before.length).toBe(4); // admin + 3 members

  const leoRow = adminPage
    .getByTestId("settings-member-row")
    .filter({ hasText: `@${names[0]}` });

  // Promote Leo TO conductor, then change him AGAIN while he IS conductor (the
  // seeded-Leo "failed to change role" case — must succeed, no error banner).
  await leoRow.getByTestId("member-role-select").selectOption("conductor");
  await expect(leoRow.getByTestId("member-role-select")).toHaveValue("conductor");
  expect(await orderUsernames()).toEqual(before);

  // FROM conductor -> admin (this is the transition the bug report hits).
  await leoRow.getByTestId("member-role-select").selectOption("admin");
  await expect(leoRow.getByTestId("member-role-select")).toHaveValue("admin");
  expect(await orderUsernames()).toEqual(before);

  // FROM admin -> member, then back to conductor: still no reorder, no error.
  await leoRow.getByTestId("member-role-select").selectOption("member");
  await expect(leoRow.getByTestId("member-role-select")).toHaveValue("member");
  await leoRow.getByTestId("member-role-select").selectOption("conductor");
  await expect(leoRow.getByTestId("member-role-select")).toHaveValue("conductor");
  expect(await orderUsernames()).toEqual(before);

  // No error banner ever appeared.
  await expect(adminPage.getByText("Failed to change role")).toHaveCount(0);

  // Persists across reload, order still stable.
  await adminPage.reload();
  await expect(adminPage.getByTestId("settings-member-row")).toHaveCount(4);
  expect(await orderUsernames()).toEqual(before);

  await adminCtx.close();
  for (const ctx of ctxs) await ctx.close();
});

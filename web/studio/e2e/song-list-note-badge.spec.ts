/**
 * T173 — the band's song list flags which songs carry a rehearsal note and puts them first.
 *
 * The point is the WORKLIST: a note exists to be recopied into a real annotation and then deleted
 * (T170 §1), and until this nothing in Studio said which songs had one waiting. So the assertions are
 * about ORDER and about the badge being a pointer, not a control.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createBandAndOpen, createSongAndOpen, register, stamp, uploadPdf } from "./setup-helpers";

const NOTE_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));

async function putNote(page: Page, bandId: string, songId: string, pageIndex: number) {
  const res = await page.request.fetch(
    `/api/bands/${bandId}/songs/${songId}/rehearsal-notes/${pageIndex}`,
    {
      method: "PUT",
      multipart: {
        file: { name: "note.png", mimeType: "image/png", buffer: readFileSync(NOTE_PATH) },
        rasterHash: "rh",
        concertId: "c",
        concertRev: "1",
        takenAs: "m",
        width: "1600",
        height: "2261",
      },
    },
  );
  expect(res.status()).toBe(200);
}

test("a song carrying a note is badged and sorted to the top of the band's song list", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `badge${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);

  // three songs, created in a known order; the LAST one gets the note so "first" cannot be an accident
  await createSongAndOpen(page, `Alpha ${who}`);
  await page.goto(band.url);
  await createSongAndOpen(page, `Bravo ${who}`);
  await page.goto(band.url);
  const noted = await createSongAndOpen(page, `Charlie ${who}`);
  await uploadPdf(page);

  await page.goto(band.url);
  // before any note: no badge anywhere, and the creation order stands
  await expect(page.getByTestId("song-note-badge")).toHaveCount(0);
  // ⟨D6⟩ — and the empty list SAYS it is empty rather than looking the same as a failed lookup
  await expect(page.getByTestId("note-scope-note")).toContainText("No rehearsal notes waiting in Studio");
  const before = await page.getByTestId("song-link").allInnerTexts();
  expect(before[0]).toContain("Alpha");

  await putNote(page, band.id, noted, 0);
  await page.reload();

  const badge = page.getByTestId("song-note-badge");
  await expect(badge).toHaveCount(1);
  await expect(badge).toHaveText(/1/);
  // ⟨D6⟩ the hover text says WHERE the note is — "2 notes" alone is ambiguous between tablet and Studio
  const title = await badge.getAttribute("title");
  expect(title).toContain("Studio");

  // ⟨D2⟩ sorted to the top — the row that was third is now first
  const rows = await page.getByTestId("song-link").allInnerTexts();
  expect(rows[0]).toContain("Charlie");
  expect(rows).toHaveLength(3); // ordering only: nothing is hidden, unlike a filter

  // with a note present the line changes register — it stops claiming there is nothing waiting
  await expect(page.getByTestId("note-scope-note")).toContainText("marked");
  await expect(page.getByTestId("note-scope-note")).toContainText("tablet");

  // ⟨D5⟩ the badge is a pointer into the editor, where the underlay already lives
  await page.getByTestId("song-link").first().click();
  await expect(page).toHaveURL(new RegExp(`/songs/${noted}$`));
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();
});

test("the count follows the notes, and Done-remove clears the badge", async ({ page }) => {
  const who = stamp();
  await register(page, `badge2${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Solo ${who}`);
  await uploadPdf(page);

  await putNote(page, band.id, songId, 0);
  await putNote(page, band.id, songId, 1);
  await page.goto(band.url);
  await expect(page.getByTestId("song-note-badge")).toHaveText(/2/);

  const del = await page.request.fetch(`/api/bands/${band.id}/songs/${songId}/rehearsal-notes/1`, {
    method: "DELETE",
  });
  expect(del.status()).toBe(204);
  await page.reload();
  await expect(page.getByTestId("song-note-badge")).toHaveText(/1/);

  const del0 = await page.request.fetch(`/api/bands/${band.id}/songs/${songId}/rehearsal-notes/0`, {
    method: "DELETE",
  });
  expect(del0.status()).toBe(204);
  await page.reload();
  // gone entirely, not a zero: the badge is absent when there is nothing waiting
  await expect(page.getByTestId("song-note-badge")).toHaveCount(0);
});

test("a failed lookup is not reported as an empty one", async ({ page }) => {
  const who = stamp();
  await register(page, `fail${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  await createSongAndOpen(page, `Only ${who}`);

  // The degrade is deliberate — a song list must open whether or not this call answered — but it must
  // not LOOK like "nothing waiting", which is the one conclusion a musician would act on.
  await page.route("**/api/bands/*/rehearsal-notes", (r) => r.abort());
  await page.goto(band.url);
  await expect(page.getByTestId("songs-list")).toBeVisible(); // the list still opens
  await expect(page.getByTestId("note-scope-note")).toContainText("Couldn’t check");
  await expect(page.getByTestId("note-scope-note")).not.toContainText("No rehearsal notes waiting");
});

test("another member's notes put no badge on the list", async ({ page, browser }) => {
  const who = stamp();
  await register(page, `owner${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Shared ${who}`);
  await uploadPdf(page);
  await putNote(page, band.id, songId, 0);

  const inv = await page.request.post(`/api/bands/${band.id}/invite-links`, {
    data: { role: "member" },
  });
  const token = (await inv.json()).token as string;

  const ctx = await browser.newContext();
  const other = await ctx.newPage();
  await register(other, `other${who}`);
  const join = await other.request.post(`/api/invite-links/${token}/accept`, { data: {} });
  expect(join.ok()).toBeTruthy();
  await other.goto(band.url);
  await expect(other.getByTestId("song-link")).toHaveCount(1);
  // a badge here would nag them about work that is not theirs, on a note they cannot even open
  await expect(other.getByTestId("song-note-badge")).toHaveCount(0);
  await ctx.close();
});

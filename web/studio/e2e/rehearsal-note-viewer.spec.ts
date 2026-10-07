/**
 * T186 — the rehearsal-note viewer. The real-background composite (sha256 of the background == the note's
 * rasterHash) is pinned by the Go property test and the live :8080 check (a baked hash can't be forged in
 * this mem-store e2e without a zip reader for the .tstage). This spec pins what the browser owns: the dialog
 * opens from the row, the no-background / "strokes alone" path (a note with no matching bake → 404), paging
 * and Escape with focus returned to the row, and the owner-only background endpoint.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createBandAndOpen, createSongAndOpen, register, stamp } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));
const NOTE_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));
const noteImage = () => readFileSync(NOTE_PATH);

async function openSongWithPdf(page: Page) {
  const who = stamp();
  await register(page, `nv${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  return { bandId: band.id, songId };
}

async function putNote(page: Page, bandId: string, songId: string, pageIndex: number) {
  return page.request.fetch(`/api/bands/${bandId}/songs/${songId}/rehearsal-notes/${pageIndex}`, {
    method: "PUT",
    multipart: {
      file: { name: "note.png", mimeType: "image/png", buffer: noteImage() },
      rasterHash: `fake-hash-${pageIndex}`, // matches no bake → background 404 (the purged/"strokes alone" case)
      concertId: "concert-e2e",
      concertRev: "8",
      takenAs: "member-e2e",
      width: "1600",
      height: "2261",
    },
  });
}

test("the viewer opens from the row, shows strokes-alone when no background, pages, and Escape returns focus", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);
  expect((await putNote(page, bandId, songId, 1)).status()).toBe(200);
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();

  // Open the list and the viewer from the first row's View button.
  await page.getByTestId("rehearsal-notes-more").click();
  const viewBtns = page.getByTestId("rehearsal-note-view");
  await viewBtns.first().click();

  const dlg = page.getByTestId("note-viewer");
  await expect(dlg).toBeVisible();
  await expect(dlg).toHaveAttribute("role", "dialog");
  // These notes carry a hash no bake has → the background 404s → strokes alone, said plainly.
  await expect(page.getByTestId("note-viewer-info")).toHaveText(
    "The page this note was drawn on is no longer available. Showing the strokes alone.",
  );
  await expect(page.getByTestId("note-viewer-bg")).toHaveCount(0);
  await expect(page.getByTestId("note-viewer-note")).toBeVisible(); // the strokes are there
  await expect(page.getByTestId("note-viewer-count")).toHaveText("1 / 2");

  // Page to the next note, then back.
  await page.getByTestId("note-viewer-next").click();
  await expect(page.getByTestId("note-viewer-count")).toHaveText("2 / 2");
  await expect(page.getByTestId("note-viewer-next")).toBeDisabled();
  await page.keyboard.press("ArrowLeft");
  await expect(page.getByTestId("note-viewer-count")).toHaveText("1 / 2");

  // Escape closes and returns focus to the row's View button (the popover stayed open behind the modal).
  await page.keyboard.press("Escape");
  await expect(page.getByTestId("note-viewer")).toHaveCount(0);
  await expect(viewBtns.first()).toBeFocused();
});

test("the background endpoint 404s for a note with no bake, and refuses a non-owner", async ({
  page,
  browser,
}) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);

  // No bake for this concert → the background is a 404, which is how the viewer falls to strokes-alone.
  const bg = await page.request.get(
    `/api/bands/${bandId}/songs/${songId}/rehearsal-notes/0/background`,
  );
  expect(bg.status()).toBe(404);

  // A second band member must not be able to fetch someone else's note background.
  const inv = await page.request.post(`/api/bands/${bandId}/invite-links`, { data: { role: "member" } });
  const token = (await inv.json()).token as string;
  const ctx = await browser.newContext();
  const other = await ctx.newPage();
  await register(other, `nvother${stamp()}`);
  await other.request.post(`/api/invite-links/${token}/accept`, { data: {} });
  const theirs = await other.request.get(
    `/api/bands/${bandId}/songs/${songId}/rehearsal-notes/0/background`,
  );
  expect(theirs.status(), "another member's note background must be 404, not 403").toBe(404);
  await ctx.close();
});

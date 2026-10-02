/**
 * T183 — "Go to page N" on a rehearsal-note popover row. The value is only visible on a LONG chart, so the
 * fixture is an 8-page PDF and the test asserts the note's page starts OUT of the viewport before the click;
 * on a short chart the button would appear to work while doing nothing.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { createBandAndOpen, createSongAndOpen, register, stamp } from "./setup-helpers";

const LONG_PDF = fileURLToPath(new URL("./fixtures/long-chart.pdf", import.meta.url));
const NOTE_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));
const noteImage = () => readFileSync(NOTE_PATH);

async function openSongWithLongPdf(page: Page) {
  const who = stamp();
  await register(page, `goto${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(LONG_PDF);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  // Sanity: the fixture really is long.
  await expect(page.getByTestId("pdf-page")).toHaveCount(8);
  return { bandId: band.id, songId };
}

const IMG_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));

async function openSongWithImage(page: Page) {
  const who = stamp();
  await register(page, `img${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(IMG_PATH);
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
      rasterHash: "raster-hash-from-the-tablet",
      concertId: "concert-e2e",
      concertRev: "8",
      takenAs: "member-e2e",
      width: "1600",
      height: "2261",
    },
  });
}

test("Go to page N centres the note's page, turns the underlay on, and closes the popover", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithLongPdf(page);
  expect((await putNote(page, bandId, songId, 7)).status()).toBe(200); // note on the LAST page (8)
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();

  const lastPage = page.getByTestId("pdf-page").nth(7);
  // The fixture must be long enough that the note's page is NOT already on screen, or this proves nothing.
  await expect(lastPage).not.toBeInViewport();

  // Underlay starts OFF; open the list and go to the note's page.
  await page.getByTestId("rehearsal-notes-more").click();
  const row = page.getByTestId("rehearsal-note-row");
  await expect(row.getByTestId("rehearsal-note-goto")).toHaveText("Go to page 8");
  await row.getByTestId("rehearsal-note-goto").click();

  // The popover closed…
  await expect(page.getByTestId("rehearsal-notes-popover")).toHaveCount(0);
  // …the page is now on screen…
  await expect(lastPage).toBeInViewport();
  // …with its underlay shown (Go turned it on).
  await expect(page.getByTestId("rehearsal-notes-chip")).toHaveAttribute("aria-pressed", "true");
  await expect(lastPage.getByTestId("rehearsal-underlay")).toBeVisible();
});

test("a note whose page is beyond the open file disables Go, and says why", async ({ page }) => {
  const { bandId, songId } = await openSongWithLongPdf(page); // 8 pages → indices 0..7
  expect((await putNote(page, bandId, songId, 9)).status()).toBe(200); // page 10, not in this file
  await page.reload();

  await page.getByTestId("rehearsal-notes-more").click();
  const goto = page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto");
  await expect(goto).toHaveText("Go to page 10");
  await expect(goto).toBeDisabled();
  await expect(goto).toHaveAttribute("title", "This file has no page 10");
});

test("T183 ⟨1⟩: on an IMAGE chart a page-1 note is enabled (usePdfDocument reports 0 pages, but the viewer draws it)", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithImage(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200); // page 1 of the image
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();

  await page.getByTestId("rehearsal-notes-more").click();
  const goto = page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto");
  await expect(goto).toHaveText("Go to page 1");
  await expect(goto).not.toBeDisabled();
  await goto.click();
  // The image page is on screen with its underlay shown, and the popover closed.
  await expect(page.getByTestId("rehearsal-notes-popover")).toHaveCount(0);
  await expect(page.getByTestId("pdf-page").first()).toBeInViewport();
  await expect(page.getByTestId("rehearsal-underlay").first()).toBeVisible();
});

test("T183 ⟨1⟩: on an IMAGE chart a page-2 note is disabled — the image has one page", async ({ page }) => {
  const { bandId, songId } = await openSongWithImage(page);
  expect((await putNote(page, bandId, songId, 1)).status()).toBe(200); // page 2 — not in a one-page image
  await page.reload();

  await page.getByTestId("rehearsal-notes-more").click();
  const goto = page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto");
  await expect(goto).toHaveText("Go to page 2");
  await expect(goto).toBeDisabled();
  await expect(goto).toHaveAttribute("title", "This file has no page 2");
});

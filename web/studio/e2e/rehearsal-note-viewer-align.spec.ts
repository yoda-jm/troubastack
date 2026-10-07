/**
 * T186 fix — the note must sit EXACTLY over the background page, not float high/low on it (VLL). We can't
 * forge a real baked background in the mem-store e2e, so we intercept the /background request and serve a
 * known page image (same dimensions as the note), then MEASURE: the note image's box must coincide with the
 * background image's box. This is the alignment property, measured in real pixels.
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
  await register(page, `align${who}`);
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
      rasterHash: "align-hash",
      concertId: "concert-e2e",
      concertRev: "8",
      takenAs: "member-e2e",
      width: "1600",
      height: "2261",
    },
  });
}

test("the note image sits exactly over the background page", async ({ page }) => {
  const { bandId, songId } = await openSongWithPdf(page);
  expect((await putNote(page, bandId, songId, 0)).status()).toBe(200);

  // Serve a known page image for the background (same dimensions as the note), so the two boxes must coincide.
  await page.route("**/rehearsal-notes/0/background**", (route) =>
    route.fulfill({ status: 200, contentType: "image/png", body: noteImage() }),
  );

  await page.reload();
  await page.getByTestId("rehearsal-notes-more").click();
  await page.getByTestId("rehearsal-note-view").first().click();
  await expect(page.getByTestId("note-viewer")).toBeVisible();

  const bg = page.getByTestId("note-viewer-bg");
  const note = page.getByTestId("note-viewer-note");
  await expect(bg).toBeVisible();
  await expect(note).toBeVisible();

  const b = await bg.boundingBox();
  const nb = await note.boundingBox();
  if (!b || !nb) throw new Error("missing boxes");
  // The strokes must land on the page: same rectangle, within a pixel of rounding.
  expect(Math.abs(nb.x - b.x), `x off by ${nb.x - b.x}`).toBeLessThanOrEqual(1);
  expect(Math.abs(nb.y - b.y), `y off by ${nb.y - b.y}`).toBeLessThanOrEqual(1);
  expect(Math.abs(nb.width - b.width), `w off by ${nb.width - b.width}`).toBeLessThanOrEqual(1);
  expect(Math.abs(nb.height - b.height), `h off by ${nb.height - b.height}`).toBeLessThanOrEqual(1);
});

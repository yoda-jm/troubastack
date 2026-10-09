/**
 * FILEPDF — "Download PDF" for the file being viewed, from the "This file" rail.
 *
 * The PDF endpoint itself is MOCKED (page.route — the bake-progress / lyrics-search precedent): the e2e
 * job has no poppler, and the server's rasterize + compositing + layer selection are covered by the Go
 * tests (internal/bake/filepdf_test.go, internal/httpapi/filepdf_test.go). What this pins is the Studio
 * half: the button sends EXACTLY the layers shown on screen (a toggled-off layer is left out), and the
 * response lands as a download under the server's filename.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function myUserId(page: Page): Promise<string> {
  return page.evaluate(async (): Promise<string> => {
    const r = await fetch("/api/me", { credentials: "include" });
    return ((await r.json()) as { user: { id: string } }).user.id;
  });
}

async function firstFileId(page: Page, bandId: string, songId: string): Promise<string> {
  return page.evaluate(
    async ([b, s]): Promise<string> => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      return ((await r.json()) as { files: { id: string }[] }).files[0].id;
    },
    [bandId, songId],
  );
}

test("downloads the viewed file as a PDF with the layers shown on screen", async ({ page }) => {
  await register(page, `fpdf_${stamp()}`);
  const band = await createBandAndOpen(page, `FpdfBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `FpdfSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);

  // Two personal layers of mine on this file — both visible by default.
  const layer = (id: string, name: string, order: number) => ({
    id, fileId, name, ownerId: me, zone: "personal", order, access: "rw", mandatory: false, roleTag: "",
  });
  const imported = await page.evaluate(
    async ([b, s, body]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return r.ok;
    },
    [band.id, songId, { layers: [layer("fpdf-keep", "Keep me", 0), layer("fpdf-hide", "Hide me", 1)], objects: [] }] as const,
  );
  expect(imported).toBeTruthy();
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();

  // Mock the print: capture the request, answer with a tiny PDF under a server-chosen name.
  let requested = "";
  await page.route("**/api/bands/*/songs/*/files/*/pdf*", async (route) => {
    requested = route.request().url();
    await route.fulfill({
      status: 200,
      headers: {
        "Content-Type": "application/pdf",
        "Content-Disposition": `attachment; filename="FpdfSong - sample.pdf"; filename*=UTF-8''FpdfSong%20-%20sample.pdf`,
      },
      body: "%PDF-1.4\n%mock\n%%EOF\n",
    });
  });

  await openDrawer(page, "layers");
  // Hide one layer on screen → it must not be asked for.
  await page.getByTestId("layer-item").filter({ hasText: "Hide me" }).getByTestId("layer-toggle").uncheck();

  const btn = page.getByTestId("file-pdf-download");
  await expect(btn).toHaveAttribute("title", "Download this file as a PDF, with the layers shown on screen");
  // A plain pointer click right after a toggle: the button must not move under the press (it sits above the
  // layer list, whose toggled checkbox re-lays out on blur — below the list this click used to miss).
  const downloadPromise = page.waitForEvent("download");
  await btn.click();
  const download = await downloadPromise;

  expect(download.suggestedFilename()).toBe("FpdfSong - sample.pdf");
  const body = readFileSync((await download.path())!);
  expect(body.length).toBeGreaterThan(0);
  expect(body.subarray(0, 5).toString()).toBe("%PDF-");

  const url = new URL(requested);
  expect(url.pathname).toBe(`/api/bands/${band.id}/songs/${songId}/files/${fileId}/pdf`);
  expect(url.searchParams.get("layers")).toBe("fpdf-keep");
  await expect(btn).toBeEnabled();
});

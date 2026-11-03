/**
 * T208 — download a song file from its ⋯ menu, with or without annotations.
 *
 * The FILEPDF endpoint is MOCKED (page.route — the file-pdf.spec precedent): the e2e job has no poppler,
 * and the server's rasterize/composite/layer-selection is covered by the Go tests. What this pins is the
 * Studio half: the ⋯ menu's download entry opens the dialog; "With annotations" sends EXACTLY the ticked
 * layers (role-tagged shared INCLUDED — the discriminating vector) and lands the response as a download;
 * "Without annotations" downloads the stored bytes themselves (equal to GET /api/files/{id}) and opens no
 * new tab; and an image row downloads straight, with no dialog.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { readFileSync } from "node:fs";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

const PNG_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));

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

test("downloads a file from its ⋯ menu — with annotations, then as uploaded", async ({ page }) => {
  await register(page, `dl_${stamp()}`);
  const band = await createBandAndOpen(page, `DlBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `DlSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);

  // Two layers of THIS file: a SHARED layer carrying a role tag (defaultVisibility ticks it; the
  // server's absent-param default would drop it) and my own personal layer. Both tick by default.
  const layer = (id: string, name: string, zone: string, ownerId: string, roleTag: string, order: number) => ({
    id, fileId, name, ownerId, zone, order, access: "rw", mandatory: false, roleTag,
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
    [
      band.id,
      songId,
      {
        layers: [
          layer("dl-role", "Bass cues", "shared", "u-cond", "bass", 0),
          layer("dl-mine", "My notes", "personal", me, "", 1),
        ],
        objects: [],
      },
    ] as const,
  );
  expect(imported).toBeTruthy();
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();

  // Mock the composited print: capture the request, answer with a tiny PDF under a server-chosen name.
  let requested = "";
  await page.route("**/api/bands/*/songs/*/files/*/pdf*", async (route) => {
    requested = route.request().url();
    await route.fulfill({
      status: 200,
      headers: {
        "Content-Type": "application/pdf",
        "Content-Disposition": `attachment; filename="DlSong - sample.pdf"; filename*=UTF-8''DlSong%20-%20sample.pdf`,
      },
      body: "%PDF-1.4\n%mock\n%%EOF\n",
    });
  });

  // No stray tab may open from any download.
  let popped = false;
  page.on("popup", () => {
    popped = true;
  });

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  // ⋯ → the download entry is FIRST and, on a PDF, opens the dialog.
  await panel.getByTestId("file-menu").click();
  const entry = page.getByTestId("file-menu-download");
  await expect(entry).toHaveText("Download PDF…");
  await entry.click();
  await expect(page.getByTestId("download-dialog")).toBeVisible();

  // "With annotations" is the default; its checklist carries both of this file's layers, ticked.
  await expect(page.getByTestId("download-choice-with")).toBeChecked();
  await expect(page.getByTestId("download-layer")).toHaveCount(2);

  const d1 = page.waitForEvent("download");
  await page.getByTestId("download-dialog-confirm").click();
  const withDl = await d1;
  expect(withDl.suggestedFilename()).toBe("DlSong - sample.pdf");
  await expect(page.getByTestId("download-dialog")).toHaveCount(0); // closed on success

  // The routed request sent `layers=` explicitly, with the role-tagged shared id INCLUDED.
  const url = new URL(requested);
  expect(url.pathname).toBe(`/api/bands/${band.id}/songs/${songId}/files/${fileId}/pdf`);
  const sent = (url.searchParams.get("layers") ?? "").split(",").filter(Boolean);
  expect(new Set(sent)).toEqual(new Set(["dl-role", "dl-mine"]));

  // Reopen the dialog → "Without annotations" → Download: the STORED bytes, equal to GET /api/files/{id}.
  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-download").click();
  await expect(page.getByTestId("download-dialog")).toBeVisible();
  await page.getByTestId("download-choice-without").check();
  const d2 = page.waitForEvent("download");
  await page.getByTestId("download-dialog-confirm").click();
  const plainDl = await d2;

  const got = readFileSync((await plainDl.path())!);
  const wantB64 = await page.evaluate(async (id) => {
    const r = await fetch(`/api/files/${id}`, { credentials: "include" });
    const buf = new Uint8Array(await r.arrayBuffer());
    let s = "";
    for (const b of buf) s += String.fromCharCode(b);
    return btoa(s);
  }, fileId);
  expect(got.equals(Buffer.from(wantB64, "base64"))).toBe(true);

  expect(popped).toBe(false); // no download ever opened a new tab
});

test("the dialog's layer rows are one line, checkboxes aligned in a left column", async ({ page }) => {
  await register(page, `dlrows_${stamp()}`);
  const band = await createBandAndOpen(page, `DlRowsBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `DlRowsSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);

  // Three layers of this file, including a mandatory one and a very long name (ellipsis path).
  const layer = (id: string, name: string, zone: string, owner: string, order: number, mandatory = false) => ({
    id, fileId, name, ownerId: owner, zone, order, access: "rw", mandatory, roleTag: "",
  });
  await page.evaluate(
    async ([b, s, body]) => {
      await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST", credentials: "include", headers: { "Content-Type": "application/json" }, body: JSON.stringify(body),
      });
    },
    [band.id, songId, { layers: [
      layer("row-a", "A very long annotation layer name that should ellipsise before it reaches the edge", "shared", "u-cond", 0),
      layer("row-b", "Form", "conductor", "u-cond", 1),
      layer("row-c", "Set list", "shared", "u-cond", 2, true),
    ], objects: [] }] as const,
  );
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await panel.getByTestId("file-row").first().waitFor();
  await panel.getByTestId("file-menu").click();
  await page.getByTestId("file-menu-download").click();
  await expect(page.getByTestId("download-dialog")).toBeVisible();

  const rows = page.getByTestId("download-layer");
  const n = await rows.count();
  expect(n).toBe(3);
  let firstLeft: number | null = null;
  for (let i = 0; i < n; i++) {
    const row = rows.nth(i);
    const cb = (await row.getByTestId("download-layer-toggle").boundingBox())!;
    const name = (await row.getByTestId("download-layer-name").boundingBox())!;
    expect(cb && name).toBeTruthy();
    const cbMid = cb.y + cb.height / 2;
    // the checkbox's vertical centre lies inside the name's box (one line, not stacked)
    expect(cbMid).toBeGreaterThanOrEqual(name.y - 0.5);
    expect(cbMid).toBeLessThanOrEqual(name.y + name.height + 0.5);
    // the checkbox's right edge is left of the name's left edge (checkbox first, then name)
    expect(cb.x + cb.width).toBeLessThanOrEqual(name.x + 0.5);
    // every row's checkbox shares the same left x, ±1 px (one left column)
    if (firstLeft === null) firstLeft = cb.x;
    else expect(Math.abs(cb.x - firstLeft)).toBeLessThanOrEqual(1);
  }
});

test("an image row downloads straight from the ⋯ menu, with no dialog", async ({ page }) => {
  await register(page, `dlimg_${stamp()}`);
  await createBandAndOpen(page, `DlImgBand ${stamp()}`);
  await createSongAndOpen(page, `DlImgSong ${stamp()}`);

  const panel = page.getByTestId("details-panel");
  await page.getByTestId("my-files-edit").click();
  await panel.getByTestId("file-input").setInputFiles(PNG_PATH);
  await panel.getByTestId("file-upload").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);

  await panel.getByTestId("file-menu").click();
  const entry = page.getByTestId("file-menu-download");
  await expect(entry).toHaveText("Download image");

  const dl = page.waitForEvent("download");
  await entry.click();
  await dl; // a real download fired immediately — no dialog
  await expect(page.getByTestId("download-dialog")).toHaveCount(0);
});

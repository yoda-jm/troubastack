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

// Drive the raster seam (usePdfDocument's window.__troubaE2eRaster, a no-op in production) on EVERY run, so the "pages
// still sizing below the goto target" race fires here instead of only when CI is slow. `delayMs` slows each page's
// sizing (the uniform race). `hold` PARKS those pages' sizing until the test calls release(i) — so the out-of-order
// test drives a deterministic order rather than racing timers. Call BEFORE page.reload().
async function armRasterSeam(page: Page, cfg: { delayMs?: number; hold?: number[] }) {
  await page.addInitScript((c) => {
    const held = new Set(c.hold ?? []);
    const sizers = new Map<number, () => void>();
    (window as unknown as { __troubaE2eRaster?: unknown }).__troubaE2eRaster = {
      delayMs: c.delayMs,
      isHeld: (i: number) => held.has(i),
      register: (i: number, size: () => void) => void sizers.set(i, size),
      release: (i: number) => {
        held.delete(i);
        const size = sizers.get(i);
        if (size) {
          sizers.delete(i);
          size();
        }
      },
    };
  }, cfg);
}
// Release a held page's sizing and wait until its display size has actually landed.
async function releasePage(page: Page, i: number) {
  await page.evaluate((idx) => {
    (window as unknown as { __troubaE2eRaster?: { release(i: number): void } }).__troubaE2eRaster?.release(idx);
  }, i);
  await page.waitForFunction((idx) => {
    const c = document.querySelector('[data-testid="viewer-scroll"]')?.querySelectorAll<HTMLElement>(".pdf-canvas")[idx];
    return !!c && c.style.height !== "";
  }, i);
}
// The goto computes an EXACT centred scroll (scrollPageIntoView aligns the target page's centre with the viewer's
// centre), so assert that precisely: |page centre − viewer centre| ≤ 24 px. Precise, not "box contains the midpoint"
// — that is loose by ±half a page for a page taller than the viewport, so even a half-page-off landing would pass it.
// First wait for the scroll to SETTLE (two readings a frame apart that don't move), so we read the final position and
// not a frame mid-smooth-scroll where the page is merely passing through the centre.
async function expectCentred(page: Page, targetIndex: number) {
  // First let every page finish sizing — otherwise the scroll is still being re-centred as the column grows and the
  // settle check below can catch a brief rest mid-way (far off centre).
  await page.waitForFunction(() => {
    const cs = document.querySelector('[data-testid="viewer-scroll"]')?.querySelectorAll<HTMLElement>(".pdf-canvas");
    return !!cs && cs.length > 0 && Array.from(cs).every((c) => c.style.height !== "");
  });
  await page.evaluate(() => ((window as unknown as { __st?: number }).__st = undefined));
  await page.waitForFunction(
    () => {
      const sc = document.querySelector<HTMLElement>('[data-testid="viewer-scroll"]');
      if (!sc) return false;
      const w = window as unknown as { __st?: number };
      const prev = w.__st;
      w.__st = sc.scrollTop;
      return prev !== undefined && Math.abs(prev - sc.scrollTop) < 1;
    },
    undefined,
    { polling: 150, timeout: 10_000 },
  );
  const dist = await page.evaluate((idx) => {
    const sc = document.querySelector<HTMLElement>('[data-testid="viewer-scroll"]');
    const el = sc?.querySelectorAll<HTMLElement>(".pdf-page")[idx];
    if (!sc || !el) return Number.POSITIVE_INFINITY;
    const sr = sc.getBoundingClientRect();
    const r = el.getBoundingClientRect();
    return Math.abs(r.top + r.height / 2 - (sr.top + sr.height / 2));
  }, targetIndex);
  expect(dist).toBeLessThanOrEqual(24);
}
test.beforeEach(async ({ page }) => {
  await armRasterSeam(page, { delayMs: 300 });
});

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

// Two long files on one song (A then B), for the cross-file goto. Returns both file ids.
async function openSongWithTwoLongPdfs(page: Page) {
  const who = stamp();
  await register(page, `xf${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  const songId = await createSongAndOpen(page, `Song ${who}`);
  for (let k = 0; k < 2; k++) {
    await page.getByTestId("my-files-edit").click(); // open the files editor
    await page.getByTestId("file-input").setInputFiles(LONG_PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(k + 1);
    await page.getByTestId("my-files-edit").click(); // close it again
  }
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  const files = (await page.request.get(`/api/bands/${band.id}/songs/${songId}/files`).then((r) => r.json()))
    .files as { id: string }[];
  return { bandId: band.id, songId, fileA: files[0].id, fileB: files[1].id };
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
  // …the page is CENTRED (the feature's promise — not merely some part of it in view)…
  await expectCentred(page, 7);
  // …with its underlay shown (Go turned it on).
  await expect(page.getByTestId("rehearsal-notes-chip")).toHaveAttribute("aria-pressed", "true");
  await expect(lastPage.getByTestId("rehearsal-underlay")).toBeVisible();
});

test("Go to page N waits on the REAL per-page size: a page above the target that sizes LATE (out of order) still lands the target", async ({
  page,
}) => {
  const { bandId, songId } = await openSongWithLongPdf(page);
  expect((await putNote(page, bandId, songId, 7)).status()).toBe(200);
  // Hold EVERY page's sizing, then release in a controlled order so the sequence is certain in every run (no racing
  // timers). Page 1 — above the target — is released LAST, out of top-down order.
  await armRasterSeam(page, { hold: [0, 1, 2, 3, 4, 5, 6, 7] });
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();
  await expect(page.getByTestId("pdf-page")).toHaveCount(8);

  // Press Go while every page is still a placeholder — the re-centre is now active.
  await page.getByTestId("rehearsal-notes-more").click();
  await page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto").click();
  await expect(page.getByTestId("rehearsal-notes-popover")).toHaveCount(0);

  // Size every page EXCEPT page 1, top-down. After page 7's own size its offsetTop no longer moves (page 1 is still a
  // placeholder), so an offset-stability proxy takes its unchanged-offset tick HERE and STOPS.
  for (const i of [0, 2, 3, 4, 5, 6, 7]) await releasePage(page, i);

  // Now release page 1 — above the target, out of top-down order. It shifts the target down by its height. The real
  // condition kept the re-centre alive (page 1 is a lower index, still unsized), so it re-centres on page 1's sizing
  // and lands the target back on centre; the proxy, having stopped, leaves it ~a page off, and a one-shot never did.
  // (The precise centre assertion catches that one-page miss — no need to pile up several late pages.)
  await releasePage(page, 1);
  await expectCentred(page, 7);
});

test("Go to page N yields to the person's own scroll during the settle", async ({ page }) => {
  const { bandId, songId } = await openSongWithLongPdf(page);
  expect((await putNote(page, bandId, songId, 7)).status()).toBe(200);
  // Hold every page so the settle is under the test's control (no racing timers), and the person's scroll lands
  // squarely inside it.
  await armRasterSeam(page, { hold: [0, 1, 2, 3, 4, 5, 6, 7] });
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();
  await expect(page.getByTestId("pdf-page")).toHaveCount(8);

  const scroll = page.getByTestId("viewer-scroll");
  await page.getByTestId("rehearsal-notes-more").click();
  await page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto").click();
  await expect(page.getByTestId("rehearsal-notes-popover")).toHaveCount(0);

  // Mid-settle (every page still a placeholder, the goto re-centring active), the person scrolls to the TOP with a
  // REAL wheel — which interrupts our in-flight smooth scroll and fires the cancel.
  await scroll.hover();
  await page.mouse.wheel(0, -20000);

  // Now size every page. Without the yield, the first size re-centres the view back down to page 8; with it, the view
  // stays where the person put it.
  for (const i of [0, 1, 2, 3, 4, 5, 6, 7]) await releasePage(page, i);
  await page.waitForTimeout(600); // let any (sabotage) re-centre animation finish
  const { scrollTop, maxScroll } = await scroll.evaluate((el) => ({
    scrollTop: el.scrollTop,
    maxScroll: el.scrollHeight - el.clientHeight,
  }));
  // Page 8 centred sits near the BOTTOM (~maxScroll). Staying where the person scrolled keeps us in the top portion,
  // well under half of max; a re-centre back to page 8 would push us past that.
  expect(scrollTop).toBeLessThan(maxScroll / 2);
});

test("Go to a note on ANOTHER file switches to that file and centres ITS page (cross-file, T185 ⟨D3⟩)", async ({
  page,
}) => {
  const { bandId, songId, fileB } = await openSongWithTwoLongPdfs(page);
  expect((await putNote(page, bandId, songId, 3)).status()).toBe(200);
  // The e2e core has no bake to resolve a note to a file, so inject the resolution: mark the note as belonging to
  // file B's page 8 (index 7) in the notes listing. That drives the cross-file goto path.
  await page.route("**/rehearsal-notes", async (route) => {
    if (route.request().method() !== "GET") return route.fallback();
    const res = await route.fetch();
    const body = await res.json();
    body.notes = (body.notes ?? []).map((n: Record<string, unknown>) => ({ ...n, fileId: fileB, filePage: 7 }));
    await route.fulfill({ response: res, json: body });
  });
  await page.reload();
  await expect(page.getByTestId("rehearsal-notes-chip")).toBeVisible();

  // Open file A and let it FULLY size — so a goto that mistakenly settled on the open file (A) would look finished.
  await page.getByTestId("file-tab").nth(0).click();
  await page.waitForFunction(() => {
    const cs = document.querySelector('[data-testid="viewer-scroll"]')?.querySelectorAll<HTMLElement>(".pdf-canvas");
    return !!cs && cs.length === 8 && Array.from(cs).every((c) => c.style.height !== "");
  });

  // The note lives on file B, so Go switches files. It must land centred on B's page 8, not leave us on A.
  await page.getByTestId("rehearsal-notes-more").click();
  await page.getByTestId("rehearsal-note-row").getByTestId("rehearsal-note-goto").click();
  await expect(page.getByTestId("rehearsal-notes-popover")).toHaveCount(0);
  await expect(page.getByTestId("file-tab").nth(1)).toHaveAttribute("aria-selected", "true"); // switched to B
  await expectCentred(page, 7);
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

/**
 * The editor's annotation tools stay usable at every width (fix 2026-10-31, VLL on an 11" tablet in portrait:
 * "tools not shown in studio on the tablet").
 *
 * Above the phone breakpoint the tool row was the only shrinkable item in the top bar, and the bar was capped
 * at 1080px while its content had grown to ~1360px. So the row absorbed the whole overflow: its buttons shrank
 * to ~17px and clipped away, leaving 0 of 9 tools in portrait (686px) and losing the last ones even on a
 * desktop. Now the tools keep their size, the bar grows to its content (capped by the viewport) and the rest
 * scrolls in the pinned-Back strip, as on the phone (T66).
 *
 * "Shown" is a HIT-TEST at the button's centre, not toBeVisible(): a button clipped by a scroll container or
 * covered by another layer still reports visible (see a-seam-test-proves-the-seam).
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));
const NOTE_PATH = fileURLToPath(new URL("./fixtures/rehearsal-note.png", import.meta.url));

// A song with a PDF and a rehearsal note open in the editor: the T66 fixture plus the "Rehearsal notes (1)"
// pill that real songs carry (the note is put the way rehearsal-note-viewer.spec.ts does it).
async function openEditor(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  const band = `${prefix}B ${stamp()}`;
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(band);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: band }).click();
  await expect(page.getByTestId("band-title")).toHaveText(band);
  // A LONG title: it takes its full 22vw in the bar, so the fixture's content is wider than the old 1080px cap at
  // every width tested (asserted by expectFixtureBeatsOldCap), as real songs with notes and charts are.
  const song = `${prefix}S ${stamp()} with a deliberately long title that fills the bar`;
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(song);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: song }).click();
  await expect(page).toHaveURL(/\/songs\//);
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
  const [, bandId, songId] = page.url().match(/\/bands\/([^/]+)\/songs\/([^/?]+)/)!;
  const put = await page.request.fetch(`/api/bands/${bandId}/songs/${songId}/rehearsal-notes/0`, {
    method: "PUT",
    multipart: {
      file: { name: "note.png", mimeType: "image/png", buffer: readFileSync(NOTE_PATH) },
      rasterHash: "fake-hash-0",
      concertId: "concert-e2e",
      concertRev: "8",
      takenAs: "member-e2e",
      width: "1600",
      height: "2261",
    },
  });
  expect(put.status()).toBe(200);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
}

// Each tool button: its width, and whether a hit-test at its centre lands on it.
async function toolHits(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>(".topbar-pill .tool-palette .tool-btn")].map((b) => {
      const r = b.getBoundingClientRect();
      const x = r.left + r.width / 2;
      const y = r.top + r.height / 2;
      const t = r.width > 0 && x >= 0 && x <= innerWidth ? document.elementFromPoint(x, y) : null;
      return { label: b.getAttribute("aria-label"), width: Math.round(r.width), hit: !!t && (t === b || b.contains(t)) };
    }),
  );
}

// The pill buttons at the END of the bar (This file, Details, …): hit-test each.
async function pillHits(page: Page) {
  return page.evaluate(() =>
    [...document.querySelectorAll<HTMLElement>(".topbar-pill .tb-scroll .pill-btn")].map((b) => {
      const r = b.getBoundingClientRect();
      const x = r.left + r.width / 2;
      const y = r.top + r.height / 2;
      const t = r.width > 0 && x >= 0 && x <= innerWidth ? document.elementFromPoint(x, y) : null;
      return { id: b.dataset.testid ?? b.textContent, hit: !!t && (t === b || b.contains(t)) };
    }),
  );
}

// The fixture is adversarial: the bar's natural content is wider than the OLD layout's space for it,
// min(1080px, 100% - 1.75rem), so the old layout would have squeezed the tools here. Without this the tests
// could pass on the bug. Measured under the fix (nav is a real box), so call it after the tool assertions.
async function expectFixtureBeatsOldCap(page: Page) {
  const m = await page.getByTestId("viewer-chrome").evaluate((pill) => {
    const nav = pill.querySelector(".tb-nav") as HTMLElement;
    const strip = pill.querySelector(".tb-scroll") as HTMLElement;
    const cs = getComputedStyle(pill);
    const extra = parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight) + parseFloat(cs.columnGap) + 2;
    const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
    return { natural: nav.getBoundingClientRect().width + strip.scrollWidth + extra, oldSpace: Math.min(1080, innerWidth - 1.75 * rem) };
  });
  expect(m.natural, `fixture must not fit the old bar: ${JSON.stringify(m)}`).toBeGreaterThan(m.oldSpace + 20);
}

async function expectAllToolsUsable(page: Page) {
  const tools = await toolHits(page);
  expect(tools.length, "the fixture must render the tool row").toBeGreaterThanOrEqual(9);
  for (const t of tools) {
    expect(t.hit, `tool "${t.label}" must be on screen and hittable: ${JSON.stringify(tools)}`).toBe(true);
    expect(t.width, `tool "${t.label}" must keep its size, not be squeezed`).toBeGreaterThanOrEqual(24);
  }
}

for (const [name, width, height] of [
  ["11-inch tablet, portrait", 686, 1097],
  ["11-inch tablet, landscape", 1097, 686],
] as const) {
  test.describe(name, () => {
    test.use({ viewport: { width, height }, deviceScaleFactor: 1.75, isMobile: true, hasTouch: true });

    test(`every tool is shown at full size, and the end of the bar is reachable by scrolling (${width}px)`, async ({
      page,
    }) => {
      await openEditor(page, `tab${width}`);
      await expectAllToolsUsable(page);
      await expectFixtureBeatsOldCap(page);
      const strip = page.getByTestId("tb-scroll");
      expect(await strip.evaluate((s) => s.scrollWidth > s.clientWidth + 1), "the bar overflows here, so it scrolls").toBe(true);

      // The controls past the edge are reachable: scroll the strip to its end, the last one is hittable.
      await strip.evaluate((s) => (s.scrollLeft = s.scrollWidth));
      const pills = await pillHits(page);
      expect(pills.length).toBeGreaterThan(0);
      expect(pills[pills.length - 1].hit, `last pill button reachable: ${JSON.stringify(pills)}`).toBe(true);
    });
  });
}

test("desktop 1280: every tool is shown at full size", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await openEditor(page, "tab1280");
  await expectAllToolsUsable(page);
  await expectFixtureBeatsOldCap(page);
});

test("wide screen: the bar grows to its content, so nothing is hidden and nothing scrolls", async ({ page }) => {
  await page.setViewportSize({ width: 1800, height: 900 });
  await openEditor(page, "tabwide");
  await expectAllToolsUsable(page);
  await expectFixtureBeatsOldCap(page);
  const strip = page.getByTestId("tb-scroll");
  expect(await strip.evaluate((s) => s.scrollWidth <= s.clientWidth + 1), "no overflow on a wide screen").toBe(true);
  const pills = await pillHits(page);
  for (const p of pills) expect(p.hit, `pill "${p.id}" on screen: ${JSON.stringify(pills)}`).toBe(true);
});

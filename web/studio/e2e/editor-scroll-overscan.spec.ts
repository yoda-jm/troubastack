/**
 * T59 — editor scroll OVERSCAN: the first page's top edge and the last page's
 * bottom edge must be scrollable FULLY clear of the floating chrome.
 *
 * The fullscreen editor floats its chrome (top pill, contextual .ctx style row,
 * bottom parts/status pill) as position:absolute glass over the canvas. Before
 * T59 the scroll range ended AT the page edges: scrolled fully up, page 1's top
 * sat ~at the top pill's bottom (still under the glass, and under any control
 * that overlaps it); scrolled fully down, the last page's bottom sat under the
 * bottom pill. VLL: "we should be able to scroll a little bit more up and down so
 * that they are on top of / before the first page and after the last."
 *
 * The fix is constant scroll overscan on .viewer-scroll (padding + scroll-padding
 * at both ends), so at the scroll extremes each terminal page edge lands inside
 * the CLEAR BAND — the canvas span not covered by the chrome (see clearBand()).
 *
 * These two probes are the guard spec (red-first: they fail on the pre-T59 CSS,
 * where the page edge sits ~10px above clearBand.top / below clearBand.bottom):
 *   1. scrolled fully UP   → first page top edge  >= clearBand.top
 *   2. scrolled fully DOWN → last page bottom edge <= clearBand.bottom
 * Plus: an annotation placed at the very top of page 1 lands + selects — the
 * extreme is now genuinely reachable/editable, not just visible.
 *
 * The live-banner variant (the has-live-banner top-pill shift) needs no separate
 * fixture: the banner shifts ONLY the top pill (+~1.9rem); the ctx band is reserved
 * ALWAYS (constant), so the shifted pill tucks inside it — clearance is unchanged.
 * editor-zeroshift.spec proves toggling chrome never moves the score. So default
 * clearance ⇒ banner clearance.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { clearBand } from "./fullscreen-helpers";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url)); // 2 pages

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}
async function createSongAndOpen(page: Page, title: string) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
}
async function uploadPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}
async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}
async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  await createBandAndOpen(page, `${prefix}Band ${stamp()}`);
  await createSongAndOpen(page, `${prefix}Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
}

/** Scroll the viewer column to an absolute top offset and settle. */
async function scrollTo(page: Page, top: number) {
  await page.getByTestId("viewer-scroll").evaluate((s, t) => s.scrollTo(0, t), top);
  await page.waitForTimeout(120);
}
async function scrollToTop(page: Page) {
  await scrollTo(page, 0);
}
async function scrollToBottom(page: Page) {
  await page.getByTestId("viewer-scroll").evaluate((s) => s.scrollTo(0, s.scrollHeight));
  await page.waitForTimeout(120);
}

test("editor: scrolled fully up, page 1's top edge clears the top chrome (T59 overscan)", async ({
  page,
}) => {
  await setup(page, "OverTop");
  await scrollToTop(page);

  const first = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const { top } = await clearBand(page);
  // The page's TOP edge must sit at/below the clear band's top — i.e. below the
  // whole top chrome, not trapped under it. Pre-T59 this is ~10px above `top`.
  expect(first.y).toBeGreaterThanOrEqual(top - 1);
});

test("editor: scrolled fully down, the last page's bottom edge clears the bottom chrome (T59 overscan)", async ({
  page,
}) => {
  await setup(page, "OverBot");
  await scrollToBottom(page);

  const pages = page.getByTestId("pdf-page");
  await expect(pages).toHaveCount(2);
  const last = (await pages.nth(1).boundingBox())!;
  const { bottom } = await clearBand(page);
  // The last page's BOTTOM edge must sit at/above the clear band's bottom — clear
  // of the floating bottom pill. Pre-T59 this is ~10px below `bottom`.
  expect(last.y + last.height).toBeLessThanOrEqual(bottom + 1);
});

test("editor: an annotation at the very top of page 1 is reachable + selectable (T59)", async ({
  page,
}) => {
  await setup(page, "OverDraw");
  await scrollToTop(page);

  // With page 1's top now in the clear band, a rect drawn against its very top
  // (page fractions ~0.01–0.06) lands in reachable canvas — not under the chrome.
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const { top } = await clearBand(page);
  // Sanity: the top of page 1 really is below the chrome (the T59 precondition).
  expect(box.y).toBeGreaterThanOrEqual(top - 1);

  await page.getByTestId("tool-rect").click();
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => box.y + box.height * f;
  await page.mouse.move(px(0.2), py(0.01));
  await page.mouse.down();
  await page.mouse.move(px(0.5), py(0.06), { steps: 12 });
  await page.mouse.up();

  // Drawing auto-provisions a personal layer (no-silent-ink) and auto-selects the
  // new object → its selection bbox appears. That proves the top-of-page-1 extreme
  // is genuinely editable, not merely visible.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.getByTestId("object-count")).toHaveText("1 objects");
});

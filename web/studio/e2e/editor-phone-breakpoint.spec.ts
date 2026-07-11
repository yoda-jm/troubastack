/**
 * T27 stage-4 residue — the phone breakpoint (<600px) of the canvas-first editor.
 *
 * The floating glass chrome (top bar / ctx bar / bottom bar) is a set of centered
 * pills on desktop/tablet; on a phone the mockup calls for EDGE-TO-EDGE sheets, and
 * the `backdrop-filter` blur — a real GPU cost on low-end Android WebViews with 3+
 * bars over a full-screen canvas (the Stage app embeds this exact route) — drops to a
 * cheap opaque fill. This guards both facts so the responsive rules can't silently
 * regress. On-device pen/finger feel is out of scope here (rides the attended T27 pass).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";

// A phone viewport: media queries respond to test.use({viewport}) in Chromium.
test.use({ viewport: { width: 390, height: 780 } });

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

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
  // T36: file management moved into the editor's Details panel — open it to reach the
  // upload form, then close it so the canvas is unobstructed for whatever follows.
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

const backdrop = (el: Element) => {
  const s = getComputedStyle(el);
  return s.backdropFilter || (s as unknown as { webkitBackdropFilter?: string }).webkitBackdropFilter || "none";
};

test("editor phone breakpoint: chrome bars are full-width sheets with the blur dropped", async ({
  page,
}) => {
  await register(page, `pb_${stamp()}`);
  await createBandAndOpen(page, `PBBand ${stamp()}`);
  await createSongAndOpen(page, `PBSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const vw = page.viewportSize()!.width;
  for (const sel of [".viewer-chrome.topbar-pill", ".bottombar-pill"]) {
    const bar = page.locator(sel);
    await expect(bar).toBeVisible();
    // Edge-to-edge SHEET, not the centered pill: flush to the left edge (the pill is
    // translateX(-50%)-centered with a ~14px inset) and wider than the pill's
    // min(1080px, 100% - 1.75rem) = vw-28. Left-edge check is robust vs scrollbar jitter.
    const box = (await bar.boundingBox())!;
    expect(box.x).toBeLessThanOrEqual(2);
    expect(box.width).toBeGreaterThan(vw - 28);
    // Reduced-blur fallback: no backdrop blur at the phone breakpoint.
    expect(await bar.evaluate(backdrop)).toBe("none");
  }

  // The desktop wheel/zoom hint is not shown on phones.
  await expect(page.locator(".wheelhint")).toBeHidden();

  // HOLD fix #1: the tool cluster stays INSIDE the top bar as one row — not the
  // vertical column that spilled over the canvas (the bar wraps; the palette doesn't).
  const topbar = (await page.locator(".viewer-chrome.topbar-pill").boundingBox())!;
  const palette = (await page.locator(".tool-palette").first().boundingBox())!;
  expect(palette.x).toBeGreaterThanOrEqual(topbar.x - 1);
  expect(palette.y).toBeGreaterThanOrEqual(topbar.y - 1);
  expect(palette.x + palette.width).toBeLessThanOrEqual(topbar.x + topbar.width + 1);
  expect(palette.y + palette.height).toBeLessThanOrEqual(topbar.y + topbar.height + 1);

  // HOLD fix #2: the Details pill (the only route to song details / T19·T25 in the
  // fullscreen editor) is reachable — visible and fully within the viewport.
  const details = page.getByTestId("my-files-edit");
  await expect(details).toBeVisible();
  const db = (await details.boundingBox())!;
  expect(db.x).toBeGreaterThanOrEqual(0);
  expect(db.x + db.width).toBeLessThanOrEqual(vw + 1);
});

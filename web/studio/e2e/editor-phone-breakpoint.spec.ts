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
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

// A phone viewport: media queries respond to test.use({viewport}) in Chromium.
test.use({ viewport: { width: 390, height: 780 } });

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

  // HOLD fix #1 (T32): the tool cluster stays ONE ROW — never the vertical column that
  // spilled over the canvas. Under T66 the whole chrome is one horizontal-scroll row, so the
  // palette may extend past the bar's right edge (it scrolls) — but it must not WRAP: its
  // height is a single row and it sits within the bar vertically.
  const topbar = (await page.locator(".viewer-chrome.topbar-pill").boundingBox())!;
  const palette = (await page.locator(".tool-palette").first().boundingBox())!;
  expect(palette.y).toBeGreaterThanOrEqual(topbar.y - 1);
  expect(palette.y + palette.height).toBeLessThanOrEqual(topbar.y + topbar.height + 1);
  expect(palette.height).toBeLessThan(48); // one row of tool buttons, not a wrapped column

  // HOLD fix #2: Details (the route to song details / T19·T25) stays REACHABLE. Under T66 it
  // rides the horizontal-scroll strip (accepted, VLL-ruled), so scroll it into view — then
  // it's visible and within the viewport.
  const strip = page.getByTestId("tb-scroll");
  await strip.evaluate((s) => (s.scrollLeft = s.scrollWidth));
  await expect
    .poll(() => strip.evaluate((s) => s.scrollLeft >= s.scrollWidth - s.clientWidth - 1))
    .toBe(true); // the strip actually reached its end
  const details = page.getByTestId("my-files-edit");
  await expect(details).toBeVisible();
  const db = (await details.boundingBox())!;
  expect(db.x).toBeGreaterThanOrEqual(0);
  expect(db.x + db.width).toBeLessThanOrEqual(vw + 1);
});

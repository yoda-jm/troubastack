/**
 * T33 — the contextual style bar is ONE slim row, no taller than the main top bar.
 *
 * It used to be ~2× the top pill (three-layer label/control/value stacks). The
 * acceptance number: `ctx-bar` height ≤ `topbar-pill` height + 2px, for BOTH a shape
 * target (rect: color/opacity/width/presets/⋯) and a text target (color/opacity/font).
 * Also guards that the ⋯ overflow popover opens and Blend works inside it (fill/border/
 * blend/hex moved there to keep the row slim).
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}
const height = async (page: Page, sel: string) => (await page.locator(sel).boundingBox())!.height;

test("ctx style bar is one slim row (≤ top-bar height + 2px), shape and text", async ({ page }) => {
  await register(page, `ct_${stamp()}`);
  await createBandAndOpen(page, `CTBand ${stamp()}`);
  await createSongAndOpen(page, `CTSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  const topH = await height(page, ".viewer-chrome.topbar-pill");

  // Shape target: activating a draw tool shows the contextual style row.
  await page.getByTestId("tool-rect").click();
  await expect(page.getByTestId("style-controls")).toBeVisible();
  const shapeH = await height(page, ".ctx-bar");
  expect(shapeH).toBeLessThanOrEqual(topH + 2);

  // Text target.
  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("style-controls")).toBeVisible();
  const textH = await height(page, ".ctx-bar");
  expect(textH).toBeLessThanOrEqual(topH + 2);

  // The ⋯ overflow popover opens and Blend still works inside it.
  await page.getByTestId("tool-rect").click(); // shape → Blend applies
  await page.getByTestId("style-more").click();
  await expect(page.getByTestId("style-popover")).toBeVisible();

  // CLASS-KILLER (arch HOLD): `toBeVisible` does NOT catch a popover clipped out of
  // paint by an ancestor's overflow (Playwright's actionability scrolls the clip
  // container to reach it; a human can't). Probe the real hit-test: the top element at
  // the Blend select's own centre must BE the select (or its descendant) — not the
  // canvas behind. This fails on the clipped (position:absolute-in-overflow) version.
  const hitOk = await page.getByTestId("style-blend").evaluate((sel) => {
    const r = sel.getBoundingClientRect();
    const hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
    return hit != null && (hit === sel || sel.contains(hit) || hit.contains(sel));
  });
  expect(hitOk, "the ⋯ popover's Blend must be the top element at its own centre (not clipped/covered)").toBe(true);

  // ANCHOR (arch HOLD round 3): the ctx bar has `transform: translateX(-50%)`, which
  // makes it the containing block for `position: fixed` — so an in-bar fixed panel
  // floats ~300px off. Portaling to <body> restores viewport-relative fixed. Assert the
  // panel is docked to the ⋯ trigger: right edges align, and it sits just below.
  const btnBox = (await page.getByTestId("style-more").boundingBox())!;
  const popBox = (await page.getByTestId("style-popover").boundingBox())!;
  expect(Math.abs(popBox.x + popBox.width - (btnBox.x + btnBox.width))).toBeLessThanOrEqual(8);
  const gap = popBox.y - (btnBox.y + btnBox.height);
  expect(gap).toBeGreaterThanOrEqual(0);
  expect(gap).toBeLessThanOrEqual(12);

  await page.getByTestId("style-blend").selectOption("multiply");
  await expect(page.getByTestId("style-blend")).toHaveValue("multiply");
});

// VLL: a custom text-size dropdown (a native <select> can't preview a size on option-hover). It offers
// small sizes the old min-0.015 slider couldn't reach, and picking one commits it. RED before: the control
// was a native <select> (or slider) with no hoverable <li> options.
test("text size is a custom dropdown offering small sizes, and picking one commits it (VLL)", async ({
  page,
}) => {
  await register(page, `cf_${stamp()}`);
  await createBandAndOpen(page, `CFBand ${stamp()}`);
  await createSongAndOpen(page, `CFSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("tool-text").click();
  const font = page.getByTestId("style-font");
  await expect(font).toBeVisible();
  await font.click(); // open the custom list
  const opt8 = page.getByTestId("style-font-option").filter({ hasText: /^8$/ });
  await expect(opt8).toHaveCount(1); // 8 (0.008) — below the old slider min
  await opt8.click();
  await expect(font).toHaveText(/^8\b/); // the button reflects the pick (not 48/80/…)
});

// Set a React range input to a stop index (native setter + events so onChange fires).
async function setRange(page: Page, testid: string, value: string) {
  await page.getByTestId(testid).evaluate((el, v) => {
    const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), "value")!.set!;
    setter.call(el, v);
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
  }, value);
}

// VLL: the toolbar size preview "is capped to the toolbar size" AND a hover preview "is not possible on a
// phone" — he wants it "at the bottom … when you select the tool and when you change it". So a live preview
// pinned bottom-centre, shown on tool-select (NO hover) and uncapped. RED before: no bottom preview existed.
test("stroke size preview shows at the bottom on tool-select (no hover), scaling + uncapped (VLL)", async ({
  page,
}) => {
  await register(page, `cr_${stamp()}`);
  await createBandAndOpen(page, `CRBand ${stamp()}`);
  await createSongAndOpen(page, `CRSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  // Selecting the tool alone shows the preview — no pointer over the canvas.
  await page.getByTestId("tool-rect").click();
  await expect(page.getByTestId("style-size-preview")).toBeVisible();
  const circle = page.locator(".size-hud-circle");

  const stops = (await page.getByTestId("style-width").getAttribute("data-stops"))!.split(",").map(Number);
  await setRange(page, "style-width", "3");
  const thin = (await circle.boundingBox())!;
  await setRange(page, "style-width", String(stops.length - 1));
  const thick = (await circle.boundingBox())!;

  expect(thick.width, "the circle grows with the width").toBeGreaterThan(thin.width + 2);
  expect(thick.width, "uncapped: past the old 24px toolbar cap").toBeGreaterThan(24);
});

// VLL: "the sample text is missing" — the text tool gets the same bottom preview, a sample at the true font
// size, shown on tool-select without hovering. RED before: no bottom preview / sample.
test("text size preview shows a sample at the bottom on tool-select, scaling with the font (VLL)", async ({
  page,
}) => {
  await register(page, `ctp_${stamp()}`);
  await createBandAndOpen(page, `CTPBand ${stamp()}`);
  await createSongAndOpen(page, `CTPSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("style-size-preview")).toBeVisible(); // shown on select, no hover
  const sample = page.locator(".size-hud-text");
  const pick = async (label: RegExp) => {
    await page.getByTestId("style-font").click();
    await page.getByTestId("style-font-option").filter({ hasText: label }).click();
  };

  await pick(/^12$/); // small
  const small = (await sample.boundingBox())!;
  await pick(/^48$/); // large
  const big = (await sample.boundingBox())!;

  expect(big.height, "the sample grows with the font size").toBeGreaterThan(small.height + 2);
});

// The headline of the custom dropdown (VLL): HOVERING a size in the open list previews it in the HUD, live,
// WITHOUT committing. RED before: a native <select> has no per-option hover.
test("hovering a size in the dropdown previews it live, without committing (VLL)", async ({ page }) => {
  await register(page, `chp_${stamp()}`);
  await createBandAndOpen(page, `ChpBand ${stamp()}`);
  await createSongAndOpen(page, `ChpSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("tool-text").click();
  // Commit a small size, then open the list and HOVER a large one.
  await page.getByTestId("style-font").click();
  await page.getByTestId("style-font-option").filter({ hasText: /^10$/ }).click();
  await page.getByTestId("style-font").click();
  await page.getByTestId("style-font-option").filter({ hasText: /^48$/ }).hover();

  // The button still shows 10 (hover did NOT commit) but the HUD previews the hovered 48 (much taller).
  await expect(page.getByTestId("style-font")).toHaveText(/^10\b/);
  const hovered = (await page.locator(".size-hud-text").boundingBox())!;
  expect(hovered.height, "the HUD previews the hovered 48, not the committed 10").toBeGreaterThan(24);
});

// VLL: "no text on hover over the dropdown item, could be nice" — on desktop, hovering a size control
// re-flashes the bottom preview so you can see the size without changing it. RED before: no hover trigger,
// so once the initial select-flash fades, a hover does nothing.
test("hovering a size control re-flashes the bottom preview (desktop) (VLL)", async ({ page }) => {
  await register(page, `chov_${stamp()}`);
  await createBandAndOpen(page, `ChovBand ${stamp()}`);
  await createSongAndOpen(page, `ChovSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("tool-text").click();
  const hud = page.getByTestId("style-size-preview");
  await expect(hud).toHaveClass(/\bshow\b/); // the tool-select flash
  await expect(hud).not.toHaveClass(/\bshow\b/, { timeout: 3000 }); // …then it fades out

  await page.getByTestId("style-font").hover(); // hovering the dropdown re-flashes it
  await expect(hud).toHaveClass(/\bshow\b/);
});

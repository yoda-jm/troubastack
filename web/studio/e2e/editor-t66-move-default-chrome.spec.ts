/**
 * T66: Move is the default + first tool, the Select icon is a dashed rectangle, and the
 * phone editor chrome is compacted to ≤ 2 rows (killing the large top margin).
 */
import { test, expect, type Page, type CDPSession } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

// Real touch (pointerType:"touch") via CDP, so tests exercise the tap-vs-scroll path.
async function touchTap(cdp: CDPSession, x: number, y: number) {
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x, y }] });
  // a tiny move, like a real finger — the point of Part E is this must NOT be read as scroll
  await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: x + 2, y: y + 1 }] });
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
}
async function touchDrag(cdp: CDPSession, x0: number, y0: number, x1: number, y1: number, steps = 10) {
  await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x: x0, y: y0 }] });
  for (let i = 1; i <= steps; i++) {
    await cdp.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: x0 + ((x1 - x0) * i) / steps, y: y0 + ((y1 - y0) * i) / steps }],
    });
  }
  await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
}
// A real finger DOUBLE-TAP: two taps at ~the same point, close in time (the browser fires no
// dblclick on touch — WetCanvas detects it in onPointerUp). Each tap carries a few px of
// jitter (a real finger never lands perfectly still) — this must NOT pan the view.
async function touchDoubleTap(cdp: CDPSession, x: number, y: number) {
  for (let i = 0; i < 2; i++) {
    await cdp.send("Input.dispatchTouchEvent", { type: "touchStart", touchPoints: [{ x, y }] });
    await cdp.send("Input.dispatchTouchEvent", { type: "touchMove", touchPoints: [{ x: x + 4, y: y + 3 }] });
    await cdp.send("Input.dispatchTouchEvent", { type: "touchEnd", touchPoints: [] });
  }
}

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  const band = `${prefix}B ${stamp()}`;
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(band);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: band }).click();
  await expect(page.getByTestId("band-title")).toHaveText(band);
  const song = `${prefix}S ${stamp()}`;
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
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
}

test("Move is the default + first tool; Select icon is dashed; Select still marquees (T66 A/B)", async ({
  page,
}) => {
  await setup(page, "t66");

  // Default active tool is Move (pan) — the neutral "no tool" resting state.
  await expect(page.getByTestId("tool-move")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("tool-select")).toHaveAttribute("aria-pressed", "false");

  // Move is the FIRST button in the palette.
  const ids = await page
    .locator('.tool-palette [data-testid^="tool-"]')
    .evaluateAll((els) => els.map((e) => e.getAttribute("data-testid")));
  expect(ids[0]).toBe("tool-move");
  expect(ids[1]).toBe("tool-select");

  // The Select icon is a dashed rectangle (a rect with stroke-dasharray, no fill).
  const dashed = await page
    .getByTestId("tool-select")
    .locator("svg rect")
    .evaluate((r) => getComputedStyle(r).strokeDasharray !== "none" && (r.getAttribute("fill") === "none"));
  expect(dashed).toBe(true);

  // Picking Select still marquees: a drag on empty space raises the dashed .selection-box.
  await page.getByTestId("tool-select").click();
  await expect(page.getByTestId("tool-select")).toHaveAttribute("aria-pressed", "true");
  const box = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  await page.mouse.move(box.x + box.width * 0.2, box.y + box.height * 0.2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.6, box.y + box.height * 0.5, { steps: 6 });
  const marquee = page.locator(".selection-box");
  await expect(marquee).toBeVisible();
  expect(await marquee.evaluate((el) => getComputedStyle(el).borderStyle)).toBe("dashed");
  await page.mouse.up();
});

test("phone editor chrome is ONE row (pinned Back + scroll strip) and the margin shrinks (T66 C)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 780 });
  await setup(page, "t66c");

  await page.screenshot({
    path: "/tmp/claude-1000/-home-yoda-dev-git-troubastack/72d1f559-04a0-4860-bc97-97f9ef5cf3e3/scratchpad/t66-phone-chrome.png",
  });

  const chrome = page.getByTestId("viewer-chrome");
  const m = await chrome.evaluate((el) => {
    const rowH = (el as HTMLElement).getBoundingClientRect().height;
    const chromeH =
      parseFloat(getComputedStyle(el.parentElement!).getPropertyValue("--chrome-h")) || 0;
    // Back + the scroll strip sit on the SAME row (their vertical spans overlap) → one row.
    const back = el.querySelector(".tb-back") as HTMLElement;
    const strip = el.querySelector(".tb-scroll") as HTMLElement;
    const bR = back.getBoundingClientRect();
    const sR = strip.getBoundingClientRect();
    const oneRow = sR.top < bR.bottom - 2 && sR.bottom > bR.top + 2;
    const backVisible = bR.width > 0 && bR.height > 0;
    // The strip is a horizontal-scroll region that overflows at 390px with the full toolset.
    const stripScrolls = strip.scrollWidth > strip.clientWidth + 1;
    return { rowH, chromeH, oneRow, backVisible, stripScrolls };
  });
  // One row is materially shorter than the old 3-row ~117px (~48px + inset).
  expect(m.rowH).toBeLessThan(72);
  expect(m.chromeH).toBeGreaterThan(0);
  expect(m.chromeH).toBeLessThan(72);
  expect(m.oneRow).toBe(true);
  expect(m.backVisible).toBe(true);
  expect(m.stripScrolls).toBe(true);

  // Icon-only actions on phone (labels hidden).
  await expect(page.getByTestId("sidebar-toggle").locator(".pill-label")).toBeHidden();
  await expect(page.getByTestId("sidebar-toggle").locator(".pill-icon")).toBeVisible();

  // Back stays reachable even after scrolling the strip to the end (it's pinned, not in the strip).
  await page.getByTestId("tb-scroll").evaluate((s) => (s.scrollLeft = s.scrollWidth));
  await page.waitForTimeout(80);
  await expect(page.getByTestId("song-title").locator("xpath=..")).toBeVisible(); // .tb-nav still on screen
  await expect(page.locator(".tb-back")).toBeVisible();

  // The score's first page is reachable (not hidden under the chrome): its top sits at or
  // below the chrome bottom once scrolled to the top.
  await page.getByTestId("viewer-scroll").evaluate((s) => (s.scrollTop = 0));
  await page.waitForTimeout(150);
  const clear = await page.evaluate(() => {
    const page1 = document.querySelector('[data-testid="pdf-page"]') as HTMLElement;
    const chromeEl = document.querySelector('[data-testid="viewer-chrome"]') as HTMLElement;
    return page1.getBoundingClientRect().top >= chromeEl.getBoundingClientRect().bottom - 2;
  });
  expect(clear).toBe(true);

  await page.screenshot({
    path: "/tmp/claude-1000/-home-yoda-dev-git-troubastack/72d1f559-04a0-4860-bc97-97f9ef5cf3e3/scratchpad/t66-phone-chrome.png",
  });
});

test("double-click zooms in Move mode (Fit-width ↔ 2×), not in Select mode (T66 D)", async ({
  page,
}) => {
  await setup(page, "t66d");
  const zoom = page.getByTestId("zoom-mode");
  await expect(zoom).toHaveValue("fit-width"); // opens at fit-width

  const box = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  // Tap OFF-centre (upper area) so a real zoom-to-point is distinguishable from a re-centre.
  const cx = box.x + box.width / 2;
  const cy = box.y + box.height * 0.25;

  const pageBox = async () => (await page.getByTestId("pdf-page").first().boundingBox())!;
  const fit = await pageBox();
  // The content-point under the tap, as a fraction of the page height (the zoom anchor).
  const anchorFrac = (b: { y: number; height: number }) => (cy - b.y) / b.height;
  const fracBefore = anchorFrac(fit);

  // Move mode (default): a double-click zooms IN — the page actually grows ~2× (not just a
  // label flip / re-centre) AND stays anchored at the tapped point (the same content-fraction
  // remains under the cursor — a re-centre would move it toward 0.5).
  await page.mouse.dblclick(cx, cy);
  await expect(zoom).not.toHaveValue("fit-width");
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeGreaterThan(1.6);
  const zoomed = await pageBox();
  expect(Math.abs(anchorFrac(zoomed) - fracBefore), "zoom stays anchored to the tapped point").toBeLessThan(0.12);

  // A second double-click toggles back to fit-width (page returns to its fit size).
  await page.mouse.dblclick(cx, cy);
  await expect(zoom).toHaveValue("fit-width");
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeLessThan(1.2);

  // Desync guard (VLL's report + the ruling): the in/out choice is decided from the ACTUAL
  // zoom, not a private toggle. Zoom in, then return to fit by ANOTHER route (the zoom-mode
  // dropdown) so a boolean flag would still read "zoomed" — a double-tap must then zoom IN
  // to the point, NOT re-centre at fit. (A boolean-toggle impl fails this: it takes the fit
  // branch and only re-centres.)
  await page.mouse.dblclick(cx, cy); // zoom in again
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeGreaterThan(1.6);
  await zoom.selectOption("fit-width"); // back to fit via the dropdown, not the double-tap
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeLessThan(1.2);
  const fracAtFit = anchorFrac(await pageBox()); // the tapped content-point, measured at THIS fit state
  await page.mouse.dblclick(cx, cy); // double-tap AT fit → must zoom IN, not re-centre
  await expect(zoom).not.toHaveValue("fit-width");
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeGreaterThan(1.6);
  expect(
    Math.abs(anchorFrac(await pageBox()) - fracAtFit),
    "double-tap-at-fit zooms to the point, not a bare re-centre",
  ).toBeLessThan(0.12);
  await zoom.selectOption("fit-width"); // reset to fit for the Select-mode check below

  // Select mode: double-click does NOT zoom (reserved for future object editing).
  await page.getByTestId("tool-select").click();
  await page.mouse.dblclick(cx, cy);
  await expect(zoom).toHaveValue("fit-width");
  expect((await pageBox()).width / fit.width).toBeLessThan(1.2);
});

test("a real finger TAP on a draw tool selects it in the scrolling phone bar + draws (T66 E)", async ({
  page,
}) => {
  // The blocker VLL reported on a phone: the always-scrolling one-row chrome ate a finger
  // tap on a draw tool as a horizontal scroll, so the tool never selected and the ctx bar
  // never appeared. The fix is `touch-action: pan-x` on the scroll strip — the browser only
  // treats a *horizontal drag* as a scroll, so a tap (and any vertical jitter) fires the click.
  await page.setViewportSize({ width: 390, height: 780 });
  await setup(page, "t66e");
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await expect(page.getByTestId("tool-rect")).toBeEnabled(); // a drawable personal layer exists

  // Part E guard: the scroll strip (and its inner tool row) yield the tap to the button.
  for (const sel of [".tb-scroll", ".tb-scroll .tool-palette"]) {
    expect(await page.locator(sel).evaluate((el) => getComputedStyle(el).touchAction)).toBe(
      "pan-x",
    );
  }

  const cdp = await page.context().newCDPSession(page);

  // A REAL finger tap — with the tiny move a finger always makes — on the Rect tool selects
  // it AND raises the ctx/style bar. This is the exact gesture that was swallowed before.
  const rb = (await page.getByTestId("tool-rect").boundingBox())!;
  await touchTap(cdp, rb.x + rb.width / 2, rb.y + rb.height / 2);
  await expect(page.getByTestId("tool-rect")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("style-controls")).toBeVisible();

  // …and a finger draw then lays down an object (the tool is genuinely armed, not just lit).
  const count = async () => parseInt(await page.getByTestId("object-count").innerText(), 10);
  const before = await count();
  const cb = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  await touchDrag(
    cdp,
    cb.x + cb.width * 0.3,
    cb.y + cb.height * 0.35,
    cb.x + cb.width * 0.6,
    cb.y + cb.height * 0.55,
  );
  await page.waitForTimeout(250);
  expect(await count()).toBeGreaterThan(before);
});

test("a real finger DOUBLE-TAP zooms to the point in Move mode (T66 D, touch path)", async ({
  page,
}) => {
  // VLL tests on a phone: the double-tap-to-zoom must work via the TOUCH path (onPointerUp
  // detection), not just the mouse dblclick path. In Move mode a single tap is a degenerate
  // pan (beginGesture/endGesture) — this guards that the extra commit doesn't break the zoom.
  await page.setViewportSize({ width: 390, height: 780 });
  await setup(page, "t66dt");
  const zoom = page.getByTestId("zoom-mode");
  await expect(zoom).toHaveValue("fit-width");

  const cb = (await page.getByTestId("edit-canvas").first().boundingBox())!;
  const tx = cb.x + cb.width / 2;
  const ty = cb.y + cb.height * 0.25; // off-centre, so zoom-to-point ≠ re-centre
  const pageBox = async () => (await page.getByTestId("pdf-page").first().boundingBox())!;
  const fit = await pageBox();
  const anchorFrac = (b: { y: number; height: number }) => (ty - b.y) / b.height;
  const fracAtFit = anchorFrac(fit);

  const cdp = await page.context().newCDPSession(page);
  await touchDoubleTap(cdp, tx, ty);
  // Must actually zoom IN (~2×) toward the tapped point. Two failure modes this guards: a
  // re-centre keeps the page at fit width; a double-INVOCATION (touch detection + the browser's
  // synthesized dblclick) toggles in→out and also nets to fit. Both leave width ratio ≈ 1.
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeGreaterThan(1.6);
  expect(
    Math.abs(anchorFrac(await pageBox()) - fracAtFit),
    "finger double-tap zooms to the point, not a re-centre",
  ).toBeLessThan(0.15);

  // A second double-tap at the SAME spot returns to fit AND — RULED Option A — anchors the
  // tapped point on the way OUT too (symmetric with zoom-in): the content-fraction under the
  // finger is preserved, so it lands you back near your spot instead of dropping you low in the
  // document (VLL's "fit too low" was the old bare setZoomMode with a stale zoomed scrollTop).
  await page.waitForTimeout(450);
  await touchDoubleTap(cdp, tx, ty);
  await expect.poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 }).toBeLessThan(1.2);
  expect(
    Math.abs(anchorFrac(await pageBox()) - fracAtFit),
    "zoom-out anchors the tapped point (symmetric), not a stale-scroll drop",
  ).toBeLessThan(0.15);

  // VLL's "cycles but not always" report: REPEATED double-taps must STRICTLY alternate zoom
  // in↔out — never double-fire (touch detection + synth dblclick) back to the start, never
  // stick. Tap a stable mid-viewport point (always on the canvas) so the gesture always lands.
  await zoom.selectOption("fit-width");
  await page.waitForTimeout(300);
  const vp = page.viewportSize()!;
  const px = vp.width / 2;
  const py = vp.height / 2;
  for (let i = 0; i < 6; i++) {
    await page.waitForTimeout(450);
    await touchDoubleTap(cdp, px, py);
    const expectZoomedIn = i % 2 === 0; // DT1 in, DT2 out, DT3 in, …
    await expect
      .poll(async () => (await pageBox()).width / fit.width, { timeout: 4000 })
      [expectZoomedIn ? "toBeGreaterThan" : "toBeLessThan"](expectZoomedIn ? 1.6 : 1.2);
  }
});

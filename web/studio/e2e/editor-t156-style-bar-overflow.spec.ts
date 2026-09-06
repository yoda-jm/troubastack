/**
 * T156 ⟨A⟩ — the style bar (second toolbar) must be reachable when it overflows on a narrow screen.
 * DIAGNOSTIC FIRST (Fable's mandate): reproduce at a phone viewport and MEASURE why the existing
 * overflow-x:auto does not let you reach the last control — scrollWidth vs clientWidth, whether it wrapped,
 * its computed pointer-events, and whether setting scrollLeft actually moves it.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

async function setup(page: Page) {
  await register(page, `t156_${stamp()}`);
  await createBandAndOpen(page, `T156Band ${stamp()}`);
  await createSongAndOpen(page, `T156Song ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await openEditorReady(page);
}

async function measure(page: Page) {
  return page.getByTestId("style-controls").evaluate((el) => {
    const cs = getComputedStyle(el);
    const kids = Array.from(el.children) as HTMLElement[];
    const firstTop = kids.length ? kids[0].offsetTop : 0;
    const wrapped = kids.some((k) => k.offsetTop > firstTop + 4);
    const last = kids[kids.length - 1];
    el.scrollLeft = 99999;
    const scrolledTo = el.scrollLeft;
    el.scrollLeft = 0;
    return {
      scrollWidth: el.scrollWidth,
      clientWidth: el.clientWidth,
      offsetWidth: (el as HTMLElement).offsetWidth,
      overflows: el.scrollWidth > el.clientWidth + 1,
      wrapped,
      childCount: kids.length,
      pointerEvents: cs.pointerEvents,
      overflowX: cs.overflowX,
      maxScrollLeft: scrolledTo, // >0 ⇒ the container IS scrollable programmatically
      lastRight: last ? Math.round(last.getBoundingClientRect().right) : 0,
      viewportW: window.innerWidth,
    };
  });
}

test("⟨A⟩ diagnose: does the style bar overflow + scroll at a phone width?", async ({ page }) => {
  await page.setViewportSize({ width: 360, height: 780 });
  await setup(page);
  await page.getByTestId("tool-rect").click(); // draw tool → the ctx style bar shows
  await expect(page.getByTestId("style-controls")).toBeVisible();

  const m = await measure(page);
  console.log("T156⟨A⟩ phone measure:", JSON.stringify(m, null, 2));

  // The premise of the whole task: at a phone width the bar overflows its container, without wrapping.
  expect(m.overflows, "style bar should overflow at phone width (else the fixture is wrong)").toBe(true);
  expect(m.wrapped, "must NOT wrap to a second line (T33) — overflow into the scroll strip instead").toBe(
    false,
  );

  // ⟨R1⟩ / the real defect: the strip overflows AND is programmatically scrollable, yet the user can't
  // reach the far controls because the scroll CONTAINER cannot receive a pan gesture — it inherits
  // `pointer-events: none` from the pass-through .ctx-bar (unlike the top pill). So a touch-drag on the
  // strip falls through to the canvas instead of scrolling it. When it overflows, the strip must be
  // interactive so it can be panned. Red today (pointer-events "none").
  expect(m.overflows && m.maxScrollLeft > 0, "strip should be programmatically scrollable").toBe(true);
  expect(m.pointerEvents, "an overflowing style strip must accept a pan gesture, not fall through").not.toBe(
    "none",
  );
});

test("⟨A⟩ teeth: at a desktop width the bar does NOT overflow (fixture measures overflow, not something else)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await setup(page);
  await page.getByTestId("tool-rect").click();
  await expect(page.getByTestId("style-controls")).toBeVisible();
  const m = await measure(page);
  console.log("T156⟨A⟩ desktop measure:", JSON.stringify(m, null, 2));
  expect(m.overflows, "at desktop width the bar should fit (no overflow)").toBe(false);
  // When it fits, the strip must STAY pass-through (pointer-events:none) so a gesture over the glass still
  // reaches the score — the fix only makes an OVERFLOWING strip interactive.
  expect(m.pointerEvents, "a non-overflowing strip must remain pass-through glass").toBe("none");
});

// Set a React-controlled range input to `value` (native setter + input event, so React's onChange fires).
async function setRange(page: Page, testid: string, value: string) {
  await page.getByTestId(testid).evaluate((el, v) => {
    const inp = el as HTMLInputElement;
    const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
    setter.call(inp, v);
    inp.dispatchEvent(new Event("input", { bubbles: true }));
  }, value);
}

const previewBox = (page: Page) => page.getByTestId("style-size-preview").boundingBox();

test("⟨B⟩ the stroke-size preview circle's diameter tracks the width", async ({ page }) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await setup(page);
  await page.getByTestId("tool-rect").click(); // stroke target → dotted-circle preview
  await expect(page.getByTestId("style-size-preview")).toBeVisible();

  await setRange(page, "style-width", "4"); // a thin stroke
  const thin = (await previewBox(page))!;
  await setRange(page, "style-width", "11"); // a much thicker stroke
  const thick = (await previewBox(page))!;

  console.log("T156⟨B⟩ circle diameters:", thin.width, "→", thick.width);
  // A preview that never changed would pass a presence-only test — so assert it GROWS with the width.
  expect(thick.width).toBeGreaterThan(thin.width + 2);
  expect(thin.width).toBeGreaterThan(0);
});

test("⟨B⟩ the text-size preview renders a neutral sample at the chosen size (no brand string)", async ({
  page,
}) => {
  // A phone viewport (short rendered page) so a text sample at the chosen size fits UNDER the slim pill's
  // height cap and its range is visible — at a desktop page height even the minimum font is ~a full pill
  // tall, so the cap (correctly) saturates it. The preview is the first strip item, so it stays in view.
  await page.setViewportSize({ width: 360, height: 780 });
  await setup(page);
  await page.getByTestId("tool-text").click(); // text target → text sample preview
  const preview = page.getByTestId("style-size-preview");
  await expect(preview).toBeVisible();

  // Neutral legend, never a brand word (i18n + maintenance).
  await expect(preview).toHaveText("Abc");

  await setRange(page, "style-font", "0.015"); // small
  const small = (await preview.boundingBox())!;
  await setRange(page, "style-font", "0.03"); // large
  const large = (await preview.boundingBox())!;

  console.log("T156⟨B⟩ text sample heights:", small.height, "→", large.height);
  expect(large.height).toBeGreaterThan(small.height + 2);
});

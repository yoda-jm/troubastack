// T198 — the Studio per-page crop tool (UI stage U1). Exercises the real in-memory core (the crop endpoint),
// not a mock. Covers: the authoring loop (draw → save → badge), the U1 blocker (a crop-mode drag that reaches
// a non-croppable file must create NO annotation), the Keep/Cancel warning naming the layer, and the passive
// veil that stays when the tool is off.
import { fileURLToPath } from "node:url";
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

const LONG_PDF = fileURLToPath(new URL("./fixtures/long-chart.pdf", import.meta.url));

// Upload a second PDF (long-chart.pdf) so a test has two file tabs.
async function uploadSecondPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(LONG_PDF);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(2);
  await page.getByTestId("my-files-edit").click();
}

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 6) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => top + bandH * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

// Drag within a page's crop overlay, in overlay-relative fractions (the overlay is inset over the page).
async function dragCrop(page: Page, i: number, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const ov = page.getByTestId(`crop-overlay-${i}`);
  await expect(ov).toBeVisible();
  const b = (await ov.boundingBox())!;
  await page.mouse.move(b.x + b.width * fx, b.y + b.height * fy);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width * tx, b.y + b.height * ty, { steps });
  await page.mouse.up();
}

const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function openPdf(page: Page) {
  await register(page, `crop_${stamp()}`);
  await createBandAndOpen(page, `CropBand ${stamp()}`);
  await createSongAndOpen(page, `CropSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

test("crop: draw a bake area, save it, and the file is marked cropped — making no annotation", async ({ page }) => {
  await openPdf(page);
  const before = await objectCount(page);

  await page.getByTestId("tool-crop").click();
  await expect(page.getByTestId("crop-save")).toBeDisabled();

  // Draw the window → it appears with handles, and Save enables.
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-handle-0-nw")).toBeVisible();
  await expect(page.getByTestId("crop-grid-0")).toBeVisible(); // the alignment grid shows while editing
  await expect(page.getByTestId("crop-save")).toBeEnabled();
  // Crop is NOT an annotation: nothing was added to the layer.
  expect(await objectCount(page)).toBe(before);

  // No annotations exist → Save commits straight away (no Keep/Cancel step), and the badge appears.
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeDisabled();

  // Escape during a REDRAW over a saved crop must RESTORE it, never wipe it. Start a new draw from the veil
  // corner (outside the window), move, then press Escape mid-gesture: the window stays and the draft matches
  // the saved crop (Save stays disabled). Pre-fix (draw recorded before:null) this cleared the page.
  const ov = page.getByTestId("crop-overlay-0");
  const b = (await ov.boundingBox())!;
  await page.mouse.move(b.x + b.width * 0.9, b.y + b.height * 0.9);
  await page.mouse.down();
  await page.mouse.move(b.x + b.width * 0.75, b.y + b.height * 0.75, { steps: 5 });
  await page.keyboard.press("Escape");
  await page.mouse.up();
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeDisabled(); // restored to the saved crop → not dirty

  // The veil stays when the tool is off: switch to Move → the window still shows, but its handles are gone.
  await page.getByTestId("tool-move").click();
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-handle-0-nw")).toHaveCount(0);
});

test("crop: switching to a generated chart with the tool active creates no annotation (U1 blocker)", async ({ page }) => {
  await openPdf(page);

  // Add a generated chart to the song (a non-croppable file), and a layer on it so a stray draw COULD land.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("new-text-chart").click();
  await expect(page.getByTestId("chart-editor")).toBeVisible();
  await page.getByTestId("chart-source").fill("# Title\n\n## Verse\nsome words here\n");
  await page.getByTestId("chart-save").click();
  // Two files now in the strip.
  await expect(page.getByTestId("file-tab")).toHaveCount(2);

  // Open the chart and give it an editable layer.
  const chartTab = page.getByTestId("file-tab").nth(1);
  await chartTab.click();
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  const before = await objectCount(page);

  // Select Crop while the PDF is open...
  await page.getByTestId("file-tab").first().click();
  await page.getByTestId("tool-crop").click();
  // ...then switch to the chart: the crop tool must drop (not croppable) and NOT persist as a draw tool.
  await chartTab.click();
  await expect(page.getByTestId("tool-crop")).toHaveCount(0); // button gone for a generated chart
  await expect(page.getByTestId("tool-move")).toHaveAttribute("aria-pressed", "true"); // reset off crop

  // A drag on the chart must create NO annotation (pre-fix it saved a type:"" object).
  await dragOnPage(page, 0.2, 0.3, 0.6, 0.6);
  await page.waitForTimeout(300);
  expect(await objectCount(page)).toBe(before);
});

test("crop: a crop that hides a mark asks Keep/Cancel, naming the layer", async ({ page }) => {
  await openPdf(page);

  // Draw a rect annotation in the bottom-right corner on a named layer.
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.8, 0.8, 0.95, 0.95);
  await expect.poll(() => objectCount(page)).toBe(1);
  await closeDrawer(page); // so the floating chrome doesn't cover the crop Save button in the test viewport

  // Crop the top-left: the corner mark falls outside.
  await page.getByTestId("tool-crop").click();
  await dragCrop(page, 0, 0.05, 0.05, 0.4, 0.4);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible(); // the window drew
  // The live dry run flags the outside mark (checkpoint: the mark is detected as hidden).
  await expect(page.getByTestId("crop-warning")).toBeVisible();

  await page.getByTestId("crop-save").click();

  // The Keep/Cancel dialog appears and names the actual LAYER ("My notes", the default). A stubbed empty
  // layer-name list would red here.
  await expect(page.getByTestId("crop-confirm")).toBeVisible();
  await expect(page.getByTestId("crop-confirm")).toContainText("annotation");
  await expect(page.getByTestId("crop-confirm")).toContainText("My notes");
  await page.getByTestId("crop-confirm-cancel").click();
  await expect(page.getByTestId("crop-confirm")).toHaveCount(0);
  await expect(page.getByTestId("file-tab-cropped")).toHaveCount(0);

  // Save again → Keep → it commits and the badge appears.
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("crop-confirm")).toBeVisible();
  await page.getByTestId("crop-confirm-keep").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();
});

// HOTFIX (data loss): after a RELOAD the file id arrives before its crop data, so the draft was seeded {} and
// never re-seeded — the window vanished and a Save WIPED the stored crop. Red on pre-fix main: post-reload
// the crop window is absent in crop mode and Save is enabled (a wipe). Fixed: the window shows and Save is
// disabled (draft == saved).
test("crop: a saved crop survives a reload (no empty-seed wipe)", async ({ page }) => {
  await openPdf(page);
  await page.getByTestId("tool-crop").click();
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7);
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();

  // Reload: the editor re-opens the same file (?file=), its crop data arriving after the id.
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });

  await page.getByTestId("tool-crop").click();
  // The saved crop seeded the draft: the window is there, and Save is DISABLED (nothing changed — no wipe).
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-save")).toBeDisabled();
});

// 4b: Auto (all pages). crop-detect is mocked for determinism (the real endpoint is Go-tested); this covers
// the UI wiring — the proposal merges in, shows the "auto" tag, and saves.
test("crop: Auto all pages proposes a crop that then saves", async ({ page }) => {
  await openPdf(page);
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.15, y: 0.15, w: 0.6, h: 0.6, source: "auto" } }, hiddenMarks: [] }),
    }),
  );
  await page.getByTestId("tool-crop").click();
  await expect(page.getByTestId("crop-save")).toBeDisabled();

  await page.getByTestId("crop-auto-all").click();
  // The proposal appears as a window tagged "auto", Save enables, and the summary reports the outcome.
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-auto-tag-0")).toBeVisible();
  await expect(page.getByTestId("crop-notice")).toContainText("Auto-cropped");
  await expect(page.getByTestId("crop-save")).toBeEnabled();

  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();
});

// 4b (ruling A): Auto never overwrites a MANUAL rect. A hand-drawn page stays manual after Auto all.
test("crop: Auto all does not overwrite a manual rect", async ({ page }) => {
  await openPdf(page);
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.1, y: 0.1, w: 0.3, h: 0.3, source: "auto" } }, hiddenMarks: [] }),
    }),
  );
  await page.getByTestId("tool-crop").click();
  // Draw a manual crop on page 0.
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
  await expect(page.getByTestId("crop-auto-tag-0")).toHaveCount(0); // manual, not auto

  // Auto all: page 0 has a manual rect, so the proposal must NOT replace it — no "auto" tag appears.
  await page.getByTestId("crop-auto-all").click();
  await expect(page.getByTestId("crop-auto-tag-0")).toHaveCount(0);
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
});

// 4b NO-GO #1: a detect result that RETURNS after a file switch must NOT land on the now-open file.
test("crop: a delayed Auto result does not land on another file after a switch", async ({ page }) => {
  await openPdf(page);
  await uploadSecondPdf(page);
  // A slow crop-detect that proposes a crop for page 0.
  await page.route("**/files/*/crop-detect", async (route) => {
    await new Promise((r) => setTimeout(r, 900));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.1, y: 0.1, w: 0.6, h: 0.6, source: "auto" } }, hiddenMarks: [] }),
    });
  });
  await page.getByTestId("tool-crop").click();
  // Fire Auto on file A, then immediately switch to file B before it returns.
  await page.getByTestId("crop-auto-all").click();
  await page.getByTestId("file-tab").nth(1).click();
  // Positive control: B's interactive crop overlay IS mounted (so "no window on B" below is a real absence,
  // not a vacuous pass because the overlay never rendered).
  await expect(page.getByTestId("crop-overlay-0")).toBeVisible();
  await page.waitForTimeout(1200); // let A's detect resolve while B is open
  // B must be untouched: no crop window appeared on it.
  await expect(page.getByTestId("crop-rect-0")).toHaveCount(0);
  // Back on A, the proposal IS there — which also proves the detect actually resolved (so the B check above
  // wasn't passing merely because nothing came back).
  await page.getByTestId("file-tab").first().click();
  await expect(page.getByTestId("crop-rect-0")).toBeVisible();
});

// 4b NO-GO #2: Escape during an edit restores the rect VERBATIM — an auto rect stays auto.
test("crop: Escape restores an auto rect without forcing it manual", async ({ page }) => {
  await openPdf(page);
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.2, y: 0.2, w: 0.5, h: 0.5, source: "auto" } }, hiddenMarks: [] }),
    }),
  );
  await page.getByTestId("tool-crop").click();
  await page.getByTestId("crop-auto-all").click();
  await expect(page.getByTestId("crop-auto-tag-0")).toBeVisible();

  const rectBox = () => page.getByTestId("crop-rect-0").boundingBox();
  const before = (await rectBox())!;
  // Grab INSIDE the window at a point that is actually on-screen (the page can be taller than the viewport,
  // so a point deep in the window may be below the fold — a pointerdown there would miss).
  const vh = page.viewportSize()!.height;
  const gx = before.x + before.width * 0.5;
  const gy = Math.min(before.y + 40, vh - 60);
  await page.mouse.move(gx, gy);
  await page.mouse.down();
  await page.mouse.move(gx + 150, gy + 30, { steps: 6 }); // move right+down, staying on-screen
  // Positive control: the window ACTUALLY moved mid-drag (so Escape below is undoing a real move, not a
  // no-op). Poll — the move dispatches onChange, whose React re-render is a tick behind the mouse event.
  await expect
    .poll(async () => {
      const d = (await rectBox())!;
      return Math.abs(d.x - before.x) + Math.abs(d.y - before.y);
    })
    .toBeGreaterThan(8);
  // Escape mid-gesture → restore.
  await page.keyboard.press("Escape");
  await page.mouse.up();
  const after = (await rectBox())!;
  // Restored to where it was (within a px or two) AND still auto (not forced manual).
  expect(Math.abs(after.x - before.x) + Math.abs(after.y - before.y)).toBeLessThan(3);
  await expect(page.getByTestId("crop-auto-tag-0")).toBeVisible();
});

// 4b NO-GO #1 nit: per-page Auto also never overwrites a manual rect.
test("crop: per-page Auto does not overwrite a manual rect", async ({ page }) => {
  await openPdf(page);
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.1, y: 0.1, w: 0.3, h: 0.3, source: "auto" } }, hiddenMarks: [] }),
    }),
  );
  await page.getByTestId("tool-crop").click();
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7); // a manual crop on page 0
  await expect(page.getByTestId("crop-auto-tag-0")).toHaveCount(0);
  await page.getByTestId("crop-auto-0").click(); // per-page Auto
  // Wait for the Auto to COMPLETE and report it kept the manual rect — so the no-overwrite check below isn't
  // a vacuous pass before the proposal lands. This notice only appears when the merge started from the
  // existing (manual) rect and kept it; a merge from {} would apply the proposal (no "kept it", auto tag on).
  await expect(page.getByTestId("crop-notice")).toContainText("kept it");
  await expect(page.getByTestId("crop-auto-tag-0")).toHaveCount(0); // still manual, not replaced
});

// 4b NO-GO #3: an explicit Auto that finds nothing says so.
test("crop: Auto with no detected content reports it", async ({ page }) => {
  await openPdf(page);
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ pages: {}, hiddenMarks: [] }) }),
  );
  await page.getByTestId("tool-crop").click();
  await page.getByTestId("crop-auto-0").click(); // per-page Auto, empty result
  await expect(page.getByTestId("crop-notice")).toContainText("No content detected");
});

// GO follow-up: after a RELOAD, Auto all keeps a SAVED manual rect (the seed restores source "manual", and
// the merge starts from it — not from {}). Red if the reload mis-seeds the source or Auto merges from {}.
test("crop: after a reload, Auto all keeps a saved manual rect", async ({ page }) => {
  await openPdf(page);
  await page.getByTestId("tool-crop").click();
  await dragCrop(page, 0, 0.2, 0.2, 0.7, 0.7); // a manual crop on page 0
  await page.getByTestId("crop-save").click();
  await expect(page.getByTestId("file-tab-cropped").first()).toBeVisible();

  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await page.route("**/files/*/crop-detect", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ pages: { "0": { x: 0.1, y: 0.1, w: 0.3, h: 0.3, source: "auto" } }, hiddenMarks: [] }),
    }),
  );
  await page.getByTestId("tool-crop").click();
  await page.getByTestId("crop-auto-all").click();
  // The saved manual rect on page 0 is kept, not overwritten by the proposal.
  await expect(page.getByTestId("crop-notice")).toContainText("Kept your crop");
  await expect(page.getByTestId("crop-auto-tag-0")).toHaveCount(0);
});

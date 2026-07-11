/**
 * T35 — slow freehand at reduced opacity must not show alpha-stacked dark bands while
 * wet. The T06 incremental wet path bakes overlapping segments (WET_OVERLAP) each at
 * `globalAlpha=opacity`, so seams double-coat (α+α(1−α)) into darker bands. Fix: build
 * the cache + tail OPAQUE and blit the composed stroke ONCE at opacity (uniform alpha),
 * plus a capture-time min-distance filter so dense strokes stop storing near-duplicates.
 *
 * Reproducer: a dense synthetic freehand stroke at opacity 0.5, sampled MID-STROKE — no
 * wet pixel may be darker than the single-coat opacity (pre-fix the seams are). Second
 * test: a very dense stroke stores far fewer points than dispatched (the filter).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer, closeDrawer } from "./fullscreen-helpers";

test.use({ hasTouch: true });

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function shimPointerCapture(page: Page) {
  await page.addInitScript(() => {
    for (const m of ["setPointerCapture", "releasePointerCapture"] as const) {
      const orig = Element.prototype[m];
      Element.prototype[m] = function (this: Element, id: number) {
        try {
          return orig.call(this, id);
        } catch {
          /* synthetic pointer id */
        }
      };
    }
  });
}
async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}
async function createBandAndOpen(page: Page, bandName: string): Promise<string> {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  return page.url().split("/bands/")[1];
}
async function createSongAndOpen(page: Page, title: string): Promise<string> {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  return page.url().split("/songs/")[1];
}
async function uploadPdf(page: Page) {
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
}
async function armFreehand(page: Page, opacity: number) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
  await page.getByTestId("tool-freehand").click();
  // Set opacity via the native value setter so the controlled onChange fires.
  await page.getByTestId("style-opacity").evaluate((el, v) => {
    const set = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), "value")!.set!;
    set.call(el, String(v));
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
  }, opacity);
}

async function pt(page: Page, type: "pointerdown" | "pointermove" | "pointerup", x: number, y: number) {
  await page.evaluate(
    ({ type, x, y }) => {
      const el = document.querySelector('[data-testid="edit-canvas"]') as Element;
      el.dispatchEvent(
        new PointerEvent(type, {
          pointerId: 1,
          pointerType: "touch",
          isPrimary: true,
          clientX: x,
          clientY: y,
          button: type === "pointerup" ? -1 : 0,
          buttons: type === "pointerup" ? 0 : 1,
          bubbles: true,
          cancelable: true,
          composed: true,
        }),
      );
    },
    { type, x, y },
  );
}

// Max alpha among "core" (well-covered) wet pixels — excludes feathered stroke edges.
const maxCoreAlpha = (page: Page) =>
  page.getByTestId("edit-canvas").first().evaluate((c) => {
    const cv = c as HTMLCanvasElement;
    const { data } = cv.getContext("2d")!.getImageData(0, 0, cv.width, cv.height);
    let max = 0;
    for (let i = 3; i < data.length; i += 4) if (data[i] > 102 && data[i] > max) max = data[i];
    return max;
  });

test("wet freehand at 50% opacity has no alpha-stacked dark bands (T35)", async ({ page }) => {
  await shimPointerCapture(page);
  await register(page, `wa_${stamp()}`);
  await createBandAndOpen(page, `WABand ${stamp()}`);
  await createSongAndOpen(page, `WASong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await armFreehand(page, 0.5);

  // A long horizontal stroke, points spaced above the min-step so all are KEPT and many
  // cache segments (+ their overlapping seams) bake — the alpha-stacking pathology.
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const y = box.y + box.height * 0.28;
  const x0 = box.x + box.width * 0.2;
  const x1 = box.x + box.width * 0.7;
  await pt(page, "pointerdown", x0, y);
  const N = 80;
  for (let i = 1; i <= N; i++) await pt(page, "pointermove", x0 + ((x1 - x0) * i) / N, y);
  await page.waitForTimeout(200); // let the wet frames + segment bakes settle

  // No wet pixel may be darker than a single coat at 0.5 opacity (~127). Pre-fix the
  // seams stack to ~0.75 (~191); a 20% margin over 127 (~153) separates them cleanly.
  const max = await maxCoreAlpha(page);
  expect(max, `max wet core alpha ${max} must be ~opacity (no stacked bands)`).toBeLessThanOrEqual(153);

  await pt(page, "pointerup", x1, y);
});

test("capture-time min-distance filter thins a dense freehand stroke (T35)", async ({ page }) => {
  await shimPointerCapture(page);
  await register(page, `wf_${stamp()}`);
  const bandId = await createBandAndOpen(page, `WFBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `WFSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await armFreehand(page, 1);

  // A dense stroke: many moves spaced BELOW the min-step → most are dropped at capture.
  const box = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const y = box.y + box.height * 0.4;
  const x0 = box.x + box.width * 0.3;
  const x1 = box.x + box.width * 0.45; // short span, densely sampled
  await pt(page, "pointerdown", x0, y);
  const DISPATCHED = 200;
  for (let i = 1; i <= DISPATCHED; i++) await pt(page, "pointermove", x0 + ((x1 - x0) * i) / DISPATCHED, y);
  await pt(page, "pointerup", x1, y);
  await expect.poll(() => page.getByTestId("object-count").innerText()).toContain("1");

  const kept = await page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      const doc = (await r.json()) as { objects: { type: string; points: unknown[] }[] };
      return doc.objects.find((o) => o.type === "freehand")?.points.length ?? 0;
    },
    [bandId, songId],
  );
  // The stored object must be far thinner than the dispatched sample count (the diet).
  expect(kept, `kept ${kept} of ${DISPATCHED} dispatched points`).toBeGreaterThan(2);
  expect(kept).toBeLessThan(DISPATCHED * 0.5);
});

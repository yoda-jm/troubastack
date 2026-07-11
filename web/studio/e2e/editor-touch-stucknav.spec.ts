/**
 * T34 — a two-finger nav whose second finger lifts OFF-canvas must not jam the touch
 * editor in navigation forever.
 *
 * Field bug (VLL 2026-07-11): after a two-finger gesture where one finger lifted over
 * the floating chrome (its buttons are pointer-events:auto islands, so they swallow the
 * pointerup) or off-window, the canvas never saw that `pointerup`. The stale
 * `pointersRef` entry made every later single touch read as size>=2 → instant nav →
 * `cancelWetGesture()` → no stroke ever starts. Editing was dead until reload.
 *
 * Repro: raw touch PointerEvents; F2's lift is dispatched on `document.body` (the missed
 * up), then a fresh single-finger stroke must still commit. Fails pre-fix (the self-heal
 * on the primary pointerdown is what recovers it); the clean-lift control passes on both.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer, closeDrawer } from "./fullscreen-helpers";

test.use({ hasTouch: true });

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

// Synthetic pointerIds can't be captured — `setPointerCapture` throws NotFoundError,
// which would kill the draw branch (and the app's nav-capture) and make the spec lie.
// Shim both to no-op-on-throw, before any app code, on every navigation.
async function shimPointerCapture(page: Page) {
  await page.addInitScript(() => {
    for (const m of ["setPointerCapture", "releasePointerCapture"] as const) {
      const orig = Element.prototype[m];
      Element.prototype[m] = function (this: Element, id: number) {
        try {
          return orig.call(this, id);
        } catch {
          /* synthetic pointer id — not capturable in a dispatched PointerEvent */
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
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

/** Dispatch a raw touch PointerEvent. target "canvas" = the first edit-canvas; "body" =
 *  the missed off-canvas lift. */
async function touch(
  page: Page,
  target: "canvas" | "body",
  type: "pointerdown" | "pointermove" | "pointerup",
  id: number,
  x: number,
  y: number,
  isPrimary: boolean,
) {
  await page.evaluate(
    ({ target, type, id, x, y, isPrimary }) => {
      const el =
        target === "body"
          ? document.body
          : (document.querySelector('[data-testid="edit-canvas"]') as Element);
      el.dispatchEvent(
        new PointerEvent(type, {
          pointerId: id,
          pointerType: "touch",
          isPrimary,
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
    { target, type, id, x, y, isPrimary },
  );
}

/** Register → band → song → upload → editor, ensure an editable layer + freehand armed;
 *  returns the first pdf-page box for coordinates. */
async function setup(page: Page): Promise<{ x: number; y: number; width: number; height: number }> {
  await register(page, `sn_${stamp()}`);
  await createBandAndOpen(page, `SNBand ${stamp()}`);
  await createSongAndOpen(page, `SNSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await closeDrawer(page);
  await page.getByTestId("tool-freehand").click();
  return (await page.getByTestId("pdf-page").first().boundingBox())!;
}

async function stroke(page: Page, cx: number, cy: number, id: number) {
  await touch(page, "canvas", "pointerdown", id, cx, cy, true);
  await touch(page, "canvas", "pointermove", id, cx + 30, cy + 20, true);
  await touch(page, "canvas", "pointermove", id, cx + 60, cy + 40, true);
  await touch(page, "canvas", "pointerup", id, cx + 60, cy + 40, false);
}

test("touch: a nav finger lifting off-canvas does not jam the editor in nav (T34)", async ({
  page,
}) => {
  await shimPointerCapture(page);
  const box = await setup(page);
  const cx = box.x + box.width * 0.4;
  const cy = box.y + box.height * 0.28;
  const before = await objectCount(page);

  // Two-finger nav on the canvas …
  await touch(page, "canvas", "pointerdown", 1, cx - 40, cy, true);
  await touch(page, "canvas", "pointerdown", 2, cx + 40, cy, false);
  // … F1 lifts on canvas, but F2's up is MISSED — dispatched on document.body (as when
  // a finger lifts over a chrome button or off-window). The canvas never sees it.
  await touch(page, "canvas", "pointerup", 1, cx - 40, cy, false);
  await touch(page, "body", "pointerup", 2, cx + 40, cy, false);

  // A fresh single-finger stroke must still commit. Pre-fix, the stale F2 entry makes
  // this read as size>=2 → nav → no object; the primary-pointer self-heal recovers it.
  await stroke(page, cx, cy, 3);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
});

test("touch: clean two-finger nav then a single-finger stroke commits (control, T34)", async ({
  page,
}) => {
  await shimPointerCapture(page);
  const box = await setup(page);
  const cx = box.x + box.width * 0.4;
  const cy = box.y + box.height * 0.28;
  const before = await objectCount(page);

  // BOTH nav fingers lift ON canvas (no missed up) → no stale entry.
  await touch(page, "canvas", "pointerdown", 1, cx - 40, cy, true);
  await touch(page, "canvas", "pointerdown", 2, cx + 40, cy, false);
  await touch(page, "canvas", "pointerup", 1, cx - 40, cy, false);
  await touch(page, "canvas", "pointerup", 2, cx + 40, cy, false);

  await stroke(page, cx, cy, 3);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
});

test("touch: BOTH nav ups missed — the heal settles the live transform, not just the stroke (T34 #3)", async ({
  page,
}) => {
  await shimPointerCapture(page);
  const box = await setup(page);
  const cx = box.x + box.width * 0.4;
  const cy = box.y + box.height * 0.28;
  const before = await objectCount(page);
  const wrapTransform = () =>
    page.locator(".viewer-content").evaluate((el) => (el as HTMLElement).style.transform);

  // Two-finger nav WITH a pinch move (spread apart) so a live CSS transform is applied
  // to the pages wrapper …
  await touch(page, "canvas", "pointerdown", 1, cx - 40, cy, true);
  await touch(page, "canvas", "pointerdown", 2, cx + 40, cy, false);
  await touch(page, "canvas", "pointermove", 1, cx - 80, cy, false);
  await touch(page, "canvas", "pointermove", 2, cx + 80, cy, false);
  expect(await wrapTransform(), "the live nav should apply a CSS transform").not.toBe("");
  // … then BOTH fingers' ups are missed (dispatched off-canvas). nav stays live.
  await touch(page, "body", "pointerup", 1, cx - 80, cy, false);
  await touch(page, "body", "pointerup", 2, cx + 80, cy, false);

  // A fresh single-finger stroke must commit AND leave NO residual transform: the heal
  // must endGesture() the live nav (settle to a crisp raster), not just clear pointersRef.
  // Pre-condition-fix the stroke still commits but the transform stays stuck (blurry).
  await stroke(page, cx, cy, 3);
  await expect.poll(() => objectCount(page)).toBe(before + 1);
  expect(await wrapTransform(), "the live nav's transform must be settled (no residual)").toBe("");
});

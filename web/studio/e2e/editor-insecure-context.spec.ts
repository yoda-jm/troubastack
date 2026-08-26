/**
 * T32 — the product must work on a plain-HTTP (insecure) origin, and no client error
 * may die silently. Three guards:
 *
 *  1. newUuid() falls back to a valid v4 when crypto.randomUUID is absent (unit-level,
 *     via the shipped module).
 *  2. THE CLASS-KILLER: with crypto.randomUUID deleted (emulating VLL's
 *     http://…leligeour.net:8080 box), the standard draw flow still COMMITS an object.
 *     This fails on pre-fix code (buildObject threw on pointerup, nothing was sent) and
 *     passes post-fix. Every other e2e drives http://localhost, which browsers treat as
 *     SECURE even over plain HTTP — so this bug class was invisible by construction.
 *  3. The global error backstop surfaces any uncaught error / unhandled rejection as a
 *     dismissible banner (VLL: "it is not normal to just die silently").
 *
 * Sibling guard for the SAME bug class on the BAKE path (T99): `bake-insecure-origin.spec.ts`.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

// Emulate an insecure origin: crypto.randomUUID is absent (getRandomValues stays).
// Runs before every navigation, so it survives the reload in the draw flow.
const deleteRandomUUID = (page: Page) =>
  page.addInitScript(() => {
    try {
      Object.defineProperty(window.crypto, "randomUUID", { value: undefined, configurable: true });
    } catch {
      /* already shadowed */
    }
  });

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  await openDrawer(page, "layers");
}
const objectCount = (page: Page) =>
  page.getByTestId("object-count").innerText().then((t) => parseInt(t, 10));

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 12) {
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

test("newUuid falls back to a valid v4 when crypto.randomUUID is absent", async ({ page }) => {
  await page.goto("/");
  const { withApi, withoutApi, hadApi } = await page.evaluate(async () => {
    // A runtime dev-server URL, not a build-time module path — keep it a non-literal
    // string so TS doesn't try to resolve it (vite serves the transformed module).
    const editorUrl: string = "/src/editor" + ".ts";
    const mod = (await import(editorUrl)) as { newUuid: () => string };
    const hadApi = typeof crypto.randomUUID === "function";
    const withApi = [mod.newUuid(), mod.newUuid()];
    Object.defineProperty(window.crypto, "randomUUID", { value: undefined, configurable: true });
    const withoutApi = [mod.newUuid(), mod.newUuid(), mod.newUuid()];
    return { withApi, withoutApi, hadApi };
  });
  expect(hadApi).toBe(true); // localhost is a secure context — the happy path
  for (const id of [...withApi, ...withoutApi]) expect(id).toMatch(V4);
  expect(new Set(withoutApi).size).toBe(withoutApi.length); // unique
});

test("insecure origin (no crypto.randomUUID): drawing still commits an object", async ({ page }) => {
  await deleteRandomUUID(page);
  await register(page, `ic_${stamp()}`);
  await createBandAndOpen(page, `ICBand ${stamp()}`);
  await createSongAndOpen(page, `ICSong ${stamp()}`);
  await uploadPdf(page);
  await page.reload(); // deleteRandomUUID re-runs → still insecure after reload
  await openEditorReady(page);

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  const before = await objectCount(page);

  // Pre-fix this threw in buildObject on pointerup and nothing committed.
  await page.getByTestId("tool-freehand").click();
  await dragOnPage(page, 0.15, 0.6, 0.5, 0.75);
  await expect.poll(() => objectCount(page)).toBe(before + 1);

  // And the create path did NOT surface an error (no throw to catch).
  await expect(page.getByTestId("global-error")).toBeHidden();
  await expect(page.getByTestId("reject-notice")).toHaveCount(0);
});

test("global error backstop: uncaught error + rejection surface a dismissible banner", { tag: "@smoke" }, async ({
  page,
}) => {
  await register(page, `ge_${stamp()}`); // lands on /bands with the Shell mounted
  await page.evaluate(() => {
    setTimeout(() => {
      throw new Error("kaboom-uncaught");
    });
  });
  const banner = page.getByTestId("global-error");
  await expect(banner).toBeVisible();
  await expect(banner).toContainText("kaboom-uncaught");
  await page.getByTestId("global-error-dismiss").click();
  await expect(banner).toBeHidden();

  // An unhandled promise rejection surfaces too.
  await page.evaluate(() => {
    void Promise.reject(new Error("kaboom-rejection"));
  });
  await expect(page.getByTestId("global-error")).toContainText("kaboom-rejection");
});

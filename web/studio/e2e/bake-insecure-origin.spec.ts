/**
 * T99 guard (T32 class): the bake must still fire on an INSECURE origin. `crypto.randomUUID`
 * is secure-context-only — `undefined` on the plain-http:// LAN IP box every band member who
 * isn't at the server reaches TroubaStack through. The bake dialog mints its progress id there;
 * if it used `crypto.randomUUID` the call would throw before the POST and the bake would
 * silently never fire. This deletes `crypto.randomUUID` before load (as editor-insecure-context
 * does for the draw path) and proves the POST still goes out, carrying a valid v4 id from the
 * fallback. Fails on `crypto.randomUUID()`, passes on `newUuid()`.
 *
 * Sibling guard for the SAME bug class on the annotate/draw path: `editor-insecure-context.spec.ts`.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const V4 = /^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;

// Emulate the insecure LAN origin: crypto.randomUUID absent (getRandomValues stays). Re-runs
// before every navigation, so it survives the SPA's internal route changes.
const deleteRandomUUID = (page: Page) =>
  page.addInitScript(() => {
    try {
      Object.defineProperty(window.crypto, "randomUUID", { value: undefined, configurable: true });
    } catch {
      /* already shadowed */
    }
  });

test("insecure origin (no crypto.randomUUID): the bake still fires, with a valid v4 id", async ({
  page,
}) => {
  await deleteRandomUUID(page);

  await page.goto("/register");
  await page.getByTestId("username").fill(`ib_${stamp()}`);
  await page.getByTestId("displayName").fill("D");
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);

  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`IBBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];

  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("The Open Road");
  await page.getByTestId("create-song").click();

  await page.goto(`/bands/${bandId}/setlists`);
  await page.getByTestId("setlist-name").fill("Gig");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  await page.getByTestId("add-item-song").selectOption({ label: "The Open Road" });
  await page.getByTestId("add-item").click();

  // Capture whether the bake POST fired, and the id it carried. Fulfil a fake concert so the
  // flow completes without a real bake.
  let posted = false;
  let suppliedId: string | undefined;
  await page.route("**/setlists/*/bake", async (route) => {
    posted = true;
    suppliedId = route.request().headers()["x-trouba-bake-id"];
    await route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: suppliedId }) });
  });
  // T103: the outcome comes from the poll now — report succeeded so the dialog closes.
  await page.route("**/bakes/*/progress", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ state: "succeeded", done: 1, total: 1 }) }),
  );

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click();

  // The load-bearing assertion: on pre-fix code confirm() throws on crypto.randomUUID and the
  // POST never fires (dialog hangs open). Post-fix it goes out, with a valid v4 from the fallback.
  await expect.poll(() => posted).toBe(true);
  await expect(page.getByTestId("bake-dialog")).toBeHidden();
  expect(suppliedId).toMatch(V4);
  // And no silent death: the global error backstop stayed quiet.
  await expect(page.getByTestId("global-error")).toBeHidden();
});

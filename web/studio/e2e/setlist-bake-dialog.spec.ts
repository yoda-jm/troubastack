/**
 * P205 Stage 1 guard: baking goes through a dialog that captures which layers are
 * default-ON — never silently. Toggling a layer off must send `layerDefaults` with that
 * layer false to the bake API (the server stamps `default_on=false`); mandatory layers
 * are locked on. Red-first: pre-P205 baking has no dialog.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

async function register(page: Page, u: string) {
  await page.goto("/register");
  await page.getByTestId("username").fill(u);
  await page.getByTestId("displayName").fill(`D ${u}`);
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

test("bake dialog captures per-layer default-on; toggling one off sends it false (P205)", async ({
  page,
}) => {
  await register(page, `bd_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`BDBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("Wonderwall");
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: "Wonderwall" }).first().click();
  const songId = page.url().split("/songs/")[1];
  const me = await page.evaluate(async () => {
    const r = await fetch("/api/me", { credentials: "include" });
    return ((await r.json()) as { user: { id: string } }).user.id;
  });

  // A mandatory "Cues" layer + an optional "My notes" layer (the annotation import
  // provisions layers; a file id isn't needed for layers to exist).
  await page.evaluate(
    async ([b, s, me]) => {
      await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          layers: [
            { id: "L-cues", fileId: "f", name: "Cues", ownerId: "_shared_", zone: "conductor", order: 0, access: "ro", mandatory: true, roleTag: "conductor" },
            { id: "L-mine", fileId: "f", name: "My notes", ownerId: me, zone: "personal", order: 1, access: "rw", mandatory: false, roleTag: "" },
          ],
          objects: [],
        }),
      });
    },
    [bandId, songId, me] as const,
  );

  // Setlist with the song.
  await page.goto(`/bands/${bandId}/setlists`);
  await page.getByTestId("setlist-name").fill("Gig");
  await page.getByTestId("create-setlist").click();
  await page.getByTestId("setlist-link").filter({ hasText: "Gig" }).click();
  await page.getByTestId("add-item-song").selectOption({ label: "Wonderwall" });
  await page.getByTestId("add-item").click();

  // Intercept the bake POST → capture its body; fulfill with a fake concert so the
  // flow completes without a real (poppler) bake.
  let body: { layerDefaults?: Record<string, boolean> } | null = null;
  await page.route("**/setlists/*/bake**", async (route) => {
    body = route.request().postDataJSON();
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ concertId: "c", name: "c", concertRev: "1", currentRev: 1, bakedAt: "0", downloadUrl: "/x" }),
    });
  });

  // Open the dialog: it lists both layers; Cues is locked ON (mandatory).
  await page.getByTestId("bake-setlist").click();
  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  const cues = page.locator('[data-testid="bake-dialog-layer"][data-layer="Cues"] [data-testid="bake-dialog-toggle"]');
  const mine = page.locator('[data-testid="bake-dialog-layer"][data-layer="My notes"] [data-testid="bake-dialog-toggle"]');
  await expect(cues).toBeChecked();
  await expect(cues).toBeDisabled(); // mandatory → locked on
  await expect(mine).toBeChecked();

  // Toggle "My notes" off, confirm → the bake POST carries it false, Cues true.
  await mine.uncheck();
  await page.getByTestId("bake-dialog-confirm").click();
  await expect.poll(() => body).not.toBeNull();
  expect(body!.layerDefaults).toEqual({ Cues: true, "My notes": false });
});

/**
 * T55 guard: the "Drawing on: <layer>" indicator declares its AUDIENCE — 👤 Mine for a
 * personal layer, 👥 Band for a shared/conductor layer (conductor labelled). So every
 * stroke/stamp shows whether it's private or band-visible at the moment it lands.
 * Red-first: pre-T55 the indicator has no audience tag.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer } from "./fullscreen-helpers";
import { stamp, register } from "./setup-helpers";

const PDF = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function myId(page: Page) {
  return page.evaluate(async () => {
    const r = await fetch("/api/me", { credentials: "include" });
    return ((await r.json()) as { user: { id: string } }).user.id;
  });
}
async function importDoc(page: Page, bandId: string, songId: string, doc: unknown) {
  return page.evaluate(
    async ([b, s, body]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return r.ok;
    },
    [bandId, songId, doc] as const,
  );
}

test("Drawing-on indicator shows 👤 Mine for a personal layer, 👥 Band for shared (T55)", async ({
  page,
}) => {
  await register(page, `aud_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`AudBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill("The Open Road");
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: "The Open Road" }).first().click();
  const songId = page.url().split("/songs/")[1];
  const me = await myId(page);

  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF);
  await page.getByTestId("file-upload").click();
  await page.getByTestId("file-row").first().waitFor();
  const fileId = await page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      return ((await r.json()) as { files: { id: string }[] }).files[0].id;
    },
    [bandId, songId] as const,
  );

  // A shared (band) layer + a personal (mine) layer, both editable.
  await importDoc(page, bandId, songId, {
    layers: [
      { id: "L-shared", fileId, name: "Section markings", ownerId: "_shared_", zone: "shared", order: 0, access: "rw", mandatory: false, roleTag: "" },
      { id: "L-mine", fileId, name: "My notes", ownerId: me, zone: "personal", order: 1, access: "rw", mandatory: false, roleTag: "" },
    ],
    objects: [],
  });
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await openDrawer(page, "layers");

  const indicator = page.getByTestId("active-layer-indicator");
  const tag = indicator.getByTestId("audience-tag");

  // Select the personal layer → 👤 Mine.
  await page.getByTestId("active-layer").selectOption({ label: "My notes" });
  await expect(tag).toHaveAttribute("data-audience", "mine");
  await expect(tag).toContainText("Mine");

  // Select the shared layer → 👥 Band.
  await page.getByTestId("active-layer").selectOption({ label: "Section markings" });
  await expect(tag).toHaveAttribute("data-audience", "band");
  await expect(tag).toContainText("Band");
});

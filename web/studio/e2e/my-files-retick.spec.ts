/**
 * T154 — re-ticking a file in "my files" must take the SECOND time too.
 *
 * VLL: "selecting a song for me pins it the first time, but then after deselecting it does not reselect it
 * the second time." A toggle that only works once is worse than one that never works.
 *
 * The server round-trips [A] -> [] -> [A] correctly (Fable proved it), so this is a Studio client bug. The
 * spec drives the FULL three-step sequence — include, exclude, include again — and asserts both the live
 * checkbox and the PERSISTED state (reload). Two files (A, B) with B always kept, so unticking A never
 * empties the selection (the empty-selection confirmation is out of scope).
 *
 * ⟨R1⟩ teeth: steps 1–2 alone pass on the current code; only step 3 (re-include) is red today.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register } from "./setup-helpers";

const PDF = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function openMineTab(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("details-tab-mine").click();
  await expect(page.getByTestId("my-files-panel")).toBeVisible();
}

test("my-files: re-ticking a file takes the second time (include → exclude → include)", async ({ page }) => {
  await register(page, `myfr2_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`RetickBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`RetickSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  const songId = page.url().split("/songs/")[1];

  // Two pool files, named A/B in order.
  await page.getByTestId("my-files-edit").click();
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }
  const ids = await page.evaluate(async ([b, s]) => {
    const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
    return ((await r.json()) as { files: { id: string }[] }).files.map((f) => f.id);
  }, [bandId, songId]);
  await page.evaluate(
    async ([b, s, fids]) => {
      const ns = ["fileA", "fileB"];
      for (let i = 0; i < fids.length; i++) {
        await fetch(`/api/bands/${b}/songs/${s}/files/${fids[i]}`, {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ filename: ns[i], displayOrder: i }),
        });
      }
      // Seed an explicit SAVED-EMPTY selection (PUT []), the state at the heart of the bug: re-including
      // from saved-empty is the "second time" VLL says fails ([]→[A]→[]→[A]). A starts excluded.
      await fetch(`/api/bands/${b}/songs/${s}/my-files`, {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fileIds: [] }),
      });
    },
    [bandId, songId, ids] as const,
  );

  await page.reload();
  await openMineTab(page);
  await expect(page.getByTestId("my-files-row")).toHaveCount(2);
  const rowA = () => page.getByTestId("my-files-row").filter({ hasText: "fileA" });
  const boxA = () => rowA().getByTestId("my-files-include");

  // Persisted state of A on the server, as the strip/editor would reload it.
  const persistedA = async () =>
    page.evaluate(
      async ([b, s, a]) => {
        const r = await fetch(`/api/bands/${b}/songs/${s}/my-files`, { credentials: "include" });
        const j = (await r.json()) as { files: { id: string }[] };
        return j.files.some((f) => f.id === a);
      },
      [bandId, songId, ids[0]] as const,
    );

  await expect(boxA()).not.toBeChecked(); // A starts excluded

  // STEP 1 — include A. Live checked, and persisted.
  await boxA().click();
  await expect(boxA()).toBeChecked();
  await expect.poll(persistedA).toBe(true);

  // STEP 2 — exclude A. Live unchecked, and persisted.
  await boxA().click();
  await expect(boxA()).not.toBeChecked();
  await expect.poll(persistedA).toBe(false);

  // STEP 3 — include A AGAIN. This is the one VLL says fails. Live checked, and persisted.
  await boxA().click();
  await expect(boxA()).toBeChecked();
  await expect.poll(persistedA).toBe(true);

  // And it survives a reload (the real "does not take" symptom).
  await page.reload();
  await openMineTab(page);
  await expect(rowA().getByTestId("my-files-include")).toBeChecked();
});

test("my-files: untick then immediately re-tick, with another file kept (Fable's corrected T154 repro)", async ({
  page,
}) => {
  await register(page, `myfr4_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`RetickBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`RetickSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  const songId = page.url().split("/songs/")[1];

  await page.getByTestId("my-files-edit").click();
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }
  const ids = await page.evaluate(async ([b, s]) => {
    const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
    return ((await r.json()) as { files: { id: string }[] }).files.map((f) => f.id);
  }, [bandId, songId]);
  await page.evaluate(
    async ([b, s, fids]) => {
      const ns = ["fileA", "fileB"];
      for (let i = 0; i < fids.length; i++) {
        await fetch(`/api/bands/${b}/songs/${s}/files/${fids[i]}`, {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ filename: ns[i], displayOrder: i }),
        });
      }
      // BOTH ticked to start — the state VLL is in (nothing empty).
      await fetch(`/api/bands/${b}/songs/${s}/my-files`, {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fileIds: fids }),
      });
    },
    [bandId, songId, ids] as const,
  );

  await page.reload();
  await openMineTab(page);
  const rowA = () => page.getByTestId("my-files-row").filter({ hasText: "fileA" });
  const boxA = () => rowA().getByTestId("my-files-include");
  await expect(boxA()).toBeChecked();

  // Delay the PUT so the untick's write is still in flight when the re-tick fires — the untick→re-tick
  // window with a real network in between, and B kept so nothing is ever empty.
  await page.route("**/my-files", async (route) => {
    if (route.request().method() === "PUT") await new Promise((r) => setTimeout(r, 1000));
    await route.continue();
  });

  // Untick A (→ PUT [B], NOT empty), then IMMEDIATELY re-tick A — same screen.
  await boxA().evaluate((el) => {
    (el as HTMLElement).click();
    (el as HTMLElement).click();
  });

  await expect(boxA()).toBeChecked();
  await expect
    .poll(
      () =>
        page.evaluate(
          async ([b, s, a]) => {
            const r = await fetch(`/api/bands/${b}/songs/${s}/my-files`, { credentials: "include" });
            const j = (await r.json()) as { files: { id: string }[] };
            return j.files.some((f) => f.id === a);
          },
          [bandId, songId, ids[0]] as const,
        ),
      { timeout: 8000 },
    )
    .toBe(true);
  await page.reload();
  await openMineTab(page);
  await expect(rowA().getByTestId("my-files-include")).toBeChecked();
});

test("my-files: rapid re-tick under a slow write still persists (race)", async ({ page }) => {
  await register(page, `myfr3_${stamp()}`);
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`RaceBand ${stamp()}`);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").first().click();
  const bandId = page.url().split("/bands/")[1];
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`RaceSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  const songId = page.url().split("/songs/")[1];

  await page.getByTestId("my-files-edit").click();
  for (let i = 0; i < 2; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }
  const ids = await page.evaluate(async ([b, s]) => {
    const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
    return ((await r.json()) as { files: { id: string }[] }).files.map((f) => f.id);
  }, [bandId, songId]);
  await page.evaluate(
    async ([b, s, fids]) => {
      await fetch(`/api/bands/${b}/songs/${s}/files/${fids[0]}`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ filename: "fileA", displayOrder: 0 }),
      });
      await fetch(`/api/bands/${b}/songs/${s}/my-files`, {
        method: "PUT",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fileIds: [fids[0]] }), // A starts included
      });
    },
    [bandId, songId, ids] as const,
  );

  await page.reload();
  await openMineTab(page);
  const rowA = () => page.getByTestId("my-files-row").filter({ hasText: "fileA" });
  const boxA = () => rowA().getByTestId("my-files-include");
  await expect(boxA()).toBeChecked();

  // Delay the my-files PUT so the untick's write is still in flight when the re-tick fires (the race
  // window). Coalescing must send [A] LAST — the user's final intent.
  await page.route("**/my-files", async (route) => {
    if (route.request().method() === "PUT") {
      await new Promise((r) => setTimeout(r, 1200));
    }
    await route.continue();
  });

  await boxA().click(); // untick (PUT [] — delayed)
  await boxA().click(); // re-tick immediately (PUT [A]) — during the delay window
  await expect(boxA()).toBeChecked();
  await expect
    .poll(
      () =>
        page.evaluate(
          async ([b, s, a]) => {
            const r = await fetch(`/api/bands/${b}/songs/${s}/my-files`, { credentials: "include" });
            const j = (await r.json()) as { files: { id: string }[] };
            return j.files.some((f) => f.id === a);
          },
          [bandId, songId, ids[0]] as const,
        ),
      { timeout: 8000 },
    )
    .toBe(true);

  await page.reload();
  await openMineTab(page);
  await expect(rowA().getByTestId("my-files-include")).toBeChecked();
});

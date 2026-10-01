/**
 * T180 — song tag entry: the chip gesture, the counted suggestions, and the bounded vocabulary cloud.
 * Every test maps to a §7 acceptance line, and each asserts on the STORED array (via the API), never on
 * the rendered chip, because "one tag with a space" is a fact about storage, not about pixels.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

/** The song's stored tags, straight from the API (cookie auth in the page context). */
async function storedTags(page: Page, bandId: string, songId: string): Promise<string[]> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs`, { credentials: "include" });
      const j = (await r.json()) as { songs: { id: string; tags?: string[] }[] };
      return j.songs.find((x) => x.id === s)?.tags ?? [];
    },
    [bandId, songId],
  );
}

/** Create a song via the API, optionally with tags, and return its id. Lets a seeded test navigate
 *  straight to the editor instead of hunting a link in a long list. */
async function apiSong(page: Page, bandId: string, title: string, tags: string[]): Promise<string> {
  return page.evaluate(
    async ([b, t, tg]) => {
      const c = await fetch(`/api/bands/${b}/songs`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: t }),
      });
      const { song } = (await c.json()) as { song: { id: string } };
      if (tg.length) {
        await fetch(`/api/bands/${b}/songs/${song.id}`, {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ tags: tg }),
        });
      }
      return song.id;
    },
    [bandId, title, tags] as const,
  );
}

/** Seed a song carrying `tags`, via the API, so a band builds a vocabulary without UI churn. */
async function seedSong(page: Page, bandId: string, title: string, tags: string[]): Promise<void> {
  await page.evaluate(
    async ([b, t, tg]) => {
      const c = await fetch(`/api/bands/${b}/songs`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ title: t }),
      });
      const { song } = (await c.json()) as { song: { id: string } };
      await fetch(`/api/bands/${b}/songs/${song.id}`, {
        method: "PATCH",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ tags: tg }),
      });
    },
    [bandId, title, tags] as const,
  );
}

async function openDetails(page: Page) {
  await page.getByTestId("my-files-edit").click();
}

test("the commit gesture is the delimiter: a space is data, a trailing comma makes no empty tag", async ({
  page,
}) => {
  await register(page, `tags_${stamp()}`);
  const band = await createBandAndOpen(page, `TagBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `TagSong ${stamp()}`);
  await openDetails(page);

  const buf = page.getByTestId("tag-buffer");
  // A multi-word tag committed with Enter — one tag, space kept.
  await buf.click();
  await buf.fill("slow blues");
  await buf.press("Enter");
  // Another via comma, plus a TRAILING comma that must create no empty tag.
  await buf.fill("encore,");
  await buf.press(",");
  await buf.press("Enter"); // buffer is empty here → no-op, not an empty tag

  await expect(page.getByTestId("tag-chip")).toHaveText(["slow blues×", "encore×"]);

  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();

  // The claim that matters is about STORAGE: exactly two tags, the space preserved, no empty string.
  expect(await storedTags(page, band.id, songId)).toEqual(["slow blues", "encore"]);
});

test("backspace on an empty input removes exactly the last chip", async ({ page }) => {
  await register(page, `tags_${stamp()}`);
  const band = await createBandAndOpen(page, `TagBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `TagSong ${stamp()}`);
  await openDetails(page);

  const buf = page.getByTestId("tag-buffer");
  for (const t of ["a", "b", "c"]) {
    await buf.fill(t);
    await buf.press("Enter");
  }
  await expect(page.getByTestId("tag-chip")).toHaveCount(3);
  await buf.press("Backspace"); // input already empty
  await expect(page.getByTestId("tag-chip")).toHaveText(["a×", "b×"]);

  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();
  expect(await storedTags(page, band.id, songId)).toEqual(["a", "b"]);
});

test("suggestions carry a usage count, and a case variant joins the existing spelling", async ({ page }) => {
  await register(page, `tags_${stamp()}`);
  const band = await createBandAndOpen(page, `TagBand ${stamp()}`);
  // Two seeded songs carry "encore" (count 2). The case variant exists only as the TYPED text below —
  // the point is that typing it joins the stored spelling instead of minting a second.
  await seedSong(page, band.id, `S1 ${stamp()}`, ["encore", "bridge"]);
  await seedSong(page, band.id, `S2 ${stamp()}`, ["encore"]);
  const songId = await createSongAndOpen(page, `TagSong ${stamp()}`);
  await openDetails(page);

  const buf = page.getByTestId("tag-buffer");
  await buf.click();
  await buf.fill("Encore"); // the FULL case variant folds to "encore"

  const sugg = page.getByTestId("tag-suggestion").filter({ hasText: "encore" });
  await expect(sugg).toBeVisible();
  // ⟨D2⟩ the count equals the number of songs carrying it (2), not decoration.
  await expect(sugg.getByTestId("tag-suggestion-count")).toHaveText("2");
  // ⟨D3⟩ typing a case variant offers NO "Create" row — it must not mint a second spelling.
  await expect(page.getByTestId("tag-create")).toHaveCount(0);

  await sugg.click();
  // The chip is the EXISTING spelling, not what was typed.
  await expect(page.getByTestId("tag-chip")).toHaveText(["encore×"]);

  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();
  expect(await storedTags(page, band.id, songId)).toEqual(["encore"]);
});

test("a partial word + a CLOUD click saves only the cloud tag, never the fragment (Fable ⟨1⟩)", async ({
  page,
}) => {
  await register(page, `tags_${stamp()}`);
  const band = await createBandAndOpen(page, `TagBand ${stamp()}`);
  await seedSong(page, band.id, `S1 ${stamp()}`, ["encore"]); // vocabulary: encore (1)
  const songId = await createSongAndOpen(page, `TagSong ${stamp()}`);
  await openDetails(page);

  // Type a partial word, then pick the full tag from the CLOUD (not the suggestion list). The blur that the
  // cloud click triggers must NOT commit "enc" as its own tag — the reader picked the existing tag and must
  // not get an invented one saved beside it.
  const buf = page.getByTestId("tag-buffer");
  await buf.click();
  await buf.fill("enc");
  await page.getByTestId("tag-cloud-item").filter({ hasText: "encore" }).click();

  await expect(page.getByTestId("tag-chip")).toHaveText(["encore×"]);

  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();
  // The claim, on storage: exactly the cloud tag, no fragment.
  expect(await storedTags(page, band.id, songId)).toEqual(["encore"]);
});

test("the cloud is bounded to N and reveals the rest on request", async ({ page }) => {
  await register(page, `tags_${stamp()}`);
  const band = await createBandAndOpen(page, `TagBand ${stamp()}`);
  // 15 distinct tags across seeded songs → more than the cloud's default of 12.
  const many = Array.from({ length: 15 }, (_, i) => `tag${String(i).padStart(2, "0")}`);
  for (let i = 0; i < many.length; i++) await seedSong(page, band.id, `S${i} ${stamp()}`, [many[i]]);
  const songId = await apiSong(page, band.id, `TagSong ${stamp()}`, []);
  await page.goto(`/bands/${band.id}/songs/${songId}`);
  await openDetails(page);

  await expect(page.getByTestId("tag-cloud-item")).toHaveCount(12);
  await expect(page.getByTestId("tag-cloud-more")).toBeVisible();
  await page.getByTestId("tag-cloud-more").click();
  await expect(page.getByTestId("tag-cloud-item")).toHaveCount(15);
  await expect(page.getByTestId("tag-cloud-more")).toHaveCount(0);
});

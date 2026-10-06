/**
 * T182 — the band Tags panel: rename (which merges and dedups), the ⟨D3⟩ adopt-existing offer, delete, the
 * ⟨D5⟩ consequence-in-numbers confirmation, and the ⟨D2⟩ tag-click that filters the song list. Each write
 * is asserted on the STORED tags (read back through the API), not only on the refreshed panel.
 */
import { test, expect, type Page } from "@playwright/test";
import { createBandAndOpen, register, stamp } from "./setup-helpers";

async function seed(page: Page, bandId: string, title: string, tags: string[]) {
  await page.evaluate(
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
    },
    [bandId, title, tags] as const,
  );
}

/** Read a band's stored tags back from the server, keyed by song title — the ground truth a rename must
 * reach, independent of what the panel shows. */
async function storedTags(page: Page, bandId: string): Promise<Record<string, string[]>> {
  return page.evaluate(async (b) => {
    const r = await fetch(`/api/bands/${b}/songs`, { credentials: "include" });
    const { songs } = (await r.json()) as { songs: { title: string; tags?: string[] }[] };
    const out: Record<string, string[]> = {};
    for (const s of songs) out[s.title] = s.tags ?? [];
    return out;
  }, bandId);
}

// Match the row by an EXACT, case-sensitive tag name — a string hasText is case-insensitive and would make
// "Encore" also match the "encore" row (two stored spellings are the whole point of this panel).
function rowFor(page: Page, tag: string) {
  const esc = tag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const name = page.getByTestId("tag-row-name").filter({ hasText: new RegExp(`^${esc}$`) });
  return page.locator('[data-testid="tag-row"]').filter({ has: name });
}

test("rename merges onto an existing spelling and dedups within a song (stored-tags asserted)", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `tags${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // A carries BOTH spellings (the merge fixture); B carries only Encore; C carries neither.
  await seed(page, band.id, "Alpha", ["ballad", "Encore", "encore"]);
  await seed(page, band.id, "Beta", ["Encore", "closer"]);
  await seed(page, band.id, "Gamma", ["ballad"]);
  await page.reload();

  await expect(page.getByTestId("tags-panel")).toBeVisible();
  // One row per stored spelling: Encore and encore are distinct rows.
  await expect(rowFor(page, "Encore")).toBeVisible();
  await expect(rowFor(page, "encore")).toBeVisible();

  // Rename "Encore" -> "encore". Typed spelling exactly matches an existing one, so it is a merge, no adopt.
  await rowFor(page, "Encore").getByTestId("tag-rename").click();
  await page.getByTestId("app-dialog-input").fill("encore");
  await page.getByTestId("app-dialog-confirm").click();
  // The consequence dialog states EXACT numbers AND that it cannot be undone (⟨D5⟩). Alpha carries both
  // spellings, so the union is 2 distinct songs, not 1+2 = 3 (Fable ⟨1⟩: never double-count the overlap).
  await expect(page.getByTestId("tag-consequence")).toContainText(
    '"Encore" will be merged into "encore": 2 songs change, "encore" will be on 2.',
  );
  await expect(page.getByTestId("app-dialog-body")).toContainText("cannot be undone");
  await page.getByTestId("app-dialog-confirm").click();

  await expect(page.getByTestId("tags-notice")).toContainText(/Merged on 2 songs/);

  // Stored tags: Alpha has exactly one encore and no Encore; Beta has encore; Gamma untouched.
  const stored = await storedTags(page, band.id);
  expect(stored["Alpha"].filter((t) => t === "encore")).toHaveLength(1);
  expect(stored["Alpha"]).not.toContain("Encore");
  expect(stored["Beta"]).toContain("encore");
  expect(stored["Gamma"]).toEqual(["ballad"]);
  // The merged row is gone; one "encore" row remains.
  await expect(rowFor(page, "Encore")).toHaveCount(0);
});

test("⟨D3⟩ a new spelling that folds onto an existing one is offered the existing spelling", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `adopt${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  await seed(page, band.id, "Alpha", ["encore"]);
  await seed(page, band.id, "Beta", ["intro"]);
  await page.reload();

  // Rename "intro" -> "ENCORE": folds onto the existing "encore" (a different spelling) → adopt offer.
  await rowFor(page, "intro").getByTestId("tag-rename").click();
  await page.getByTestId("app-dialog-input").fill("ENCORE");
  await page.getByTestId("app-dialog-confirm").click();
  // The adopt dialog offers the existing spelling.
  await expect(page.getByTestId("app-dialog-body")).toContainText("“encore” already exists");
  await page.getByTestId("app-dialog-confirm").click(); // Use "encore"
  // Then the merge consequence, then confirm.
  await expect(page.getByTestId("tag-consequence")).toContainText('into "encore"');
  await page.getByTestId("app-dialog-confirm").click();

  const stored = await storedTags(page, band.id);
  // Beta adopted the EXISTING spelling, not the typed "ENCORE".
  expect(stored["Beta"]).toContain("encore");
  expect(stored["Beta"]).not.toContain("ENCORE");
});

test("⟨D4⟩ delete removes a spelling from every song that carries it", async ({ page }) => {
  const who = stamp();
  await register(page, `del${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  await seed(page, band.id, "Alpha", ["encore", "needs work"]);
  await seed(page, band.id, "Beta", ["needs work"]);
  await page.reload();

  await rowFor(page, "needs work").getByTestId("tag-delete").click();
  await expect(page.getByTestId("tag-consequence")).toContainText('"needs work" will be removed from 2 songs');
  await expect(page.getByTestId("app-dialog-body")).toContainText("cannot be undone");
  await page.getByTestId("app-dialog-confirm").click();
  await expect(page.getByTestId("tags-notice")).toContainText(/Removed on 2 songs/);

  const stored = await storedTags(page, band.id);
  expect(stored["Alpha"]).toEqual(["encore"]);
  expect(stored["Beta"]).toEqual([]);
  await expect(rowFor(page, "needs work")).toHaveCount(0);
});

test("⟨D2⟩ clicking a tag on a small band shows the search box with its chip and filters the list", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `pick${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // Small band (≤12): the song search box is hidden until something filters.
  await seed(page, band.id, "Alpha", ["encore"]);
  await seed(page, band.id, "Beta", ["encore"]);
  await seed(page, band.id, "Gamma", ["ballad"]);
  await page.reload();

  await expect(page.getByTestId("songs-filter")).toHaveCount(0); // hidden on a small band
  await rowFor(page, "encore").getByTestId("tag-row-name").click();

  await expect(page.getByTestId("songs-filter")).toBeVisible(); // §3: box shows whatever the count
  await expect(page.getByTestId("search-chip")).toHaveText(["encore×"]);
  await expect(page.getByTestId("song-link")).toHaveText([/Alpha/, /Beta/]); // filtered to encore songs
});

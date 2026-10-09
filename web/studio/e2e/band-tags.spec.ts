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

// T188 — rename/merge/delete live in Manage mode; open it before using rowFor().
async function openManage(page: Page) {
  await page.getByTestId("tags-manage").click();
  await page.getByTestId("tag-list").waitFor();
}
// A chip by exact tag name (chip text is name + count; match the name immediately followed by a digit).
function chipFor(page: Page, tag: string) {
  const esc = tag.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  return page.getByTestId("tag-chip").filter({ hasText: new RegExp(`^${esc}\\d`) });
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
  await openManage(page);
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
  await openManage(page);

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
  await openManage(page);

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
  await chipFor(page, "encore").click(); // chip mode is the default; a chip click filters

  await expect(page.getByTestId("songs-filter")).toBeVisible(); // §3: box shows whatever the count
  await expect(page.getByTestId("search-chip")).toHaveText(["encore×"]);
  await expect(page.getByTestId("song-link")).toHaveText([/Alpha/, /Beta/]); // filtered to encore songs
});

// --- T188: compact chip cloud, natural order, Manage mode ---

async function chipNames(page: Page): Promise<string[]> {
  return page.$$eval('[data-testid="tag-chip"]', (els) =>
    els.map((e) => (e.childNodes[0]?.textContent ?? "").trim()),
  );
}

test("T188 ⟨D2⟩ chips are A–Z by default (a family in family order), Most-used reorders, and it persists", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `order${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // Counts chosen so count-descending is NOT s1, s2, s10: s2=3 > s1=2 > s10=1 (and pop=2).
  await seed(page, band.id, "A", ["s2", "pop"]);
  await seed(page, band.id, "B", ["s2", "s1"]);
  await seed(page, band.id, "C", ["s2", "s1"]);
  await seed(page, band.id, "D", ["s10", "pop"]);
  await page.reload();
  await expect(page.getByTestId("tags-panel")).toBeVisible();

  // Default A–Z: the family reads s1, s2, s10 (numeric-aware), NOT count order.
  let names = await chipNames(page);
  expect(names.indexOf("s1")).toBeLessThan(names.indexOf("s2"));
  expect(names.indexOf("s2")).toBeLessThan(names.indexOf("s10"));
  await expect(page.getByTestId("tags-order-az")).toHaveAttribute("aria-pressed", "true");

  // Most used: count order — s2 (3) leads; s2 before s1.
  await page.getByTestId("tags-order-used").click();
  names = await chipNames(page);
  expect(names[0]).toBe("s2");
  expect(names.indexOf("s2")).toBeLessThan(names.indexOf("s1"));

  // Manage honours the SAME order (shared `ordered`), never the server's byte order (Fable, T188 GO): in
  // Most-used, the rows lead with s2 and keep s2 < s1 < s10 — counts that discriminate (3 > 2 > 1).
  await openManage(page);
  const rowNames = await page.getByTestId("tag-row-name").allInnerTexts();
  expect(rowNames[0]).toBe("s2");
  expect(rowNames.indexOf("s2")).toBeLessThan(rowNames.indexOf("s1"));
  expect(rowNames.indexOf("s1")).toBeLessThan(rowNames.indexOf("s10"));

  // The Most-used choice survives a reload (localStorage); Manage does NOT — the panel reopens in chip mode.
  await page.reload();
  await expect(page.getByTestId("tags-order-used")).toHaveAttribute("aria-pressed", "true");
  await expect(page.getByTestId("tag-chip").first()).toBeVisible();
  expect((await chipNames(page))[0]).toBe("s2");
});

test("T188 ⟨D1⟩ chip mode has no Rename/Delete; Manage reveals the rows in the selected order", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `manage${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  await seed(page, band.id, "A", ["s2"]);
  await seed(page, band.id, "B", ["s1"]);
  await seed(page, band.id, "C", ["s10"]);
  await page.reload();
  await expect(page.getByTestId("tags-panel")).toBeVisible();

  // Chip mode (default): no manage affordances in the DOM.
  await expect(page.getByTestId("tag-rename")).toHaveCount(0);
  await expect(page.getByTestId("tag-delete")).toHaveCount(0);
  await expect(page.getByTestId("tag-row")).toHaveCount(0);

  // Manage → the rows appear, in the A–Z (default) order.
  await openManage(page);
  await expect(page.getByTestId("tag-row")).toHaveCount(3);
  await expect(page.getByTestId("tag-rename")).toHaveCount(3);
  await expect(page.getByTestId("tag-row-name")).toHaveText(["s1", "s2", "s10"]);
});

test("T188 size: the chip cloud is ≤40% of the Manage rows' height (light + dark)", async ({ browser }) => {
  for (const colorScheme of ["light", "dark"] as const) {
    const ctx = await browser.newContext({ viewport: { width: 1280, height: 800 }, colorScheme });
    const page = await ctx.newPage();
    const who = stamp();
    await register(page, `size${who}_${colorScheme}`);
    const band = await createBandAndOpen(page, `Band ${who}`);
    for (let i = 0; i < 20; i++) await seed(page, band.id, `song${i}`, [`kind-${String.fromCharCode(97 + i)}`]);
    await page.reload();
    await expect(page.getByTestId("tags-panel")).toBeVisible();
    await expect(page.getByTestId("tag-chip")).toHaveCount(20);

    const panel = page.getByTestId("tags-panel");
    const chipH = (await panel.boundingBox())!.height;
    await page.screenshot({ path: `/tmp/t188shots/site-tags-chip-${colorScheme}.png` });
    await openManage(page);
    await expect(page.getByTestId("tag-row")).toHaveCount(20);
    const manageH = (await panel.boundingBox())!.height;
    await page.screenshot({ path: `/tmp/t188shots/site-tags-manage-${colorScheme}.png` });

    const ratio = chipH / manageH;
    console.log(`[T188 size ${colorScheme}] chip=${chipH.toFixed(0)}px manage=${manageH.toFixed(0)}px ratio=${(ratio * 100).toFixed(1)}%`);
    expect(ratio, `chip mode should be ≤40% of Manage (${colorScheme})`).toBeLessThanOrEqual(0.4);
    await ctx.close();
  }
});

/**
 * T181 — song search on the band page: the typed word→chip sequence end to end (⟨D1⟩/⟨D2⟩), and the ⟨D6⟩ row
 * tag pills (click toggles the filter, never navigates, appears on a small band). The matching rules
 * themselves are unit-pinned in song-search.test.ts; this covers the join.
 */
import { test, expect, type Page } from "@playwright/test";
import { createBandAndOpen, register, stamp } from "./setup-helpers";

/** Create a song via the API with tags, returning nothing — fast seeding without UI churn. */
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

test("the typed sequence: 'ro' then 'op' offers a tag, accepting makes a chip and filters (⟨D1⟩/⟨D2⟩)", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `search${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // >12 songs so the filter box shows. Three contain "ro"; one of those carries the tag "opener".
  await seed(page, band.id, "Rocket", ["opener"]);
  await seed(page, band.id, "Roar", ["ballad"]);
  await seed(page, band.id, "Rose", ["encore"]);
  for (let i = 0; i < 12; i++) await seed(page, band.id, `Filler ${i}`, []);
  await page.reload();

  const box = page.getByTestId("songs-filter");
  await expect(box).toBeVisible();

  // 'ro' → the three ro-songs (among others 'ro' can't match — fillers have no r/o run? they do not contain "ro")
  await box.click();
  await box.fill("ro");
  await expect(page.getByTestId("song-link")).toHaveCount(3);

  // ' op' → a suggestion for 'opener' appears (for the word being typed), and the list narrows to Rocket.
  await box.fill("ro op");
  // caret is at the end (the 'op' word)
  const sugg = page.getByTestId("search-suggestion").filter({ hasText: "opener" });
  await expect(sugg).toBeVisible();
  await expect(page.getByTestId("song-link")).toHaveText([/Rocket/]); // only Rocket has both 'ro' and 'op'

  // accept the suggestion → a chip 'opener', the box keeps 'ro', and only opener-songs-with-ro remain.
  await sugg.click();
  await expect(page.getByTestId("search-chip")).toHaveText(["opener×"]);
  await expect(box).toHaveValue("ro");
  await expect(page.getByTestId("song-link")).toHaveText([/Rocket/]);

  // Backspace in the (now 'ro') box doesn't remove the chip; clear the box then Backspace does.
  await box.fill("");
  await box.press("Backspace");
  await expect(page.getByTestId("search-chip")).toHaveCount(0);
});

test("⟨D6⟩ a row tag pill toggles the filter, never navigates, and brings up the box on a small band", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `rows${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // A small band (≤12): the filter box is hidden until a chip exists.
  await seed(page, band.id, "Alpha", ["encore", "opener"]);
  await seed(page, band.id, "Beta", ["encore"]);
  await seed(page, band.id, "Gamma", ["ballad"]);
  await page.reload();

  await expect(page.getByTestId("songs-filter")).toHaveCount(0); // box hidden on a small band
  const alphaRow = page.locator('[data-testid="songs-list"] li', { hasText: "Alpha" });

  // ⟨D6⟩ the pills are NOT inside the row's <a> (a button in an anchor is invalid + would navigate).
  const pillsInsideLink = await alphaRow.locator('a [data-testid="row-tag"]').count();
  expect(pillsInsideLink).toBe(0);

  // Click a pill → it becomes a chip, the box appears, the list filters, and the URL does not change.
  const urlBefore = page.url();
  await alphaRow.getByTestId("row-tag").filter({ hasText: "encore" }).click();
  await expect(page).toHaveURL(urlBefore); // did NOT navigate into the song
  await expect(page.getByTestId("songs-filter")).toBeVisible(); // box now shows (chip active)
  await expect(page.getByTestId("search-chip")).toHaveText(["encore×"]);
  await expect(page.getByTestId("song-link")).toHaveText([/Alpha/, /Beta/]); // only encore songs

  // the active tag's pill carries the active style; a non-active one does not
  await expect(alphaRow.getByTestId("row-tag").filter({ hasText: "encore" })).toHaveClass(/active/);
  await expect(alphaRow.getByTestId("row-tag").filter({ hasText: "opener" })).not.toHaveClass(/active/);

  // click the same tag again (via its chip ×) → removed, box gone again
  await page.getByTestId("search-chip-remove").click();
  await expect(page.getByTestId("search-chip")).toHaveCount(0);
  await expect(page.getByTestId("songs-filter")).toHaveCount(0);
});

test("⟨1⟩ removing the last chip while text filters keeps the box, and clearing it restores the list", async ({
  page,
}) => {
  const who = stamp();
  await register(page, `stuck${who}`);
  const band = await createBandAndOpen(page, `Band ${who}`);
  // Small band (≤12): the box only appears once something filters.
  await seed(page, band.id, "Alpha", ["cover"]);
  await seed(page, band.id, "Beta", ["cover"]);
  await seed(page, band.id, "Gamma", ["ballad"]);
  await page.reload();
  await expect(page.getByTestId("songs-filter")).toHaveCount(0);

  // pill → chip + box; two cover songs
  const alpha = page.locator('[data-testid="songs-list"] li', { hasText: "Alpha" });
  await alpha.getByTestId("row-tag").filter({ hasText: "cover" }).click();
  const box = page.getByTestId("songs-filter");
  await expect(box).toBeVisible();
  await expect(page.getByTestId("song-link")).toHaveCount(2);

  // type text that matches nothing → empty list, box still there
  await box.fill("zz");
  await expect(page.getByTestId("song-link")).toHaveCount(0);

  // remove the chip while the text remains — the box MUST stay (it was the stuck state)
  await page.getByTestId("search-chip-remove").click();
  await expect(page.getByTestId("search-chip")).toHaveCount(0);
  await expect(box).toBeVisible(); // not hidden over an empty, unclearable list
  await expect(box).toHaveValue("zz");
  await expect(page.getByTestId("song-link")).toHaveCount(0);

  // clearing the text brings every song back
  await box.fill("");
  await expect(page.getByTestId("song-link")).toHaveCount(3);
});

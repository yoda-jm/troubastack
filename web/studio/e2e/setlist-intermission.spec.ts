/**
 * T153 slice 4b — an intermission in the Studio setlist editor.
 *
 * The band must be able to put a break in its own running order without anyone touching Go. This drives
 * the whole surface end to end: add it, see it unnumbered between two songs, rename it, and confirm the
 * rename survives a reload (i.e. it reached the server, not just React state).
 *
 * The assertion that matters is the NUMBERING: a break must take no number AND must not shift the song
 * after it. A naive implementation that numbered every main-order row would make the second song read 3,
 * and VLL's setlist landmarks would all move by one — that is the failure this exists to catch.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen, createSetlist } from "./setup-helpers";

test("setlist: add an intermission between two songs — unnumbered, renameable, and it does not shift the next song", async ({
  page,
}) => {
  await register(page, `int_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `IntBand ${stamp()}`);
  const first = `Opener ${stamp()}`;
  const second = `Closer ${stamp()}`;
  await createSongAndOpen(page, first);
  await page.goto(`/bands/${bandId}`);
  await createSongAndOpen(page, second);

  await page.goto(`/bands/${bandId}/setlists`);
  const gig = `Gig ${stamp()}`;
  await createSetlist(page, gig);
  await page.getByTestId("setlist-link").filter({ hasText: gig }).click();

  // Song, break, song — the break lands between them.
  await page.getByTestId("add-item-song").selectOption({ label: first });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(1);

  await page.getByTestId("add-intermission").click();
  await expect(page.getByTestId("item-row")).toHaveCount(2);

  await page.getByTestId("add-item-song").selectOption({ label: second });
  await page.getByTestId("add-item").click();
  await expect(page.getByTestId("item-row")).toHaveCount(3);

  // THE assertion: the break carries no number, and the song after it is still 2 — not 3.
  const rows = page.getByTestId("item-row");
  await expect(rows.nth(0)).toContainText("1.");
  await expect(rows.nth(2)).toContainText("2.");
  await expect(rows.nth(1)).not.toContainText("2.");

  // A break is not a song: it must not render a link into /songs/<empty>, which would be a dead route
  // wearing a title. It renders a centered divider (its name between two rules) instead.
  const breakLabel = rows.nth(1).getByTestId("item-intermission-label");
  await expect(breakLabel).toBeVisible();
  await expect(rows.nth(1).getByTestId("item-title-link")).toHaveCount(0);

  // Its message is changed behind the pencil, like a song's editor (VLL) — not an always-live field.
  await rows.nth(1).getByTestId("item-edit").click();
  const breakInput = rows.nth(1).getByTestId("item-intermission-input");
  // A break still has no musical editor: the pencil opens only its label, never key / tempo.
  await expect(rows.nth(1).getByTestId("item-key")).toHaveCount(0);
  await breakInput.fill("Entracte");
  await breakInput.press("Enter");

  // The rename round-trips (relabel awaits the server + reload); the divider shows the new name.
  await expect(rows.nth(1).getByTestId("item-intermission-label")).toContainText("Entracte");
  // Prove it PERSISTED (reached the server, not just React state) with a fresh reload.
  await page.reload();
  await expect(page.getByTestId("item-row").nth(1).getByTestId("item-intermission-label")).toContainText(
    "Entracte",
  );

  // And the numbering is unchanged by the rename.
  await expect(page.getByTestId("item-row").nth(2)).toContainText("2.");
});

/**
 * T201 (Studio tag): the "Edited since last bake" tag on the setlist list and detail, driven by
 * editedSinceBake on GET /concerts. The signal itself is Go-tested; here we mock /concerts (the e2e core has
 * no bake toolchain) and assert the tag appears on both surfaces when a concert reads edited, and not before.
 */
import { test, expect } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSetlist } from "./setup-helpers";

test("edited tag shows on the setlist list and detail when a concert is edited since its bake", async ({ page }) => {
  await register(page, `edit_${stamp()}`);
  const { id: bandId } = await createBandAndOpen(page, `EditBand ${stamp()}`);
  await page.goto(`/bands/${bandId}/setlists`);
  const name = `EditGig ${stamp()}`;
  await createSetlist(page, name);

  const link = page.getByTestId("setlist-link").filter({ hasText: name });
  const href = await link.getAttribute("href");
  const slId = href!.split("/").pop()!;

  // No concert yet ⇒ no tag.
  await expect(page.getByTestId("setlist-edited-tag")).toHaveCount(0);

  // Mock /concerts: this setlist's concert reads edited since its last bake.
  await page.route("**/concerts", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        concerts: [
          {
            concertId: slId,
            name,
            currentRev: "1",
            updatedAt: "1",
            songs: [],
            downloadUrl: `/api/bands/${bandId}/concerts/${slId}/bundle`,
            editedSinceBake: true,
            changes: { songs: [{ songId: "s1", title: "Opener", what: "content" }], setlist: false },
          },
        ],
      }),
    }),
  );
  await page.reload();

  // The list row carries the tag.
  await expect(page.getByTestId("setlist-edited-tag")).toBeVisible();

  // The detail page carries it too (T206: in the shared concert header now, fed by ConcertLayout's
  // /concerts fetch — no longer only next to the bake button).
  await page.getByTestId("setlist-link").filter({ hasText: name }).click();
  await expect(page.getByTestId("bake-setlist")).toBeVisible();
  await expect(page.getByTestId("setlist-edited-tag")).toBeVisible();
});

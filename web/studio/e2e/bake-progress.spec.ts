/**
 * T99 — the bake dialog shows T96's progress ("song 3 of 11" → "Finishing…"), degrades to
 * today's "Baking…" when progress is unavailable, names the failing song, and never leaks a
 * polling timer. Network-free: both the bake POST and the progress GET are intercepted with
 * page.route (the lyrics-search precedent), so no real (poppler) bake is stood up.
 */
import { test, expect, type Page } from "@playwright/test";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

// Build a band + one song + a one-item setlist, and land on the setlist page ready to bake.
async function setupBakeable(page: Page): Promise<void> {
  await page.goto("/register");
  await page.getByTestId("username").fill(`bp_${stamp()}`);
  await page.getByTestId("displayName").fill("D");
  await page.getByTestId("password").fill("secret123");
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);

  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(`BPBand ${stamp()}`);
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
}

test("shows 'song N of M', then 'Finishing…' for the done==total tail (T99 §3)", async ({ page }) => {
  await setupBakeable(page);

  // T103: the POST is a KICK — it returns 202 promptly; the poll is the source of truth for the outcome.
  await page.route("**/setlists/*/bake", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: "x" }) }),
  );

  // Progress: a per-song update, then the song-less done==total finishing update, then the TERMINAL
  // succeeded record (which is what now closes the dialog — no held POST to release).
  let calls = 0;
  await page.route("**/bakes/*/progress", async (route) => {
    calls++;
    const seq = [
      { state: "running", done: 1, total: 2, song: "Song A" },
      { state: "running", done: 2, total: 2 }, // finishing: done==total, no song
      { state: "running", done: 2, total: 2 }, // hold finishing so it's observable
      { state: "succeeded", done: 2, total: 2 }, // terminal ⇒ dialog closes
    ];
    const p = seq[Math.min(calls - 1, seq.length - 1)];
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(p) });
  });

  await page.getByTestId("bake-setlist").click();
  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  await page.getByTestId("bake-dialog-confirm").click();

  // Row 1: running + song → "song N of M".
  await expect(page.getByTestId("bake-progress")).toHaveText("Baking — song 1 of 2: Song A");
  // Row 2 (the one to get right): done==total with no song → "Finishing…", NOT "2 of 2".
  await expect(page.getByTestId("bake-progress")).toHaveText("Finishing…");
  // The terminal succeeded record closes the dialog.
  await expect(page.getByTestId("bake-dialog")).toBeHidden();
});

test("degrades to 'Baking…' when progress 404s, and the bake still completes (T99 §4)", async ({
  page,
}) => {
  await setupBakeable(page);

  await page.route("**/setlists/*/bake", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: "x" }) }),
  );
  // Progress momentarily unavailable (the record not yet claimed / a transient blip), then it appears
  // as the terminal succeeded. During the 404 window the dialog degrades to "Baking…", never an error.
  let calls = 0;
  await page.route("**/bakes/*/progress", async (route) => {
    calls++;
    if (calls <= 2) {
      await route.fulfill({ status: 404, contentType: "application/json", body: JSON.stringify({ error: "no such bake" }) });
      return;
    }
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify({ state: "succeeded", done: 1, total: 1 }) });
  });

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click();

  // No progress → today's "Baking…", never an error.
  await expect(page.getByTestId("bake-progress")).toHaveText("Baking…");
  await expect(page.getByTestId("bake-error")).toHaveCount(0);

  // Once the terminal record appears, the bake completes and the dialog closes — the non-regression.
  await expect(page.getByTestId("bake-dialog")).toBeHidden();
});

test("a failed bake names the song the server reported (T99 §6)", async ({ page }) => {
  await setupBakeable(page);

  // T103: the POST kicks fine (202); the FAILURE arrives on the terminal progress record, naming the
  // song (the error text is already the T102-humanised message).
  await page.route("**/setlists/*/bake", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: "x" }) }),
  );
  await page.route("**/bakes/*/progress", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ state: "failed", done: 1, total: 2, song: "The Open Road", error: "the renderer is unavailable" }),
    }),
  );

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click();

  await expect(page.getByTestId("bake-error")).toContainText("The Open Road");
  // Dialog stays open on failure (retryable), Bake re-enabled.
  await expect(page.getByTestId("bake-dialog")).toBeVisible();
  await expect(page.getByTestId("bake-dialog-confirm")).toBeEnabled();
});

test("stops polling when the dialog closes — no leaked timer (T99 §5)", async ({ page }) => {
  await setupBakeable(page);

  await page.route("**/setlists/*/bake", (route) =>
    route.fulfill({ status: 202, contentType: "application/json", body: JSON.stringify({ bakeId: "x" }) }),
  );
  let calls = 0;
  await page.route("**/bakes/*/progress", async (route) => {
    calls++;
    // A couple of running ticks, then terminal succeeded — which closes the dialog and must stop the poll.
    const p = calls <= 2 ? { state: "running", done: 1, total: 2, song: "Song A" } : { state: "succeeded", done: 2, total: 2 };
    await route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(p) });
  });

  await page.getByTestId("bake-setlist").click();
  await page.getByTestId("bake-dialog-confirm").click();
  await expect(page.getByTestId("bake-progress")).toHaveText("Baking — song 1 of 2: Song A");

  // The terminal record closes the dialog.
  await expect(page.getByTestId("bake-dialog")).toBeHidden();

  // After the dialog is gone, polling must stop climbing (the interval was cleared — no leaked timer).
  const settled = calls;
  await page.waitForTimeout(2500); // > 2 poll intervals
  expect(calls).toBe(settled);
});

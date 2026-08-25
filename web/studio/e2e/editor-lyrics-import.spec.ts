/**
 * T37 guard: "New chart from lyrics" in the editor's Details panel (T36's Files home).
 * Paste-first with a best-effort "Fetch from URL" accelerator. VLL made azlyrics a
 * must-have; the fetch is honest (server-side, SSRF-guarded, no evasion) so it OFTEN
 * comes back blocked — the paste fallback is what makes it reliable, so the guard proves
 * the fallback UX, not a live fetch (network + Cloudflare are non-deterministic).
 *
 * Fetches are STUBBED via page.route so the outcomes are deterministic in CI (the SSRF
 * guard would block a local fixture server anyway). Red-first: the button doesn't exist
 * pre-fix.
 */
import { test, expect, type Page } from "@playwright/test";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

async function openLyricsDialog(page: Page) {
  await page.getByTestId("my-files-edit").click(); // open the Details panel (T36)
  const panel = page.getByTestId("details-panel");
  await panel.getByTestId("new-lyrics-chart").click();
  await expect(panel.getByTestId("lyrics-dialog")).toBeVisible();
  return panel;
}

test("paste lyrics → normalized chart opens in the editor and saves to the pool (T37)", async ({
  page,
}) => {
  await register(page, `lp_${stamp()}`);
  await createBandAndOpen(page, `LPBand ${stamp()}`);
  await createSongAndOpen(page, `The Open Road`);
  const panel = await openLyricsDialog(page);

  // Messy paste: CRLF, a 4-blank run, and a site-cruft trailer that must be dropped.
  await panel
    .getByTestId("lyrics-text")
    .fill("Today is gonna be the day\r\n\r\n\r\n\r\nThat they're gonna throw it back to you\r\nSubmit Corrections");
  await panel.getByTestId("lyrics-create").click();

  // The T19 chart editor opens pre-filled with the NORMALIZED source (title heading +
  // collapsed blanks; the cruft line gone).
  const source = panel.getByTestId("chart-source");
  await expect(source).toBeVisible();
  const val = await source.inputValue();
  expect(val).toContain("# The Open Road");
  expect(val).toContain("Today is gonna be the day");
  expect(val).toContain("That they're gonna throw it back to you");
  expect(val).not.toContain("Submit Corrections"); // cruft dropped
  expect(val).not.toMatch(/\n{3,}/); // blank runs collapsed

  // Save → it enters the pool as a generated chart file.
  await panel.getByTestId("chart-save").click();
  await expect(panel.getByTestId("file-row")).toHaveCount(1);
  await expect(panel.getByTestId("file-chart-badge")).toBeVisible();
});

test("fetch that is blocked shows the honest fallback and keeps the paste box focused (T37)", async ({
  page,
}) => {
  await register(page, `lb_${stamp()}`);
  await createBandAndOpen(page, `LBBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  // Stub the endpoint: an honest fetch of azlyrics comes back blocked (Cloudflare).
  await page.route("**/api/bands/*/lyrics-import", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "blocked", reason: "bot protection" }),
    }),
  );
  const panel = await openLyricsDialog(page);
  await panel.getByTestId("lyrics-url").fill("https://www.azlyrics.com/lyrics/oasis/wonderwall.html");
  await panel.getByTestId("lyrics-fetch").click();

  await expect(panel.getByTestId("lyrics-fetch-msg")).toContainText(/blocked|paste/i);
  await expect(panel.getByTestId("lyrics-text")).toBeFocused(); // not dead-ended
  // The user can still paste + create.
  await panel.getByTestId("lyrics-text").fill("pasted after a block");
  await expect(panel.getByTestId("lyrics-create")).toBeEnabled();
});

test("fetch that succeeds fills the paste box for review (T37)", async ({ page }) => {
  await register(page, `lo_${stamp()}`);
  await createBandAndOpen(page, `LOBand ${stamp()}`);
  await createSongAndOpen(page, `Song ${stamp()}`);
  await page.route("**/api/bands/*/lyrics-import", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ status: "ok", text: "fetched verse one\n\nfetched verse two" }),
    }),
  );
  const panel = await openLyricsDialog(page);
  await panel.getByTestId("lyrics-url").fill("https://example.com/song");
  await panel.getByTestId("lyrics-fetch").click();
  await expect(panel.getByTestId("lyrics-text")).toHaveValue(/fetched verse one[\s\S]*fetched verse two/);
});

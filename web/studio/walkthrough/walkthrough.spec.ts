/**
 * DEMO-VID Part B — web walkthrough (first cut). Tours the SEEDED demo through the storyboard's
 * web scenes, paced so each on-screen beat lands under its narration segment (docs/video/
 * script.md). Records one 1920×1080 video for the whole tour; Part D cuts it per scene and muxes
 * the TTS. Navigation is by visible text / testids so it survives fresh-seed UUIDs.
 *
 * This is the deterministic "show the built demo" spine (scenes 6–14). The build-from-scratch
 * scenes (1–5: create band / invite / write a chart) are a follow-up segment.
 */
import { test, expect, Page } from "@playwright/test";

// Pace helper: hold the current frame for a scene's narration length (audio-first sets finals).
const beat = (p: Page, seconds: number) => p.waitForTimeout(seconds * 1000);

async function login(p: Page, user: string) {
  await p.goto("/login", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(user);
  await p.getByTestId("password").fill("demo");
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 15_000 });
  await p.waitForTimeout(1000);
}

test("web walkthrough — the built demo (scenes 6–14)", async ({ page }) => {
  test.setTimeout(180_000);

  // ── S1/S6 intro: Marie's bands ────────────────────────────────────────────
  await login(page, "marie");
  await beat(page, 3);
  await page.getByText("The Troubadours", { exact: false }).first().click();
  await page.waitForLoadState("networkidle");
  await beat(page, 3); // the band overview: members + roles + songs

  // ── S6–S9: the flagship annotation showcase — The Open Road ────────────────
  await page.getByTestId("song-link").filter({ hasText: "The Open Road" }).first().click();
  await page.waitForLoadState("networkidle");
  await beat(page, 6); // the annotated lead sheet: capo highlight, chorus, conductor cue
  // switch to the Guitar file tab (the chord chart with per-file annotations)
  const guitar = page.getByText("Guitar", { exact: false }).first();
  if (await guitar.count()) {
    await guitar.click();
    await beat(page, 5);
  }

  // ── S11: the setlist ───────────────────────────────────────────────────────
  // Back out of the editor via its "Back to band" link (the editor has no band-name text).
  const back = page.getByRole("link", { name: /back to band/i });
  if (await back.count()) await back.first().click();
  else await page.goBack();
  await page.waitForLoadState("networkidle");
  await beat(page, 1);
  const setlistsTab = page.getByRole("link", { name: /^setlists$/i }).or(page.getByText("Setlists", { exact: true }));
  if (await setlistsTab.count()) {
    await setlistsTab.first().click();
    await page.waitForLoadState("networkidle");
    await beat(page, 2);
    const sl = page.getByText("Sat @ The Anchor", { exact: false }).first();
    if (await sl.count()) {
      await sl.click();
      await page.waitForLoadState("networkidle");
      await beat(page, 6); // running order: 4 songs, per-member cues, overrides
    }
  }

  // ── S13–S14: it scales to an orchestra — Eine kleine, real parts ───────────
  await login(page, "maestro");
  await beat(page, 2);
  await page.getByText("City Chamber Orchestra", { exact: false }).first().click();
  await page.waitForLoadState("networkidle");
  await beat(page, 2);
  await page.getByTestId("song-link").filter({ hasText: "Eine kleine" }).first().click();
  await page.waitForLoadState("networkidle");
  await beat(page, 5); // Violin I part with the conductor cue + player bowing
  // the full score (conductor's interpretation layer)
  const score = page.getByText("Full score", { exact: false }).first();
  if (await score.count()) {
    await score.click();
    await beat(page, 6);
  }

  // final hold
  await beat(page, 2);
  expect(true).toBe(true);
});

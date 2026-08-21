/**
 * T85 — the visual beat.
 *
 * Two layers of coverage:
 *  - The `beatPhase` CONTRACT, run against the shared vector file that A34 (Kotlin) also
 *    runs. These bind the timeline both renderers agree on: break the TS function and the
 *    vectors go red (proved by hand: flipping `% 4 === 0` to `=== 1` fails 8 vectors).
 *  - The studio UI: tap the beat control → the frame pulses, and a count-in stops itself.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { beatPhase, intervalMs, COUNT_IN_BEATS } from "../src/beatPhase";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;

interface Vector {
  elapsedMs: number;
  intervalMs: number;
  beats: number;
  beatIndex: number;
  lit: boolean;
  emphasis: boolean;
}
const vectors: { cases: Vector[] } = JSON.parse(
  readFileSync(
    fileURLToPath(new URL("../../../docs/contracts/beat-phase.vectors.json", import.meta.url)),
    "utf8",
  ),
);

// ===========================================================================
// Contract — the shared beat-phase vectors (no browser).
// ===========================================================================
test("beat contract: the studio beatPhase matches every shared vector (T85)", () => {
  expect(vectors.cases.length).toBeGreaterThanOrEqual(20);
  for (const c of vectors.cases) {
    const p = beatPhase(c.elapsedMs, c.intervalMs, c.beats);
    expect(
      { beatIndex: p.beatIndex, lit: p.lit, emphasis: p.emphasis },
      `elapsed=${c.elapsedMs} interval=${c.intervalMs} beats=${c.beats}`,
    ).toEqual({ beatIndex: c.beatIndex, lit: c.lit, emphasis: c.emphasis });
  }
});

test("beat contract: a 120bpm count-in is 8 transient beats, downbeats at 0 and 4 (T85)", () => {
  const interval = intervalMs(120); // 500 ms
  const beats = COUNT_IN_BEATS; // 8
  // Count lit→unlit transitions across the whole count-in: one per beat, so exactly 8.
  let transitions = 0;
  let prevLit = false;
  for (let e = 0; e <= beats * interval + 5; e++) {
    const lit = beatPhase(e, interval, beats).lit;
    if (lit && !prevLit) transitions++;
    prevLit = lit;
  }
  expect(transitions).toBe(8);

  // Per-beat lit window is a transient (≤ 35% of the interval), and downbeats land on 0 and 4.
  let maxLitMs = 0;
  const downbeats: number[] = [];
  for (let b = 0; b < beats; b++) {
    let litMs = 0;
    for (let e = b * interval; e < (b + 1) * interval; e++) {
      if (beatPhase(e, interval, beats).lit) litMs++;
    }
    maxLitMs = Math.max(maxLitMs, litMs);
    if (beatPhase(b * interval, interval, beats).emphasis) downbeats.push(b);
  }
  expect(maxLitMs).toBeLessThanOrEqual(interval * 0.35);
  expect(downbeats).toEqual([0, 4]);
});

test("beat contract: no drift — beat 200 lands at 200×interval, not accumulated (T85)", () => {
  // A monotonic-clock reader has no accumulation error; the app's original bug (chained
  // delays + truncated interval) would land beat 200 tens of ms early. Stub the clock in
  // 1 ms steps around the target and find where beat 200 begins.
  for (const bpm of [120, 90]) {
    const interval = intervalMs(bpm);
    const target = 200 * interval;
    let onset = -1;
    for (let e = Math.floor(target) - 10; e <= Math.ceil(target) + 10; e++) {
      if (beatPhase(e, interval, 1e9).beatIndex >= 200) {
        onset = e;
        break;
      }
    }
    expect(Math.abs(onset - target), `bpm=${bpm}`).toBeLessThanOrEqual(5);
  }
});

// ===========================================================================
// Studio UI — tap the beat, the frame pulses, a count-in stops itself.
// ===========================================================================
async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

async function createBandAndOpen(page: Page, bandName: string) {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
}

async function createSongWithTempo(page: Page, title: string, bpm: number) {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  // Set the tempo in Details, save, then close the panel so the chrome is clear.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("meta-tempo").fill(String(bpm));
  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();
  await page.getByTestId("my-files-edit").click();
}

test("editor: no tempo → no beat control (T85)", async ({ page }) => {
  await register(page, `beatnil_${stamp()}`);
  await createBandAndOpen(page, `NoTempo ${stamp()}`);
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`NT ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await expect(page.getByTestId("viewer-chrome")).toBeVisible();
  await expect(page.getByTestId("beat-controls")).toHaveCount(0);
});

test("editor: tap the beat → the frame pulses, and the count-in stops itself (T85)", async ({
  page,
}) => {
  await register(page, `beat_${stamp()}`);
  await createBandAndOpen(page, `BeatBand ${stamp()}`);
  // 240 bpm → 250 ms/beat → an 8-beat count-in finishes in ~2.2 s, so the test is quick.
  await createSongWithTempo(page, `BeatSong ${stamp()}`, 240);

  const toggle = page.getByTestId("beat-toggle");
  const frame = page.getByTestId("beat-frame");
  await expect(toggle).toBeVisible();
  await expect(toggle).toHaveAttribute("aria-pressed", "false");

  // Tap → running, and the rAF loop marks the frame with the live beat index.
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await expect(frame).toHaveAttribute("data-beat", /\d+/);

  // A count-in self-stops: within a few seconds the control releases and the frame clears.
  await expect(toggle).toHaveAttribute("aria-pressed", "false", { timeout: 6000 });
  await expect(frame).not.toHaveAttribute("data-beat", /\d+/);
});

test("editor: the ∞ toggle switches the beat to continuous (T85)", async ({ page }) => {
  await register(page, `beatloop_${stamp()}`);
  await createBandAndOpen(page, `LoopBand ${stamp()}`);
  await createSongWithTempo(page, `LoopSong ${stamp()}`, 200);

  const loop = page.getByTestId("beat-loop");
  const toggle = page.getByTestId("beat-toggle");
  await expect(loop).toHaveAttribute("aria-pressed", "false");
  await loop.click();
  await expect(loop).toHaveAttribute("aria-pressed", "true");

  // Continuous: still running well past one count-in's worth of time.
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await page.waitForTimeout(2500);
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await toggle.click(); // stop
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
});

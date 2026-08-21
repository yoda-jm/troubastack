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
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

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

// ===========================================================================
// Frame placement — hug the page on a wide screen, fall back to the viewport.
// ===========================================================================
async function uploadPdf(page: Page) {
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
}

async function firstFileId(page: Page, bandId: string, songId: string): Promise<string> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      const j = (await r.json()) as { files: { id: string }[] };
      return j.files[0].id;
    },
    [bandId, songId] as const,
  );
}

async function myUserId(page: Page): Promise<string> {
  return page.evaluate(async () => {
    const r = await fetch("/api/me", { credentials: "include" });
    return ((await r.json()) as { user: { id: string } }).user.id;
  });
}

async function importLayer(page: Page, bandId: string, songId: string, fileId: string, me: string) {
  await page.evaluate(
    async ([b, s, body]) => {
      await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
    },
    [
      bandId,
      songId,
      {
        layers: [
          { id: "L-me", fileId, name: "Mine", ownerId: me, zone: "personal", order: 0, access: "rw" },
        ],
        objects: [],
      },
    ] as const,
  );
}

test("editor: the beat frame hugs the page on a wide screen, never past the viewport (T85)", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1600, height: 820 });
  await register(page, `beathug_${stamp()}`);
  await createBandAndOpen(page, `HugBand ${stamp()}`);
  // Create a song with a tempo AND a rendered page (upload + import a layer + reload).
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`HugSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  const bandId = page.url().match(/\/bands\/([^/]+)/)![1];
  const sid = page.url().match(/\/songs\/([^/]+)/)![1];
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("meta-tempo").fill("120");
  await page.getByTestId("meta-save").click();
  await expect(page.getByTestId("meta-notice")).toBeVisible();
  await page.getByTestId("my-files-edit").click();
  await uploadPdf(page);
  const fileId = await firstFileId(page, bandId, sid);
  const me = await myUserId(page);
  await importLayer(page, bandId, sid, fileId, me);
  await page.reload();
  await expect(page.getByTestId("pdf-page").first()).toBeVisible({ timeout: 12000 });

  // Fit-page on a wide screen → the sheet is a narrow centred column.
  await page.getByTestId("zoom-mode").selectOption("fit-page");
  await page.waitForTimeout(400);

  const pageBox = (await page.getByTestId("pdf-page").first().boundingBox())!;
  const scrollBox = (await page.getByTestId("viewer-scroll").boundingBox())!;
  expect(pageBox.width).toBeLessThan(scrollBox.width * 0.85); // genuinely narrower than the viewport

  await page.getByTestId("beat-toggle").click();
  await expect(page.getByTestId("beat-frame")).toHaveAttribute("data-beat", /\d+/);
  const frame = (await page.getByTestId("beat-frame").boundingBox())!;

  // The rail hugs the page (~6px gap each side), not the far viewport edges.
  expect(Math.abs(frame.x - (pageBox.x - 6))).toBeLessThanOrEqual(4);
  expect(Math.abs(frame.width - (pageBox.width + 12))).toBeLessThanOrEqual(6);
  expect(frame.width).toBeLessThan(scrollBox.width * 0.9); // NOT viewport-wide
  // …and never spills past the viewport (the clamp side of the intersection).
  expect(frame.x).toBeGreaterThanOrEqual(scrollBox.x - 1);
  expect(frame.x + frame.width).toBeLessThanOrEqual(scrollBox.x + scrollBox.width + 1);
});

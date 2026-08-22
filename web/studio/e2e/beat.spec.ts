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
import {
  beatPhase,
  intervalMs,
  countInUnits,
  meterGroups,
  DEFAULT_GROUPS,
} from "../src/beatPhase";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

interface Vector {
  elapsedMs?: number;
  intervalMs?: number;
  beats?: number;
  groups?: number[];
  beatIndex?: number;
  lit?: boolean;
  tier?: 0 | 1 | 2;
  emphasis?: boolean;
  _?: string; // a descriptive comment row (skipped)
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
test("beat contract: the studio beatPhase matches every shared vector (T85/T86)", () => {
  const real = vectors.cases.filter((c) => c.elapsedMs !== undefined);
  expect(real.length).toBeGreaterThanOrEqual(40);
  for (const c of real) {
    const p = beatPhase(c.elapsedMs!, c.intervalMs!, c.beats!, c.groups);
    const got: Record<string, unknown> = { beatIndex: p.beatIndex, lit: p.lit, emphasis: p.emphasis };
    const want: Record<string, unknown> = { beatIndex: c.beatIndex, lit: c.lit, emphasis: c.emphasis };
    if (c.tier !== undefined) {
      got.tier = p.tier;
      want.tier = c.tier;
    }
    expect(got, `elapsed=${c.elapsedMs} interval=${c.intervalMs} groups=${c.groups ?? "4/4"}`).toEqual(
      want,
    );
  }
  // At least one 4/4 case carries no tier (the untouched-backward-compat proof) and one metre case does.
  expect(real.some((c) => c.tier === undefined)).toBe(true);
  expect(real.some((c) => c.groups && c.tier !== undefined)).toBe(true);
});

test("beat contract: a 120bpm 4/4 count-in is 8 transient beats, downbeats at 0 and 4 (T85)", () => {
  const interval = intervalMs(120); // 500 ms
  const beats = countInUnits(DEFAULT_GROUPS); // 8
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

test("beat contract: the count-in is two bars in units, every metre (T86)", () => {
  expect(countInUnits([1, 1, 1, 1])).toBe(8); // 4/4
  expect(countInUnits([1, 1, 1])).toBe(6); // 3/4
  expect(countInUnits([3, 3])).toBe(12); // 6/8 — 12 units = 4 felt pulses
  expect(countInUnits([3, 4])).toBe(14); // 3+4/8
  expect(countInUnits(DEFAULT_GROUPS)).toBe(8); // unset = 4/4, the no-regression case
});

test("beat contract: 3/4 puts a downbeat on unit 3 — the case the old %4 rule got wrong (T86)", () => {
  const g = [1, 1, 1];
  const interval = intervalMs(120); // 500 ms/quarter
  const downbeats: number[] = [];
  for (let u = 0; u < 6; u++) {
    if (beatPhase(u * interval, interval, 6, g).emphasis) downbeats.push(u);
  }
  expect(downbeats).toEqual([0, 3]); // not [0, 4]
});

test("beat contract: tier-2 subdivisions mute below 130 ms/unit, and light above (T86)", () => {
  const g = [3, 3]; // 6/8 — unit 1 is a free subdivision (tier 2)
  // At the onset of unit 1 (a tier-2 unit): lit above the threshold, dark below it.
  expect(beatPhase(200, 200, 12, g)).toMatchObject({ tier: 2, lit: true }); // 200 ms/unit
  expect(beatPhase(100, 100, 12, g)).toMatchObject({ tier: 2, lit: false }); // 100 ms/unit — muted
  // The bar and felt pulses always light, even below the threshold.
  expect(beatPhase(0, 100, 12, g)).toMatchObject({ tier: 0, lit: true });
  expect(beatPhase(300, 100, 12, g)).toMatchObject({ tier: 1, lit: true });
});

test("beat contract: meterGroups mirrors the core parser (T86)", () => {
  const table: Record<string, number[]> = {
    "4/4": [1, 1, 1, 1],
    "3/4": [1, 1, 1],
    "2/2": [1, 1],
    "6/8": [3, 3],
    "9/8": [3, 3, 3],
    "12/8": [3, 3, 3, 3],
    "5/4": [1, 1, 1, 1, 1],
    "3/8": [1, 1, 1],
    "3+2/8": [3, 2],
    "3+4/8": [3, 4],
    "2+2+3/8": [2, 2, 3],
    " 6 / 8 ": [3, 3],
  };
  for (const [m, want] of Object.entries(table)) {
    expect(meterGroups(m), `meterGroups(${JSON.stringify(m)})`).toEqual(want);
  }
  // Malformed → the 4/4 default (never throws).
  for (const bad of ["", "x/y", "4/5", "0/4", "33/4", "3+0/8", "-3/4", "4/4/4", null, undefined]) {
    expect(meterGroups(bad), `meterGroups(${JSON.stringify(bad)})`).toEqual([1, 1, 1, 1]);
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

test("editor: the beat tempo label names the metre's unit, and meta-meter persists (T86)", async ({ page }) => {
  await register(page, `t86m_${stamp()}`);
  await createBandAndOpen(page, `MeterBand ${stamp()}`);
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(`MeterSong ${stamp()}`);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").first().click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);

  const setMeta = async (fields: { tempo?: string; meter?: string }) => {
    await page.getByTestId("my-files-edit").click();
    if (fields.tempo !== undefined) await page.getByTestId("meta-tempo").fill(fields.tempo);
    if (fields.meter !== undefined) await page.getByTestId("meta-meter").fill(fields.meter);
    await page.getByTestId("meta-save").click();
    await expect(page.getByTestId("meta-notice")).toBeVisible();
    await page.getByTestId("my-files-edit").click();
  };
  const label = page.getByTestId("beat-tempo-label");

  await setMeta({ tempo: "120", meter: "6/8" });
  await expect(label).toHaveText("♩.=120"); // compound → dotted quarter

  // persists across reload
  await page.reload();
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("meta-meter")).toHaveValue("6/8");
  await page.getByTestId("my-files-edit").click();
  await expect(label).toHaveText("♩.=120");

  await setMeta({ meter: "3+4/8" });
  await expect(label).toHaveText("♪=120"); // irregular additive → eighth

  await setMeta({ meter: "" });
  await expect(label).toHaveText("♩=120"); // unset = 4/4 → quarter
});

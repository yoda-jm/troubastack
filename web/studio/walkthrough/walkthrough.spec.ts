/**
 * DEMO-VID Part B — web walkthrough (full polish). Drives the SEEDED demo through the web
 * storyboard (scenes 1–14, docs/video/script.md), paced so each on-screen beat lands under its
 * narration, and records one 1920×1080 video. Part D cuts it per scene and muxes the TTS.
 *
 * Live interactions (create a song + chart, toggle layers, draw, transpose, bake) are wrapped in
 * `soft()` so a selector miss is logged but never aborts the recording — the tour always finishes.
 */
import { test, expect, Page, Locator } from "@playwright/test";

const beat = (p: Page, seconds: number) => p.waitForTimeout(seconds * 1000);

// Best-effort step: run it, but if anything throws, log + keep recording.
async function soft(label: string, fn: () => Promise<void>) {
  try {
    await fn();
  } catch (e) {
    // eslint-disable-next-line no-console
    console.log(`[walkthrough] soft-skip "${label}": ${(e as Error).message.split("\n")[0]}`);
  }
}

const has = async (l: Locator) => (await l.count()) > 0;

async function login(p: Page, user: string) {
  await p.goto("/login", { waitUntil: "networkidle" });
  await p.getByTestId("username").fill(user);
  await p.getByTestId("password").fill("demo");
  await p.getByTestId("submit").click();
  await p.waitForURL(/\/bands/, { timeout: 15_000 });
  await p.waitForTimeout(800);
}

async function openBand(p: Page, name: string) {
  await p.getByText(name, { exact: false }).first().click();
  await p.waitForLoadState("networkidle");
  await p.waitForTimeout(600);
}

// Deterministic reset: always return to /bands and re-open the band, so a prior soft-skip
// can never leave a later scene stranded on the wrong page.
async function gotoBand(p: Page, name: string) {
  await p.goto("/bands", { waitUntil: "networkidle" });
  await p.waitForTimeout(400);
  await openBand(p, name);
}

async function openSong(p: Page, band: string, title: string) {
  await gotoBand(p, band);
  await p.getByTestId("song-link").filter({ hasText: title }).first().click();
  await p.waitForLoadState("networkidle");
  await p.waitForTimeout(600);
}

test("web walkthrough (scenes 1–14)", async ({ page }) => {
  test.setTimeout(240_000);

  await login(page, "marie");

  // ── S1–S3: the band, its members and roles ────────────────────────────────
  await openBand(page, "The Troubadours");
  await beat(page, 8); // overview: 3 members, roles (admin / conductor / member), songs

  // ── S4: create a song + write a chart that renders live ────────────────────
  await soft("create song + live chart", async () => {
    await page.getByTestId("new-song-btn").click();
    await page.getByTestId("song-title").fill("Sound Check");
    await page.getByTestId("create-song").click();
    await page.waitForLoadState("networkidle");
    await beat(page, 2);
    const chart = page.getByTestId("new-lyrics-chart").or(page.getByTestId("new-text-chart"));
    if (await has(chart)) {
      await chart.first().click();
      const ta = page.locator("textarea").first();
      await ta.click();
      await ta.type("G            D\nHere is a chord chart,\nEm           C\ntyped in plain text — rendered live.\n", { delay: 25 });
      await beat(page, 2);
      const create = page.getByTestId("lyrics-create").or(page.getByTestId("meta-save"));
      if (await has(create)) await create.first().click();
      await page.waitForLoadState("networkidle");
      await beat(page, 5); // the rendered sheet
    }
  });

  // ── S5: multi-file pool — House of the Rising Sun ──────────────────────────
  await soft("multi-file pool", async () => {
    await openSong(page, "The Troubadours", "House of the Rising Sun");
    await beat(page, 5); // file tabs: guitar tab / drums / chart
    for (const tab of ["Drums", "chart"]) {
      const t = page.getByText(tab, { exact: false }).first();
      if (await has(t)) {
        await t.click();
        await beat(page, 3);
      }
    }
  });

  // ── S6: the flagship — The Open Road annotations ───────────────────────────
  await openSong(page, "The Troubadours", "The Open Road");
  await beat(page, 6); // annotated lead sheet: capo highlight, chorus, conductor cue

  // ── S7–S8: the layers panel — toggle a layer off, then on ──────────────────
  await soft("layer toggles", async () => {
    // The Layers pill carries title="Layers" (accessible name may include a count/icon).
    const layersPill = page.getByTitle("Layers", { exact: true }).or(page.getByRole("button", { name: /layers/i }));
    await layersPill.first().click();
    await beat(page, 2);
    const toggles = page.getByTestId("layer-toggle");
    if ((await toggles.count()) > 0) {
      await toggles.first().click();
      await beat(page, 3); // a layer disappears
      await toggles.first().click();
      await beat(page, 2); // and returns
    }
  });

  // ── S9: direct editing — pick a tool, a preset, draw on the canvas ─────────
  await soft("draw", async () => {
    const pen = page.getByRole("button", { name: /pen|rectangle|select/i }).first();
    if (await has(pen)) await pen.click();
    const preset = page.getByTestId("preset-highlight").or(page.getByTestId("preset-box"));
    if (await has(preset)) await preset.first().click();
    // a short drag across the score area
    const box = await page.viewportSize();
    if (box) {
      await page.mouse.move(box.width * 0.45, box.height * 0.55);
      await page.mouse.down();
      await page.mouse.move(box.width * 0.6, box.height * 0.58, { steps: 12 });
      await page.mouse.up();
    }
    await beat(page, 3);
  });

  // ── S10: transpose the chart (Details panel) ───────────────────────────────
  await soft("transpose", async () => {
    const details = page.getByRole("button", { name: "Details" });
    if (await has(details)) {
      await details.click();
      await beat(page, 2);
      const key = page.getByTestId("transpose-target-key");
      if (await has(key)) {
        await key.selectOption({ index: 2 }).catch(() => {});
        await beat(page, 2);
        const apply = page.getByTestId("transpose-update-key");
        if (await has(apply)) await apply.click();
        await beat(page, 3); // chords rewrite, layout holds
      }
    }
  });

  // ── S11: the setlist ───────────────────────────────────────────────────────
  await gotoBand(page, "The Troubadours");
  await soft("setlist", async () => {
    const setlists = page.getByRole("link", { name: /^setlists$/i }).or(page.getByText("Setlists", { exact: true }));
    if (await has(setlists)) {
      await setlists.first().click();
      await page.waitForLoadState("networkidle");
      await beat(page, 2);
      const sl = page.getByText("Sat @ The Anchor", { exact: false }).first();
      if (await has(sl)) {
        await sl.click();
        await page.waitForLoadState("networkidle");
        await beat(page, 6); // running order: 4 songs, per-member cues, overrides
      }
    }
  });

  // ── S12: bake the concert ──────────────────────────────────────────────────
  await soft("bake", async () => {
    const bake = page.getByTestId("bake-setlist");
    if (await has(bake)) {
      await bake.click();
      await beat(page, 2); // the bake dialog (layer defaults)
      const confirm = page.getByTestId("bake-dialog-confirm");
      if (await has(confirm)) {
        await confirm.click();
        await beat(page, 6); // a concert bundle is produced
      }
    }
  });

  // ── S13–S14: it scales to an orchestra ─────────────────────────────────────
  await login(page, "maestro");
  await openBand(page, "City Chamber Orchestra");
  await beat(page, 2);
  await soft("orchestra part + score", async () => {
    await page.getByTestId("song-link").filter({ hasText: "Eine kleine" }).first().click();
    await page.waitForLoadState("networkidle");
    await beat(page, 6); // Violin I part: conductor cue + player bowing
    const score = page.getByText("Full score", { exact: false }).first();
    if (await has(score)) {
      await score.click();
      await beat(page, 6); // the conductor's score + interpretation layer
    }
  });

  await beat(page, 2);
  expect(true).toBe(true);
});

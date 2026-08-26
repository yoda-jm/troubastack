/**
 * T85 — the visual beat, studio UI: tap the beat control → the frame pulses, and a count-in stops itself.
 *
 * The pure `beatPhase`/`meterGroups` CONTRACT (the shared-vector check A34 also runs) moved to a browser-
 * free vitest suite in T110 — see test/beat-phase.test.ts.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

// Studio UI — tap the beat, the frame pulses, a count-in stops itself.
// ===========================================================================

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
  // Poll the fit-page end-state (the sheet shrinks to a narrow column) rather than sleeping.
  await expect
    .poll(async () => {
      const pb = await page.getByTestId("pdf-page").first().boundingBox();
      const sb = await page.getByTestId("viewer-scroll").boundingBox();
      return pb && sb ? pb.width < sb.width * 0.85 : false;
    })
    .toBe(true);

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

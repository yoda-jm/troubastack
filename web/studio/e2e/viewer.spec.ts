/**
 * View-only annotation viewer e2e (live stack). Registers a user, creates a
 * band + song, uploads a 2-page PDF fixture, imports (via the API, through the
 * page's same-origin session) a rich annotation set covering EVERY object type
 * across two layers in different zones spanning both pages, then drives the
 * viewer: PDF pages + overlays render, the layers panel lists the layers,
 * toggling a layer re-renders, and zoom re-rasterizes. Screenshots land in /tmp.
 *
 * Kept separate from flows.spec.ts so the original 9 flows stay untouched.
 */
import { test, expect, type Page } from "@playwright/test";
import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

// These callbacks run in the browser on a real <canvas>. Locator.evaluate hands
// the callback an Element; narrow to the canvas API inside.
const canvasWidth = (c: Element) => (c as HTMLCanvasElement).width;
const pixelSum = (c: Element) => {
  const canvas = c as HTMLCanvasElement;
  const d = canvas.getContext("2d")!.getImageData(0, 0, canvas.width, canvas.height).data;
  let s = 0;
  for (let i = 0; i < d.length; i++) s += d[i];
  return s;
};

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

/** A rich annotation doc: 2 layers (conductor + a personal layer owned by me),
 *  every object type, spread across page 0 and page 1, varied color/opacity/size. */
function richDoc(fileId: string, myUserId: string) {
  const layers = [
    {
      id: "layer-conductor",
      fileId,
      name: "Conductor cues",
      ownerId: "conductor-user",
      zone: "conductor",
      order: 0,
      access: "ro",
      mandatory: true,
      roleTag: "conductor",
    },
    {
      id: "layer-mine",
      fileId,
      name: "My notes",
      ownerId: myUserId,
      zone: "personal",
      order: 0,
      access: "rw",
      mandatory: false,
      roleTag: "",
    },
  ];

  const objects = [
    // ---- page 0, conductor layer (mandatory) ----
    {
      uuid: "o-free-0",
      layerId: "layer-conductor",
      type: "freehand",
      page: 0,
      text: "",
      points: [
        { x: 0.15, y: 0.2 },
        { x: 0.25, y: 0.32 },
        { x: 0.4, y: 0.22 },
        { x: 0.55, y: 0.38 },
      ],
      style: { color: "#e11d48", opacity: 1, width: 0.01, fontSize: 0 },
    },
    {
      uuid: "o-rect-0",
      layerId: "layer-conductor",
      type: "rect",
      page: 0,
      text: "",
      points: [
        { x: 0.1, y: 0.5 },
        { x: 0.45, y: 0.7 },
      ],
      style: { color: "#2563eb", opacity: 0.9, width: 0.006, fontSize: 0 },
    },
    {
      uuid: "o-text-0",
      layerId: "layer-conductor",
      type: "text",
      page: 0,
      text: "Watch the cue!",
      points: [{ x: 0.12, y: 0.8 }],
      style: { color: "#7c3aed", opacity: 1, width: 0, fontSize: 0.035 },
    },
    // ---- page 0, my personal layer ----
    {
      uuid: "o-hl-0",
      layerId: "layer-mine",
      type: "highlight",
      page: 0,
      text: "",
      points: [
        { x: 0.55, y: 0.12 },
        { x: 0.9, y: 0.2 },
      ],
      style: { color: "#facc15", opacity: 0.4, width: 0, fontSize: 0 },
    },
    // ---- page 1, conductor layer ----
    {
      uuid: "o-ellipse-1",
      layerId: "layer-conductor",
      type: "ellipse",
      page: 1,
      text: "",
      points: [
        { x: 0.2, y: 0.25 },
        { x: 0.6, y: 0.55 },
      ],
      style: { color: "#059669", opacity: 1, width: 0.008, fontSize: 0 },
    },
    // ---- page 1, my personal layer ----
    {
      uuid: "o-line-1",
      layerId: "layer-mine",
      type: "line",
      page: 1,
      text: "",
      points: [
        { x: 0.1, y: 0.7 },
        { x: 0.85, y: 0.85 },
      ],
      style: { color: "#db2777", opacity: 0.85, width: 0.012, fontSize: 0 },
    },
  ];

  return { layers, objects };
}

test("viewer: PDF + annotation layers render, toggle + zoom (screenshots)", async ({ page }) => {
  await register(page, `viewer_${stamp()}`);

  // My user id (for the personal-layer ownership rule).
  const me = await page.evaluate(async (): Promise<{ id: string }> => {
    const r = await fetch("/api/me", { credentials: "include" });
    const j = (await r.json()) as { user: { id: string } };
    return j.user;
  });

  // Band + song.
  const bandName = `ViewBand ${stamp()}`;
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const bandUrl = page.url();
  const bandId = bandUrl.split("/bands/")[1];

  const songTitle = `ViewSong ${stamp()}`;
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(songTitle);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: songTitle }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  const songId = page.url().split("/songs/")[1];

  // Upload the PDF fixture via the Details & files section (open by default).
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);

  // Discover the uploaded file's id, then import a rich annotation set via API
  // (same-origin fetch carries the session cookie).
  const fileId = await page.evaluate(
    async ([b, s]): Promise<string> => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      const j = (await r.json()) as { files: { id: string }[] };
      return j.files[0].id;
    },
    [bandId, songId],
  );

  const doc = richDoc(fileId, me.id);
  const importResult = await page.evaluate(
    async ([b, s, body]): Promise<{
      ok: boolean;
      status: number;
      json: { layers: unknown[]; objects: unknown[] };
    }> => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      const json = (await r.json()) as { layers: unknown[]; objects: unknown[] };
      return { ok: r.ok, status: r.status, json };
    },
    [bandId, songId, doc] as const,
  );
  expect(importResult.ok).toBeTruthy();
  expect(importResult.json.layers.length).toBe(2);
  expect(importResult.json.objects.length).toBe(6);

  // Reload the viewer so it picks up the freshly imported annotations + PDF.
  await page.reload();

  // PDF pages render (2-page fixture → 2 page wrappers).
  await expect(page.getByTestId("pdf-page")).toHaveCount(2);
  const overlays = page.getByTestId("annotation-overlay");
  await expect(overlays).toHaveCount(2);

  // First overlay canvas has a non-zero backing-store size (it rendered).
  await expect
    .poll(async () =>
      page.getByTestId("annotation-overlay").first().evaluate(canvasWidth),
    )
    .toBeGreaterThan(0);

  // Layers panel lists both layers.
  await expect(page.getByTestId("layers-panel")).toBeVisible();
  await expect(page.getByTestId("layer-item")).toHaveCount(2);
  const toggles = page.getByTestId("layer-toggle");
  await expect(toggles).toHaveCount(2);

  // Default zoom is Fit width — the PDF fills the column.
  await expect(page.getByTestId("zoom-mode")).toHaveValue("fit-width");

  // The file picker is present (the new card/tab strip) with one tab per file,
  // the active one selected.
  await expect(page.getByTestId("file-picker")).toBeVisible();
  await expect(page.getByTestId("file-tab")).toHaveCount(1);
  await expect(page.getByTestId("file-tab").first()).toHaveAttribute("aria-selected", "true");

  // At Fit width the first page canvas is wider than its intrinsic size would
  // be at 100% (proving it scaled up to fill the column).
  const fitWidthCanvasW = await page
    .getByTestId("pdf-page")
    .first()
    .locator("canvas.pdf-canvas")
    .evaluate(canvasWidth);
  expect(fitWidthCanvasW).toBeGreaterThan(0);

  await page.screenshot({ path: "/tmp/view-100.png", fullPage: true });

  // Capture overlay pixels before toggling, to prove the toggle re-renders.
  const overlayPixelsBefore = await overlays.first().evaluate(pixelSum);

  // The conductor layer is mandatory → its toggle is disabled+checked.
  const conductorToggle = page
    .getByTestId("layer-item")
    .filter({ hasText: "Conductor cues" })
    .getByTestId("layer-toggle");
  await expect(conductorToggle).toBeChecked();
  await expect(conductorToggle).toBeDisabled();

  // Toggle MY personal layer OFF (it is enabled). Overlay must change.
  const mineToggle = page
    .getByTestId("layer-item")
    .filter({ hasText: "My notes" })
    .getByTestId("layer-toggle");
  await expect(mineToggle).toBeEnabled();
  await expect(mineToggle).toBeChecked();
  await mineToggle.uncheck();
  await expect(mineToggle).not.toBeChecked();

  await expect
    .poll(async () => overlays.first().evaluate(pixelSum))
    .not.toBe(overlayPixelsBefore);

  // Screenshot 2: a layer hidden.
  await page.screenshot({ path: "/tmp/view-toggle.png", fullPage: true });

  // Re-enable, then select an explicit zoom % (re-rasterizes the PDF).
  await mineToggle.check();
  await expect(mineToggle).toBeChecked();

  // Select Actual size (100%) via the zoom-mode select.
  await page.getByTestId("zoom-mode").selectOption("100");
  await expect(page.getByTestId("zoom-mode")).toHaveValue("100");
  await expect
    .poll(async () =>
      page.getByTestId("pdf-page").first().locator("canvas.pdf-canvas").evaluate(canvasWidth),
    )
    .toBeGreaterThan(0);

  // The −/+ buttons step through the percentage stops.
  await page.getByTestId("zoom-in").click();
  await expect(page.getByTestId("zoom-mode")).toHaveValue("125");
  await page.getByTestId("zoom-out").click();
  await expect(page.getByTestId("zoom-mode")).toHaveValue("100");

  // Switch to Fit page mode.
  await page.getByTestId("zoom-mode").selectOption("fit-page");
  await expect(page.getByTestId("zoom-mode")).toHaveValue("fit-page");

  // Screenshot 3: zoomed.
  await page.screenshot({ path: "/tmp/view-zoom.png", fullPage: true });

  // Collapse the layers sidebar → PDF gets the full width; panel disappears.
  await page.getByTestId("sidebar-toggle").click();
  await expect(page.getByTestId("layers-panel")).toHaveCount(0);
  await page.getByTestId("sidebar-toggle").click();
  await expect(page.getByTestId("layers-panel")).toBeVisible();
});

/** Create a band + song and return their ids (logged-in page assumed). */
async function makeBandAndSong(page: Page): Promise<{ bandId: string; songId: string }> {
  const bandName = `MyFilesBand ${stamp()}`;
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const bandId = page.url().split("/bands/")[1];

  const songTitle = `MyFilesSong ${stamp()}`;
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(songTitle);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: songTitle }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  const songId = page.url().split("/songs/")[1];
  return { bandId, songId };
}

test("my-files: per-member selection drives the strip (exclude, reorder, persist, reset)", async ({
  page,
}) => {
  await register(page, `myfiles_${stamp()}`);
  const { bandId, songId } = await makeBandAndSong(page);

  // Upload the same fixture 3 times → 3 distinct pool files, then rename each via
  // the API so the file-tab labels are distinguishable (A, B, C in pool order).
  for (let i = 0; i < 3; i++) {
    await page.getByTestId("file-input").setInputFiles(PDF_PATH);
    await page.getByTestId("file-upload").click();
    await expect(page.getByTestId("file-row")).toHaveCount(i + 1);
  }

  // Fresh uploads all share displayOrder 0, so the default order is otherwise
  // unstable. Assign each a distinct displayOrder (0,1,2) AND a distinct name
  // (A,B,C) so the default my-files view is deterministically A, B, C.
  const fileIds = await page.evaluate(
    async ([b, s]): Promise<string[]> => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      const j = (await r.json()) as { files: { id: string }[] };
      return j.files.map((f) => f.id);
    },
    [bandId, songId],
  );
  expect(fileIds.length).toBe(3);
  const names = ["fileA.pdf", "fileB.pdf", "fileC.pdf"];
  await page.evaluate(
    async ([b, s, ids, ns]) => {
      for (let i = 0; i < ids.length; i++) {
        await fetch(`/api/bands/${b}/songs/${s}/files/${ids[i]}`, {
          method: "PATCH",
          credentials: "include",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ filename: ns[i], displayOrder: i }),
        });
      }
    },
    [bandId, songId, fileIds, names] as const,
  );

  // Open the viewer: default selection = all 3 pool files, customized=false.
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(3);
  await expect(page.getByTestId("my-files-custom")).toHaveCount(0);
  // Default order matches the pool order A, B, C.
  await expect(page.getByTestId("file-tab").nth(0)).toContainText("fileA.pdf");
  await expect(page.getByTestId("file-tab").nth(1)).toContainText("fileB.pdf");
  await expect(page.getByTestId("file-tab").nth(2)).toContainText("fileC.pdf");

  // Open the my-files editor; it lists the whole pool (3 rows).
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("my-files-panel")).toBeVisible();
  await expect(page.getByTestId("my-files-row")).toHaveCount(3);

  // Screenshot 1: editor open showing the pool with includes/reorder.
  // Exclude fileA (click its include checkbox — controlled, applies via PUT).
  const rowA = page.getByTestId("my-files-row").filter({ hasText: "fileA.pdf" });
  await rowA.getByTestId("my-files-include").click();

  // The strip now shows 2 tabs (B, C) and the custom pill appears.
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  await expect(page.getByTestId("my-files-custom")).toBeVisible();

  // Reorder the remaining two: move fileC up so the order becomes C, B.
  const rowC = page.getByTestId("my-files-row").filter({ hasText: "fileC.pdf" });
  await rowC.getByTestId("my-files-up").click();
  await expect(page.getByTestId("file-tab").nth(0)).toContainText("fileC.pdf");
  await expect(page.getByTestId("file-tab").nth(1)).toContainText("fileB.pdf");

  await page.screenshot({ path: "/tmp/myfiles-editor.png", fullPage: true });

  // Close the editor and screenshot the customized 2-tab strip.
  await page.getByTestId("my-files-edit").click();
  await expect(page.getByTestId("my-files-panel")).toHaveCount(0);
  await page.screenshot({ path: "/tmp/myfiles-strip.png", fullPage: true });

  // Persistence: reload → still 2 tabs in C, B order, still customized.
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(2);
  await expect(page.getByTestId("my-files-custom")).toBeVisible();
  await expect(page.getByTestId("file-tab").nth(0)).toContainText("fileC.pdf");
  await expect(page.getByTestId("file-tab").nth(1)).toContainText("fileB.pdf");

  // Reset to all → back to 3 tabs, custom pill gone, default A, B, C order.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("my-files-reset").click();
  await expect(page.getByTestId("file-tab")).toHaveCount(3);
  await expect(page.getByTestId("my-files-custom")).toHaveCount(0);
  await expect(page.getByTestId("file-tab").nth(0)).toContainText("fileA.pdf");
  await expect(page.getByTestId("file-tab").nth(1)).toContainText("fileB.pdf");
  await expect(page.getByTestId("file-tab").nth(2)).toContainText("fileC.pdf");

  // Persists across reload as default (not customized).
  await page.reload();
  await expect(page.getByTestId("file-tab")).toHaveCount(3);
  await expect(page.getByTestId("my-files-custom")).toHaveCount(0);
});

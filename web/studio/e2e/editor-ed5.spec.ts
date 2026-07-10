/**
 * Editor cycle ed5 — bug repros (#1, #2) + features (#4, #5).
 *
 *  #1 — text is hard to select: a generous hit area lets a click slightly OUTSIDE
 *       the glyph extents still select the text object (selected-bbox appears).
 *  #2 — small annotations: a drag starting at a small rect's CENTER MOVES it
 *       (position changes, size unchanged) — it does not grab a handle and scale.
 *  #4 — lock/unlock icon per shared layer flips its access (rw↔ro); shown only for
 *       the owner/admin.
 *  #5 — shape style presets: Highlight = rect with fill+multiply+no-stroke; Outline
 *       = rect with stroke only; a legacy "highlight" object still renders.
 *
 * Screenshots: /tmp/ed5-presets.png, /tmp/ed5-lock.png, /tmp/ed5-smallmove.png.
 */
import { test, expect, type Page } from "@playwright/test";
import { scrollFracIntoBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
import { fileURLToPath } from "node:url";

const stamp = () => `${Date.now()}${Math.floor(Math.random() * 1000)}`;
const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function register(page: Page, username: string, password = "secret123") {
  await page.goto("/register");
  await page.getByTestId("username").fill(username);
  await page.getByTestId("displayName").fill(`Display ${username}`);
  await page.getByTestId("password").fill(password);
  await page.getByTestId("submit").click();
  await expect(page).toHaveURL(/\/bands$/);
}

async function myUserId(page: Page): Promise<string> {
  return page.evaluate(async (): Promise<string> => {
    const r = await fetch("/api/me", { credentials: "include" });
    const j = (await r.json()) as { user: { id: string } };
    return j.user.id;
  });
}

async function createBandAndOpen(page: Page, bandName: string): Promise<{ url: string; id: string }> {
  await page.getByTestId("new-band-btn").click();
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const url = page.url();
  return { url, id: url.split("/bands/")[1] };
}

async function createSongAndOpen(page: Page, title: string): Promise<string> {
  await page.getByTestId("new-song-btn").click();
  await page.getByTestId("song-title").fill(title);
  await page.getByTestId("create-song").click();
  await page.getByTestId("song-link").filter({ hasText: title }).click();
  await expect(page).toHaveURL(/\/bands\/[^/]+\/songs\/[^/]+$/);
  return page.url().split("/songs/")[1];
}

async function uploadPdf(page: Page) {
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
}

async function firstFileId(page: Page, bandId: string, songId: string): Promise<string> {
  return page.evaluate(
    async ([b, s]): Promise<string> => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/files`, { credentials: "include" });
      const j = (await r.json()) as { files: { id: string }[] };
      return j.files[0].id;
    },
    [bandId, songId],
  );
}

async function importDoc(page: Page, bandId: string, songId: string, doc: unknown) {
  return page.evaluate(
    async ([b, s, body]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations/import`, {
        method: "POST",
        credentials: "include",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      return { ok: r.ok, status: r.status };
    },
    [bandId, songId, doc] as const,
  );
}

type WireObject = {
  uuid: string;
  type: string;
  layerId: string;
  points: { x: number; y: number }[];
  style: { color: string; opacity: number; width: number; fontSize: number; fill?: boolean; stroke?: boolean; blend?: string };
  text?: string;
};

async function getAnnotations(page: Page, bandId: string, songId: string) {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as { objects: WireObject[]; layers: { id: string }[] };
    },
    [bandId, songId],
  );
}

// Scroll a page-relative Y fraction into the visible band (the full-viewport editor's
// page can exceed the scroll height / sit under the floating chrome — T27 stage 3),
// then return the page box.
async function pageBoxFor(page: Page, ...pys: number[]) {
  return scrollFracIntoBand(page, ...pys);
}

/** Click at a TRUE page-relative fraction (scrolled into view). */
async function clickPageFrac(page: Page, px: number, py: number) {
  const box = await pageBoxFor(page, py);
  await page.mouse.click(box.x + box.width * px, box.y + box.height * py);
}

/** Drag between two TRUE page-relative fractions (scrolled into view). */
async function dragPageFrac(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 14) {
  const box = await pageBoxFor(page, fy, ty);
  await page.mouse.move(box.x + box.width * fx, box.y + box.height * fy);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * tx, box.y + box.height * ty, { steps });
  await page.mouse.up();
}

const STYLE = { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.04 };

function personalLayer(fileId: string, me: string) {
  return {
    id: "layer-mine",
    fileId,
    name: "My notes",
    ownerId: me,
    zone: "personal",
    order: 0,
    access: "rw",
    mandatory: false,
    roleTag: "",
  };
}

function sharedLayer(fileId: string, me: string) {
  return {
    id: "layer-shared",
    fileId,
    name: "Section markings",
    ownerId: me,
    zone: "shared",
    order: 1,
    access: "rw",
    mandatory: false,
    roleTag: "",
  };
}

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
}

async function setup(page: Page, prefix: string) {
  await register(page, `${prefix}_${stamp()}`);
  const band = await createBandAndOpen(page, `${prefix}Band ${stamp()}`);
  const songId = await createSongAndOpen(page, `${prefix}Song ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);
  return { bandId: band.id, songId, fileId, me };
}

// ===========================================================================
// #1 (repro) — clicking slightly OUTSIDE a text's glyphs still selects it.
// ===========================================================================
test("editor: a click just outside the text glyphs still selects the text (#1)", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "TxtPad");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-text",
        layerId: "layer-mine",
        type: "text",
        points: [{ x: 0.4, y: 0.2 }],
        page: 0,
        text: "Cue",
        style: { ...STYLE, fontSize: 0.03 },
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  await page.getByTestId("tool-select").click();
  // The glyph box for "Cue" at fontSize 0.03 is small, anchored top-left at
  // (0.4,0.2). Click ABOVE-LEFT of the anchor (0.375,0.176) — ~0.024 page-frac
  // outside the glyph extents: BEYOND the old flat 0.02 hit pad (so it missed
  // before), but inside the new generous text hit area (Bug #1 fix).
  await clickPageFrac(page, 0.375, 0.176);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
});

// ===========================================================================
// #2 (repro) — a drag from a SMALL rect's center MOVES it (no scale-away).
// ===========================================================================
test("editor: dragging a small rect's center moves it without resizing (#2)", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "SmallMv");
  // A SMALL rect: ~0.02 × 0.02 page-relative (well under the handle-show threshold,
  // which needs ≥ ~3× the 10px handle on both axes).
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-small",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.30, y: 0.15 }, { x: 0.32, y: 0.17 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  const before = await getAnnotations(page, bandId, songId);
  const r0 = before.objects.find((o) => o.uuid === "obj-small")!;
  const w0 = Math.abs(r0.points[1].x - r0.points[0].x);
  const h0 = Math.abs(r0.points[1].y - r0.points[0].y);

  // Select it via the list (deterministic), then verify NO resize handles show
  // (small object → move-only, Bug #2).
  await page.getByTestId("tool-select").click();
  await openDrawer(page, "annotations");
  await page.getByTestId("annotation-item").filter({ hasText: "rect" }).first().click();
  const bbox = page.getByTestId("selected-bbox");
  await expect(bbox).toHaveCount(1);
  await expect(page.getByTestId("resize-handle")).toHaveCount(0);

  await page.screenshot({ path: "/tmp/ed5-smallmove.png", fullPage: true });

  // Drag from the rect's REAL on-screen center (read from the selection bbox) to a
  // point well to the lower-right. A body-drag must MOVE it (Bug #2), not resize.
  // The press lands on the edit-canvas (the bbox overlay is pointer-events:none);
  // hit-testing the small rect's center starts a MOVE gesture, not a resize.
  // Ensure the bbox is actually within the viewport before reading its pixel box
  // (the list-select may have scrolled it just out of view).
  await bbox.first().scrollIntoViewIfNeeded();
  const sb = (await bbox.first().boundingBox())!;
  const cx = sb.x + sb.width / 2;
  const cy = sb.y + sb.height / 2;
  await page.mouse.move(cx, cy);
  await page.mouse.down();
  // Mostly-horizontal drag (stays on the page canvas) so pointermove fires and the
  // move preview builds; the assertion only needs a clear horizontal displacement.
  await page.mouse.move(cx + 60, cy + 20, { steps: 6 });
  await page.mouse.move(cx + 240, cy + 60, { steps: 16 });
  await page.mouse.up();

  await expect
    .poll(async () => {
      const doc2 = await getAnnotations(page, bandId, songId);
      const o = doc2.objects.find((x) => x.uuid === "obj-small");
      if (!o) return "missing";
      const w1 = Math.abs(o.points[1].x - o.points[0].x);
      const h1 = Math.abs(o.points[1].y - o.points[0].y);
      const cx0 = (r0.points[0].x + r0.points[1].x) / 2;
      const cx1 = (o.points[0].x + o.points[1].x) / 2;
      // Size unchanged (within float slop) AND the center actually moved right.
      const sizeSame = Math.abs(w1 - w0) < 0.004 && Math.abs(h1 - h0) < 0.004;
      const moved = cx1 - cx0 > 0.1;
      return sizeSame && moved ? "moved-not-scaled" : `bad w${w1.toFixed(3)} h${h1.toFixed(3)} dx${(cx1 - cx0).toFixed(3)}`;
    })
    .toBe("moved-not-scaled");
});

// ===========================================================================
// #5 — shape style presets (Outline / Box / Highlight) on rect; legacy renders.
// ===========================================================================
test("editor: Highlight preset draws a filled multiply rect; Outline is stroke-only (#5)", async ({
  page,
}) => {
  const { bandId, songId } = await setup(page, "Preset");
  await page.reload();
  await openEditorReady(page);
  await openDrawer(page, "layers");
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  // The drawer overlays the ctx-bar's right end (presets + ⋯); it's done its job
  // (new-layer) — close it so the preset trio and the ⋯ popover are reachable (T33).
  await closeDrawer(page);

  // Highlight preset → rect with fill+multiply+no-stroke.
  await page.getByTestId("tool-rect").click();
  await page.getByTestId("preset-highlight").click();
  await page.getByTestId("style-more").click(); // T33: fill/border/blend live in the ⋯ popover
  await expect(page.getByTestId("style-fill")).toBeChecked();
  await expect(page.getByTestId("style-stroke")).not.toBeChecked();
  await expect(page.getByTestId("style-blend")).toHaveValue("multiply");
  await dragPageFrac(page, 0.2, 0.12, 0.5, 0.22, 10);

  // Outline preset → rect with stroke only.
  await page.getByTestId("preset-outline").click();
  await page.getByTestId("style-more").click(); // reopen (the drag above dismissed it)
  await expect(page.getByTestId("style-stroke")).toBeChecked();
  await expect(page.getByTestId("style-fill")).not.toBeChecked();
  await page.getByTestId("tool-rect").click();
  await dragPageFrac(page, 0.2, 0.3, 0.5, 0.4, 10);

  // Box preset → stroke + fill.
  await page.getByTestId("preset-box").click();
  await page.getByTestId("tool-rect").click();
  await dragPageFrac(page, 0.2, 0.48, 0.5, 0.58, 10);

  await page.screenshot({ path: "/tmp/ed5-presets.png", fullPage: true });

  // Persisted shapes carry the right flags; no "highlight"-typed object created.
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      const rects = doc.objects.filter((o) => o.type === "rect");
      if (rects.length < 3) return `only ${rects.length} rects`;
      const hl = rects.find((o) => o.style.fill === true && o.style.stroke === false && o.style.blend === "multiply");
      const outline = rects.find((o) => o.style.stroke === true && o.style.fill === false);
      const box = rects.find((o) => o.style.fill === true && o.style.stroke === true);
      const noHighlightType = !doc.objects.some((o) => o.type === "highlight");
      return hl && outline && box && noHighlightType ? "ok" : "missing-variant";
    })
    .toBe("ok");
});

test("editor: a legacy highlight object still renders (#5 back-compat)", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "Legacy");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-legacy-hi",
        layerId: "layer-mine",
        type: "highlight", // legacy type, no fill/stroke/blend flags
        points: [{ x: 0.2, y: 0.15 }, { x: 0.6, y: 0.22 }],
        page: 0,
        text: "",
        style: { ...STYLE, opacity: 0.35 },
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  // It round-trips as a "highlight" object and is selectable on the canvas.
  const round = await getAnnotations(page, bandId, songId);
  expect(round.objects.some((o) => o.uuid === "obj-legacy-hi" && o.type === "highlight")).toBeTruthy();
  await page.getByTestId("tool-select").click();
  await clickPageFrac(page, 0.4, 0.185);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
});

// ===========================================================================
// #4 — lock/unlock icon on a shared layer the viewer owns flips its access.
// ===========================================================================
test("editor: lock icon appears for a shared layer the owner sees and flips state (#4)", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "Lock");
  const doc = {
    layers: [sharedLayer(fileId, me)],
    objects: [],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  // The owner sees a lock/unlock toggle on the shared layer; it starts UNLOCKED (rw).
  // T27 stage 3: the layers panel is in the on-demand drawer.
  await openDrawer(page, "layers");
  const toggle = page.getByTestId("layer-access-toggle");
  await expect(toggle).toHaveCount(1);
  await expect(toggle).toHaveAttribute("aria-pressed", "false");

  // Lock it → access becomes ro (aria-pressed true) and persists in HEAD.
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-pressed", "true");
  await page.screenshot({ path: "/tmp/ed5-lock.png", fullPage: true });

  await expect
    .poll(async () => {
      const d = await page.evaluate(
        async ([b, s]) => {
          const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
          return (await r.json()) as { layers: { id: string; access: string }[] };
        },
        [bandId, songId],
      );
      return d.layers.find((l) => l.id === "layer-shared")?.access;
    })
    .toBe("ro");

  // Unlock again → back to rw.
  await toggle.click();
  await expect(toggle).toHaveAttribute("aria-pressed", "false");
});

// ===========================================================================
// #3 — conductor zone is role-governed (client mirror). A plain member (not a
// conductor) sees the conductor-zone layer READ-ONLY: it is locked in the panel
// and NOT offered in the active-layer (editable) selector. The authoritative
// server-side enforcement is covered by the Go hub tests.
// ===========================================================================
test("editor: a non-conductor sees the conductor zone read-only (#3)", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "Cond");
  // A conductor-zone "cues" layer (owned by someone else; the viewer is a plain
  // member, not a conductor) + a personal layer the member CAN edit.
  const doc = {
    layers: [
      {
        id: "layer-cond",
        fileId,
        name: "Conductor cues",
        ownerId: "the-conductor",
        zone: "conductor",
        order: 0,
        access: "ro",
        mandatory: true,
        roleTag: "conductor",
      },
      personalLayer(fileId, me),
    ],
    objects: [
      {
        uuid: "obj-cue",
        layerId: "layer-cond",
        type: "rect",
        points: [{ x: 0.2, y: 0.12 }, { x: 0.6, y: 0.22 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  // The conductor layer shows the read-only lock cue in the layers panel…
  // T27 stage 3: the layers panel is in the on-demand drawer.
  await openDrawer(page, "layers");
  await expect(page.getByTestId("layer-lock").first()).toBeVisible();
  // …and is NOT in the active-layer (editable) selector — only my personal layer is.
  const options = await page
    .getByTestId("active-layer")
    .locator("option")
    .evaluateAll((opts) => opts.map((o) => o.getAttribute("value") ?? ""));
  expect(options).not.toContain("layer-cond");
  expect(options).toContain("layer-mine");

  // Focus the conductor layer (its row) so its annotations list, then select the
  // cue → the read-only bbox lock cue is shown (it can be inspected, not edited).
  await page.getByTestId("tool-select").click();
  await page
    .getByTestId("layer-row")
    .filter({ hasText: "Conductor cues" })
    .click();
  await openDrawer(page, "annotations");
  await page.getByTestId("annotation-item").first().click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await page.screenshot({ path: "/tmp/ed5-conductor.png", fullPage: true });
});

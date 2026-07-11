/**
 * Editor features:
 *  #3 — TEXT bounding box: a text object has a real bbox (measured width × line
 *       height), so clicking ON the text selects it and shows a selection box.
 *  #4 — cross-page list navigation: clicking a list item for an object on a
 *       DIFFERENT page scrolls that page into view and selects the object.
 *  #5 — resize handles: a selected object on the ACTIVE editable layer shows
 *       corner resize handles; dragging one resizes the object and persists.
 *
 * Screenshots: /tmp/ed4-text-bbox.png, /tmp/ed4-resize.png.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer, closeDrawer } from "./fullscreen-helpers";
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
  // T36: file management moved into the editor's Details panel — open it to reach the
  // upload form, then close it so the canvas is unobstructed for whatever follows.
  await page.getByTestId("my-files-edit").click();
  await page.getByTestId("file-input").setInputFiles(PDF_PATH);
  await page.getByTestId("file-upload").click();
  await expect(page.getByTestId("file-row")).toHaveCount(1);
  await page.getByTestId("my-files-edit").click();
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
  style: { color: string; opacity: number; width: number; fontSize: number };
  text?: string;
};

async function getAnnotations(page: Page, bandId: string, songId: string) {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as { objects: WireObject[] };
    },
    [bandId, songId],
  );
}

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 12) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => top + bandH * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

/** Click at a TRUE page-relative fraction (px,py) using the full page box —
 *  matches the app's pointer→page mapping (divide by the full page rect). The
 *  page's top region must be on-screen (we scroll its wrapper into view first). */
async function clickPageFrac(page: Page, px: number, py: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  await page.mouse.click(box.x + box.width * px, box.y + box.height * py);
}

/** Drag between two TRUE page-relative fractions (full page box mapping). */
async function dragPageFrac(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 14) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
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

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
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
// #3 — clicking ON a text object selects it (real bbox) + shows the bbox.
// ===========================================================================
test("editor: clicking a text object selects it via its bounding box", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "Txt");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-text",
        layerId: "layer-mine",
        type: "text",
        points: [{ x: 0.2, y: 0.2 }],
        page: 0,
        text: "HELLO WORLD",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  // The text anchor is at (0.2,0.2); the glyphs extend to the RIGHT and DOWN.
  // Click a point clearly to the right of the anchor (page-frac 0.3,0.21): this
  // MISSES a zero-size (anchor-only) box but HITS the measured text box.
  await page.getByTestId("tool-select").click();
  await clickPageFrac(page, 0.3, 0.21);

  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  // The bbox must be wider than it is tall (a line of text), proving a measured
  // width rather than a tiny square.
  const dims = await page.getByTestId("selected-bbox").first().boundingBox();
  expect(dims!.width).toBeGreaterThan(dims!.height);

  await page.screenshot({ path: "/tmp/ed4-text-bbox.png", fullPage: true });
});

// ===========================================================================
// #4 — clicking a list item navigates to the object's page (cross-page).
// ===========================================================================
test("editor: clicking a list item scrolls a different page into view + selects", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "Nav");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-p1",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.2, y: 0.1 }, { x: 0.4, y: 0.25 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      {
        uuid: "obj-p2",
        layerId: "layer-mine",
        type: "text",
        points: [{ x: 0.3, y: 0.5 }],
        page: 1, // page 2
        text: "PAGE TWO CUE",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  // Two pages render; ensure we're scrolled to the TOP (page 1 in view).
  await page.getByTestId("viewer-scroll").evaluate((el) => el.scrollTo({ top: 0 }));
  await expect(page.getByTestId("pdf-page")).toHaveCount(2);

  // The page-2 object's list item (focused layer = my notes, the only layer).
  // T27 stage 3: the annotation list is on the drawer's Annotations tab.
  await openDrawer(page, "annotations");
  const item = page.getByTestId("annotation-item").filter({ hasText: "PAGE TWO" });
  await expect(item).toHaveCount(1);
  await item.click();

  // The object is selected…
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // …and page 2's wrapper is now within the scroll column's viewport. Compare
  // the two elements' on-screen boxes (Playwright reports them in CSS px).
  await expect
    .poll(async () => {
      const scroll = await page.getByTestId("viewer-scroll").boundingBox();
      const p2 = await page.getByTestId("pdf-page").nth(1).boundingBox();
      if (!scroll || !p2) return false;
      // page-2 intersects the visible column band.
      return p2.y < scroll.y + scroll.height && p2.y + p2.height > scroll.y;
    })
    .toBeTruthy();
});

// ===========================================================================
// #5 — resize handles: drag a corner handle to grow a rect; it persists.
// ===========================================================================
test("editor: a selected rect on the active layer shows resize handles and resizes", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "Rsz");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "obj-rect",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.3, y: 0.12 }, { x: 0.5, y: 0.26 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  // T27 stage 3: fit-page so the whole page (incl. the top-of-page rect + its resize
  // handles) is visible clear of the floating chrome, so the corner-handle grab is
  // unambiguous and the outward drag stays on-canvas.
  await page.getByTestId("zoom-mode").selectOption("fit-page");

  const before = await getAnnotations(page, bandId, songId);
  const rectBefore = before.objects.find((o) => o.uuid === "obj-rect")!;

  // Select via the annotation list (deterministic), on the ACTIVE layer.
  await page.getByTestId("tool-select").click();
  await openDrawer(page, "annotations");
  await page.getByTestId("annotation-item").filter({ hasText: "rect" }).first().click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // The four corner handles are shown for an active-editable selection.
  await expect(page.getByTestId("resize-handle")).toHaveCount(4);

  await page.screenshot({ path: "/tmp/ed4-resize.png", fullPage: true });

  // Drag the SE corner (page-frac 0.5,0.26) outward to (0.72,0.42) → bigger rect.
  // True page-frac drag so the press lands on the handle's grab zone. Dismiss the
  // drawer first so the drag's right-side endpoint isn't intercepted by it.
  await closeDrawer(page);
  await dragPageFrac(page, 0.5, 0.26, 0.72, 0.42, 16);

  // The persisted rect is larger than before (its bbox width + height grew).
  await expect
    .poll(async () => {
      const doc2 = await getAnnotations(page, bandId, songId);
      const o = doc2.objects.find((x) => x.uuid === "obj-rect");
      if (!o) return false;
      const w0 = Math.abs(rectBefore.points[1].x - rectBefore.points[0].x);
      const w1 = Math.abs(o.points[1].x - o.points[0].x);
      const h1 = Math.abs(o.points[1].y - o.points[0].y);
      const h0 = Math.abs(rectBefore.points[1].y - rectBefore.points[0].y);
      return w1 > w0 + 0.05 && h1 > h0 + 0.05;
    })
    .toBeTruthy();
});

/**
 * Unified pick / selection priority + action cursors (refine).
 *
 * One shared pickAt drives BOTH the click gesture and the hover cursor. This
 * spec seeds overlapping objects via the annotation import API and asserts the
 * four priority cases, plus the hover cursor mapping (move over a body, a
 * *-resize over a handle).
 *
 * Screenshots: /tmp/pick-overlap.png, /tmp/cursor-move.png.
 */
import { test, expect, type Page } from "@playwright/test";
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
  await page.getByTestId("band-name").fill(bandName);
  await page.getByTestId("create-band").click();
  await page.getByTestId("band-link").filter({ hasText: bandName }).click();
  await expect(page.getByTestId("band-title")).toHaveText(bandName);
  const url = page.url();
  return { url, id: url.split("/bands/")[1] };
}

async function createSongAndOpen(page: Page, title: string): Promise<string> {
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

/** Click at a TRUE page-relative fraction using the full page box. */
async function clickPageFrac(page: Page, px: number, py: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  await page.mouse.click(box.x + box.width * px, box.y + box.height * py);
}

/** Move (hover, no button) to a TRUE page-relative fraction. */
async function hoverPageFrac(page: Page, px: number, py: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  await page.mouse.move(box.x + box.width * px, box.y + box.height * py);
}

/** The uuid of the single selected object (read off the selected-bbox overlay). */
async function selectedUuid(page: Page): Promise<string | null> {
  const bb = page.getByTestId("selected-bbox");
  if ((await bb.count()) !== 1) return null;
  return bb.first().getAttribute("data-uuid");
}

/** The inline cursor on the edit canvas. */
async function canvasCursor(page: Page): Promise<string> {
  return page.getByTestId("edit-canvas").first().evaluate((el) => el.style.cursor);
}

// ===========================================================================
// B — pick priority among overlapping objects.
// ===========================================================================
test("pick: small box overlapping a big box → SMALL box wins in the overlap", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "PickSmall");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      // Big box drawn FIRST (lower z), covering a wide region.
      {
        uuid: "big",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.1, y: 0.1 }, { x: 0.7, y: 0.6 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      // Small box drawn AFTER (higher z), inside the big one.
      {
        uuid: "small",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.3, y: 0.3 }, { x: 0.42, y: 0.42 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Click in the OVERLAP (inside both boxes) → the SMALL box must be selected.
  await clickPageFrac(page, 0.36, 0.36);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  expect(await selectedUuid(page)).toBe("small");
  await page.screenshot({ path: "/tmp/pick-overlap.png", fullPage: true });

  // To get the BIG box you click where only IT is (outside the small box).
  await clickPageFrac(page, 0.6, 0.5);
  expect(await selectedUuid(page)).toBe("big");
});

test("pick: thin line crossing a rect → click ON the line selects the LINE", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "PickLine");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "rect",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.2, y: 0.2 }, { x: 0.7, y: 0.6 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      // A near-horizontal line crossing the rect through y≈0.4.
      {
        uuid: "line",
        layerId: "layer-mine",
        type: "line",
        points: [{ x: 0.2, y: 0.4 }, { x: 0.7, y: 0.4 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Click ON the line (its midpoint) inside the rect → LINE wins (you targeted it).
  await clickPageFrac(page, 0.45, 0.4);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  expect(await selectedUuid(page)).toBe("line");
});

test("pick: inside rect but only within the line's BBOX (off the segment) → RECT", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "PickRectBbox");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "rect",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.2, y: 0.2 }, { x: 0.7, y: 0.6 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      {
        uuid: "line",
        layerId: "layer-mine",
        type: "line",
        points: [{ x: 0.2, y: 0.4 }, { x: 0.7, y: 0.4 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Inside the rect, within the LINE's bbox (x∈[0.2,0.7]) but FAR from the
  // segment (y=0.25, while the line sits at y=0.4) → containment beats the
  // narrow-bbox proximity, so the RECT wins.
  await clickPageFrac(page, 0.45, 0.25);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  expect(await selectedUuid(page)).toBe("rect");
});

test("pick: click just outside a text glyph but within its pad → TEXT", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "PickText");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "text",
        layerId: "layer-mine",
        type: "text",
        points: [{ x: 0.3, y: 0.3 }],
        page: 0,
        text: "HELLO",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Just ABOVE the glyph anchor (y=0.285 vs anchor 0.3) — outside the glyph box
  // but within the generous text pad, nothing else underneath → TEXT selected.
  await clickPageFrac(page, 0.31, 0.285);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  expect(await selectedUuid(page)).toBe("text");
});

// ===========================================================================
// C — action cursors on hover.
// ===========================================================================
test("cursor: hover over a movable body → move; over a resize handle → *-resize", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "Cursor");
  // Keep the whole box in the UPPER part of the page so its body AND its SE
  // corner handle stay within the (1280×720) viewport for real hover events.
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "box",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.2, y: 0.12 }, { x: 0.5, y: 0.36 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Hover over the body of the (editable-now) rect → "move".
  await hoverPageFrac(page, 0.35, 0.24);
  await expect.poll(() => canvasCursor(page)).toBe("move");
  await page.screenshot({ path: "/tmp/cursor-move.png", fullPage: true });

  // Select it so handles appear, then hover the SE corner handle → "nwse-resize".
  await clickPageFrac(page, 0.35, 0.24);
  await expect(page.getByTestId("resize-handle")).toHaveCount(4);
  // Hover the SE corner handle by its real on-screen centre (it sits at
  // (maxX,maxY) = (0.5,0.36)); reading the handle's box avoids viewport-clip math.
  const seBox = await page
    .locator('[data-testid="resize-handle"][data-handle="se"]')
    .boundingBox();
  expect(seBox).not.toBeNull();
  await page.mouse.move(seBox!.x + seBox!.width / 2, seBox!.y + seBox!.height / 2);
  await expect.poll(() => canvasCursor(page)).toMatch(/-resize$/);

  // Hover over clearly empty space (still within the viewport) → cursor clears to
  // the CSS default ("" inline).
  await hoverPageFrac(page, 0.85, 0.1);
  await expect.poll(() => canvasCursor(page)).toBe("");
});

test("cursor: hover a non-editable (locked) object in select mode → not-allowed", async ({
  page,
}) => {
  const { bandId, songId, fileId } = await setup(page, "CursorRO");
  // A shared layer owned by someone ELSE and read-only → visible (shared zone)
  // but not editable by me: in select mode it shows the disabled cursor.
  const doc = {
    layers: [
      {
        id: "layer-ro",
        fileId,
        name: "Shared (locked)",
        ownerId: "someone-else",
        zone: "shared",
        order: 0,
        access: "ro",
        mandatory: false,
        roleTag: "",
      },
    ],
    objects: [
      {
        uuid: "robox",
        layerId: "layer-ro",
        type: "rect",
        points: [{ x: 0.2, y: 0.12 }, { x: 0.5, y: 0.36 }],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  await page.getByTestId("tool-select").click();

  // Hover the body of the not-editable rect → "not-allowed" (you can still click
  // to inspect it, but you can't edit it here).
  await hoverPageFrac(page, 0.35, 0.24);
  await expect.poll(() => canvasCursor(page)).toBe("not-allowed");
});

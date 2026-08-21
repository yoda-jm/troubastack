/**
 * Live annotation EDITOR — locked-object protection, cross-layer selection, and
 * selection↔style restyle. These specs build on the same live stack as the
 * other editor specs (Go core + Vite SPA, same-origin cookies).
 *
 * Reproduce-first concern: "I can still modify things in a locked layer." A band
 * member who is NOT the owner of a read-only layer must not be able to move,
 * delete, or restyle an object on it — not even transiently (no mutation sent,
 * no flicker). The server `forbidden` reject is only a backstop.
 *
 * Screenshots: /tmp/ed3-restyle.png, /tmp/ed3-locked-bbox.png,
 * /tmp/ed3-crosslayer.png.
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

async function importDoc(
  page: Page,
  bandId: string,
  songId: string,
  doc: unknown,
): Promise<{ ok: boolean; status: number }> {
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

async function getAnnotations(
  page: Page,
  bandId: string,
  songId: string,
): Promise<{ layers: { id: string }[]; objects: WireObject[] }> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as { layers: { id: string }[]; objects: WireObject[] };
    },
    [bandId, songId],
  );
}

// These act on SPECIFIC imported objects, so they use TRUE page-relative positions
// (scroll the target into the clear band, then map against the page box) — not the
// band-relative "draw anywhere" mapping. T27 stage 3.
async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 8) {
  const box = await scrollFracIntoBand(page, fy, ty);
  await page.mouse.move(box.x + box.width * fx, box.y + box.height * fy);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * tx, box.y + box.height * ty, { steps });
  await page.mouse.up();
}

async function clickOnPage(page: Page, fx: number, fy: number) {
  const box = await scrollFracIntoBand(page, fy);
  await page.mouse.click(box.x + box.width * fx, box.y + box.height * fy);
}

const objectCount = (page: Page) =>
  page
    .getByTestId("object-count")
    .innerText()
    .then((t) => parseInt(t, 10));

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}

const STYLE = { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.03 };

/** A two-layer doc: one LOCKED layer (ro, owned by another member) holding a
 *  rect, plus one editable shared (rw) layer holding a rect. `me` is the
 *  current member's id (so the locked layer is genuinely not-mine + ro). */
function lockedAndEditableDoc(fileId: string, _me: string) {
  return {
    layers: [
      {
        id: "layer-locked",
        fileId,
        name: "Locked layer",
        ownerId: "other-member",
        zone: "shared",
        order: 0,
        access: "ro",
        mandatory: false,
        roleTag: "",
      },
      {
        id: "layer-editable",
        fileId,
        name: "Editable layer",
        ownerId: "_shared_",
        zone: "shared",
        order: 1,
        access: "rw",
        mandatory: false,
        roleTag: "",
      },
    ],
    objects: [
      {
        uuid: "obj-locked",
        layerId: "layer-locked",
        type: "rect",
        points: [
          { x: 0.15, y: 0.12 },
          { x: 0.45, y: 0.4 },
        ],
        page: 0,
        text: "",
        style: STYLE,
      },
      {
        uuid: "obj-editable",
        layerId: "layer-editable",
        type: "rect",
        points: [
          { x: 0.6, y: 0.12 },
          { x: 0.85, y: 0.32 },
        ],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
}

/** Setup: a band (the current user is its admin/owner = member 1) plus a locked
 *  layer owned by "other-member" (member 2's identity), seeded with objects. */
async function openLockedSong(
  page: Page,
): Promise<{ bandId: string; songId: string; me: string }> {
  await register(page, `lk_${stamp()}`);
  const band = await createBandAndOpen(page, `LkBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `LkSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);
  const res = await importDoc(page, band.id, songId, lockedAndEditableDoc(fileId, me));
  expect(res.ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  return { bandId: band.id, songId, me };
}

// ===========================================================================
// REPRODUCE-FIRST — a locked-layer object must NOT be movable/deletable.
// ===========================================================================
test("editor: object on a locked layer cannot be moved or deleted", async ({ page }) => {
  const { bandId, songId } = await openLockedSong(page);

  const before = await getAnnotations(page, bandId, songId);
  const lockedBefore = before.objects.find((o) => o.uuid === "obj-locked")!;
  expect(lockedBefore).toBeTruthy();

  // Focus the locked layer so its object is visible + listed.
  await page
    .getByTestId("layer-item")
    .filter({ hasText: "Locked layer" })
    .getByTestId("layer-row")
    .click();

  // Select the locked object via the annotation list (deterministic).
  await page.getByTestId("tool-select").click();
  await openDrawer(page, "annotations");
  const lockedItem = page.getByTestId("annotation-item").filter({ hasText: "rect" }).first();
  await lockedItem.click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // The locked cue must show and the delete button must be disabled (UI prevents).
  await expect(page.getByTestId("bbox-lock")).toBeVisible();
  await openDrawer(page, "layers");
  await expect(page.getByTestId("delete-object")).toBeDisabled();

  // (a) Drag-to-move starting ON the locked object: it must be a no-op (no move
  // gesture, no mutation sent). Kept within the locked object's region so even a
  // stray marquee can only ever (re)select the locked object, never the editable.
  await dragOnPage(page, 0.3, 0.18, 0.42, 0.35, 10);

  // (b) Press Delete on the keyboard.
  await page.keyboard.press("Delete");

  // Give any (erroneous) optimistic mutation + server round-trip time to land.
  await page.waitForTimeout(800);

  // The client must FULLY prevent the write: no forbidden mutation was sent, so
  // the server never rejected and the reject-notice never appeared (no flicker).
  await expect(page.getByTestId("reject-notice")).toHaveCount(0);

  // The locked object must be UNCHANGED: still present, same points.
  const after = await getAnnotations(page, bandId, songId);
  const lockedAfter = after.objects.find((o) => o.uuid === "obj-locked");
  expect(lockedAfter, "locked object still present").toBeTruthy();
  expect(lockedAfter!.points).toEqual(lockedBefore.points);
  // Object count unchanged overall (nothing deleted, nothing duplicated).
  expect(after.objects.length).toBe(before.objects.length);
});

// ===========================================================================
// Locked selection shows a greyed lock near the bbox + disabled style controls.
// ===========================================================================
test("editor: selecting a locked object shows bbox-lock and disables controls", async ({
  page,
}) => {
  await openLockedSong(page);

  await page
    .getByTestId("layer-item")
    .filter({ hasText: "Locked layer" })
    .getByTestId("layer-row")
    .click();
  await page.getByTestId("tool-select").click();
  await openDrawer(page, "annotations");
  await page.getByTestId("annotation-item").filter({ hasText: "rect" }).first().click();

  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.getByTestId("bbox-lock")).toBeVisible();

  // The selection box reads read-only.
  await expect(page.getByTestId("selected-bbox").first()).toHaveClass(/readonly/);

  // Style controls (contextual .ctx pill for the selection) reflect the object but
  // are disabled (no restyle on locked). toBeDisabled checks the disabled property,
  // so it holds even for a per-type-hidden slot — no tab/tool switch needed here.
  await expect(page.getByTestId("style-color")).toBeDisabled();
  await expect(page.getByTestId("style-opacity")).toBeDisabled();
  await expect(page.getByTestId("style-width")).toBeDisabled();
  await expect(page.getByTestId("style-font")).toBeDisabled();
  await openDrawer(page, "layers");
  await expect(page.getByTestId("delete-object")).toBeDisabled();

  await page.screenshot({ path: "/tmp/ed3-locked-bbox.png", fullPage: true });
});

// ===========================================================================
// Cross-layer selection — clicking an object on a non-focused layer focuses that
// layer (annotation-list title changes) and selects it.
// ===========================================================================
test("editor: clicking an object focuses its layer (cross-layer)", async ({ page }) => {
  await openLockedSong(page);

  // Focus the editable layer first (its rect is in the lower-right region).
  await page
    .getByTestId("layer-item")
    .filter({ hasText: "Editable layer" })
    .getByTestId("layer-row")
    .click();
  await openDrawer(page, "annotations");
  await expect(page.getByTestId("annotation-list-title")).toContainText("Editable layer");

  // Now click the LOCKED object on the canvas (upper-left region). This belongs
  // to a different layer → it must focus that layer and select the object.
  await page.getByTestId("tool-select").click();
  await clickOnPage(page, 0.3, 0.25);

  await expect(page.getByTestId("annotation-list-title")).toContainText("Locked layer");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.getByTestId("bbox-lock")).toBeVisible();

  await page.screenshot({ path: "/tmp/ed3-crosslayer.png", fullPage: true });

  // And clicking the editable object switches focus back to its layer.
  await clickOnPage(page, 0.72, 0.25);
  await expect(page.getByTestId("annotation-list-title")).toContainText("Editable layer");
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.getByTestId("bbox-lock")).toHaveCount(0);
});

// ===========================================================================
// Restyle — selecting an editable object reflects its style in the controls;
// changing a control restyles the object live (setStyle), persisting via GET.
// ===========================================================================
test("editor: selecting an editable object reflects its style and restyles live", async ({
  page,
}) => {
  const { bandId, songId } = await openLockedSong(page);

  // Focus the editable layer and select its rect.
  await page
    .getByTestId("layer-item")
    .filter({ hasText: "Editable layer" })
    .getByTestId("layer-row")
    .click();
  await page.getByTestId("tool-select").click();
  await openDrawer(page, "annotations");
  await page.getByTestId("annotation-item").filter({ hasText: "rect" }).first().click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  // Editable selection: no lock cue, controls enabled.
  await expect(page.getByTestId("bbox-lock")).toHaveCount(0);
  await expect(page.getByTestId("style-color")).toBeEnabled();

  // Controls reflect the object's CURRENT style (#e11d48 → #E11D48, 100%).
  await closeDrawer(page); // the drawer overlays the ctx-bar's right-end ⋯ — close it to reach the popover
  await page.getByTestId("style-more").click(); // T33: hex readout lives in the ⋯ popover
  await expect(page.getByTestId("style-color-value")).toHaveText("#E11D48");
  await expect(page.getByTestId("style-opacity-value")).toHaveText("100%");

  await page.screenshot({ path: "/tmp/ed3-restyle.png", fullPage: true });

  // Set color + opacity + width via React's native setter so onChange fires.
  const setInput = (testid: string, value: string) =>
    page.getByTestId(testid).evaluate((el, v) => {
      const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), "value")!.set!;
      setter.call(el, v);
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.dispatchEvent(new Event("change", { bubbles: true }));
    }, value);

  await setInput("style-color", "#2563eb");
  await setInput("style-opacity", "0.5");
  // T84: the width control is an INDEX into a geometric stop table, not a raw width.
  // Pick a stop by index and assert the object persists that exact stop value.
  const widthStops = (await page.getByTestId("style-width").getAttribute("data-stops"))!
    .split(",")
    .map(Number);
  const widthIdx = 8;
  await setInput("style-width", String(widthIdx));

  // The object's persisted style must now reflect the new values.
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      const o = doc.objects.find((x) => x.uuid === "obj-editable");
      return o ? `${o.style.color}|${o.style.opacity}|${o.style.width}` : null;
    })
    .toBe(`#2563eb|0.5|${widthStops[widthIdx]}`);

  // Deselect → controls revert to the draw defaults (still enabled, default red).
  // T27 stage 3: the style row is the contextual .ctx pill, so pick a draw tool to
  // reveal it (arch Q3 "activate a tool first" — steps only; the assertion is same).
  await clickOnPage(page, 0.05, 0.92);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(0);
  await page.getByTestId("tool-rect").click();
  await expect(page.getByTestId("style-color")).toBeEnabled();
});

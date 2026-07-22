/**
 * Live annotation EDITOR — layer-access & editing-UX e2e.
 *
 * These specs target the editable-layer model and the selection / annotation-
 * list UX added on top of the realtime editor (see editor.spec.ts for the core
 * draw/realtime/delete coverage).
 *
 * Setup pattern: register, make a band + song, upload the PDF fixture, then
 * import (via the REST /annotations/import path, which is permissive and does
 * not enforce the live-edit write gate) a layer set that mimics the real-world
 * problem cases:
 *
 *   - A CONDUCTOR layer the current member does NOT own (ownerId = someone
 *     else), access "ro", mandatory: true. The member must never be able to
 *     draw into it (the live WS path rejects "forbidden"), and it must not be
 *     selectable as the active drawing layer.
 *
 * Screenshots land at /tmp/ed-active-layer.png, /tmp/ed-selection.png,
 * /tmp/ed-annlist.png.
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { openDrawer, closeDrawer, clearBand } from "./fullscreen-helpers";

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

async function getAnnotations(
  page: Page,
  bandId: string,
  songId: string,
): Promise<{ layers: { id: string }[]; objects: { uuid: string; type: string; layerId: string }[] }> {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as {
        layers: { id: string }[];
        objects: { uuid: string; type: string; layerId: string }[];
      };
    },
    [bandId, songId],
  );
}

/** A doc whose ONLY layer is a conductor read-only layer owned by someone else,
 *  mandatory. The current member owns nothing editable here. */
function conductorOnlyDoc(fileId: string) {
  return {
    layers: [
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
    ],
    objects: [],
  };
}

/** A doc that mimics a member with MULTIPLE editable layers: a mandatory RO layer
 *  they OWN (still editable — they own it), a shared RW layer, and a personal layer
 *  of their own. `me` is the current member's user id. Plus a CONDUCTOR-zone layer
 *  owned by SOMEONE ELSE (locked — conductor zone is role-governed, #3) to verify it
 *  is never editable by a non-conductor.
 *
 *  Note (#3): the owned mandatory layer lives in the PERSONAL zone, not conductor —
 *  conductor-zone write access now requires the conductor ROLE, not ownership, so an
 *  owned conductor layer would NOT be editable by this plain member. */
function ownerMultiLayerDoc(fileId: string, me: string) {
  return {
    layers: [
      {
        id: "layer-mine-conductor",
        fileId,
        name: "My conductor cues",
        ownerId: me,
        zone: "personal",
        order: 0,
        access: "ro",
        mandatory: true,
        roleTag: "conductor",
      },
      {
        id: "layer-shared",
        fileId,
        name: "Shared band notes",
        ownerId: "_shared_",
        zone: "shared",
        order: 1,
        access: "rw",
        mandatory: false,
        roleTag: "",
      },
      {
        id: "layer-mine-personal",
        fileId,
        name: "My personal notes",
        ownerId: me,
        zone: "personal",
        order: 2,
        access: "rw",
        mandatory: false,
        roleTag: "",
      },
      {
        id: "layer-other-conductor",
        fileId,
        name: "Other conductor cues",
        ownerId: "someone-else",
        zone: "conductor",
        order: 3,
        access: "ro",
        mandatory: true,
        roleTag: "conductor",
      },
    ],
    objects: [],
  };
}

// T59 — map Y by the PAGE box (the app's pointer→page frame) + scroll the target
// into the clear band, instead of the old `top + bandH*fy` band approximation
// (which only held while the .6rem-clobber pinned the page top under the chrome;
// T59's scroll overscan moves the page's rest position down). See the matching
// note in editor-active-layer.spec.ts.
async function dragOnPage(
  page: Page,
  fx: number,
  fy: number,
  tx: number,
  ty: number,
  steps = 8,
) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  let box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const loY = box.y + box.height * Math.min(fy, ty);
  const hiY = box.y + box.height * Math.max(fy, ty);
  if (loY < top + 8 || hiY > bottom - 8) {
    const mid = (top + bottom) / 2;
    const rangeMid = box.y + box.height * ((fy + ty) / 2);
    await page
      .getByTestId("viewer-scroll")
      .evaluate((s, dy) => s.scrollBy(0, dy), Math.round(rangeMid - mid));
    await page.waitForTimeout(60);
    box = (await pageEl.boundingBox())!;
  }
  const px = (f: number) => box.x + box.width * f;
  const py = (f: number) => box.y + box.height * f;
  await page.mouse.move(px(fx), py(fy));
  await page.mouse.down();
  await page.mouse.move(px(tx), py(ty), { steps });
  await page.mouse.up();
}

async function clickOnPage(page: Page, fx: number, fy: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  let box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const targetY = box.y + box.height * fy;
  if (targetY < top + 8 || targetY > bottom - 8) {
    const mid = (top + bottom) / 2;
    await page
      .getByTestId("viewer-scroll")
      .evaluate((s, dy) => s.scrollBy(0, dy), Math.round(targetY - mid));
    await page.waitForTimeout(60);
    box = (await pageEl.boundingBox())!;
  }
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
  // T27 stage 3: layer/annotation controls live in the on-demand drawer (closed by
  // default). This is a layer-management suite, so open it (Layers tab) as setup —
  // sanctioned mechanics (arch Q3). Tests needing the annotation list switch tabs.
  await openDrawer(page, "layers");
}

/** Shared setup: a song with a single conductor RO layer owned by someone else. */
async function openConductorOnlySong(page: Page): Promise<{ bandId: string; songId: string }> {
  await register(page, `lyr_${stamp()}`);
  const band = await createBandAndOpen(page, `LyrBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `LyrSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const res = await importDoc(page, band.id, songId, conductorOnlyDoc(fileId));
  expect(res.ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  return { bandId: band.id, songId };
}

/** Shared setup: a song where the current member is the band owner who owns a
 *  mandatory conductor layer AND has a shared RW layer (plus other layers). */
async function openOwnerMultiLayerSong(
  page: Page,
): Promise<{ bandId: string; songId: string; me: string }> {
  await register(page, `own_${stamp()}`);
  const band = await createBandAndOpen(page, `OwnBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `OwnSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);
  const res = await importDoc(page, band.id, songId, ownerMultiLayerDoc(fileId, me));
  expect(res.ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  return { bandId: band.id, songId, me };
}

// ===========================================================================
// Active-layer switching (reproduce-then-fix) — the owner of MULTIPLE editable
// layers (incl. a mandatory conductor layer they own) must be able to switch
// the active draw layer. Editability is purely owner===me || access==="rw".
// ===========================================================================
test("editor: owner of multiple editable layers can switch the active draw layer", async ({
  page,
}) => {
  const { bandId, songId } = await openOwnerMultiLayerSong(page);

  // The selector must list ALL layers I may edit: the conductor layer I OWN
  // (even though it's ro + mandatory), the shared RW layer, and my personal one.
  const selector = page.getByTestId("active-layer");
  const optionValues = await selector
    .locator("option")
    .evaluateAll((opts) => opts.map((o) => o.getAttribute("value") ?? ""));
  expect(optionValues).toContain("layer-mine-conductor");
  expect(optionValues).toContain("layer-shared");
  expect(optionValues).toContain("layer-mine-personal");
  // The conductor layer owned by SOMEONE ELSE must NOT be editable.
  expect(optionValues).not.toContain("layer-other-conductor");
  // At least two editable layers → switching is meaningful.
  expect(optionValues.filter((v) => v).length).toBeGreaterThanOrEqual(2);

  // Switch the active layer to my shared layer and confirm the indicator updates.
  await selector.selectOption("layer-shared");
  await expect(page.getByTestId("active-layer-indicator")).toContainText("Shared band notes");

  await page.screenshot({ path: "/tmp/ed2-layers.png", fullPage: true });

  // Draw after switching → the object persists to the SELECTED layer.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(1);
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      const rect = doc.objects.find((o) => o.type === "rect");
      return rect?.layerId ?? null;
    })
    .toBe("layer-shared");

  // Switch to the conductor layer I OWN and draw again → lands there too.
  await page.getByTestId("tool-select").click();
  await selector.selectOption("layer-mine-conductor");
  await expect(page.getByTestId("active-layer-indicator")).toContainText("My conductor cues");
  await page.getByTestId("tool-line").click();
  await dragOnPage(page, 0.2, 0.7, 0.6, 0.8);
  await expect.poll(() => objectCount(page)).toBe(2);
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      const line = doc.objects.find((o) => o.type === "line");
      return line?.layerId ?? null;
    })
    .toBe("layer-mine-conductor");
});

// ===========================================================================
// Bug 1 — wet ink must commit to an editable layer and persist, even when the
// song only has a non-writable (conductor RO) layer to begin with.
// ===========================================================================
test("editor: drawing persists even when only a non-writable layer exists", async ({ page }) => {
  const { bandId, songId } = await openConductorOnlySong(page);

  // Draw a rectangle WITHOUT first creating a layer — the editor must provision
  // a writable personal layer on demand so the object commits and persists.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);

  // Live count rises and stays (no forbidden rollback).
  await expect.poll(() => objectCount(page)).toBe(1);

  // Persisted: GET annotations has the rect, and it is NOT on the conductor layer.
  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      const rect = doc.objects.find((o) => o.type === "rect");
      return rect ? rect.layerId !== "layer-conductor" : false;
    })
    .toBeTruthy();
});

// ===========================================================================
// Bug 2 — the conductor RO layer must NOT be selectable as the active layer,
// and drawing must never land on / persist to it.
// ===========================================================================
test("editor: conductor RO layer is not selectable and cannot be edited", async ({ page }) => {
  const { bandId, songId } = await openConductorOnlySong(page);

  // The active-layer selector must NOT offer the conductor RO layer.
  const optionValues = await page
    .getByTestId("active-layer")
    .locator("option")
    .evaluateAll((opts) => opts.map((o) => o.getAttribute("value") ?? ""));
  expect(optionValues).not.toContain("layer-conductor");

  // Draw something; whatever it commits to, it must never be the conductor layer.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.2, 0.25, 0.6, 0.55);
  await expect.poll(() => objectCount(page)).toBe(1);

  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      return doc.objects.some((o) => o.layerId === "layer-conductor");
    })
    .toBeFalsy();

  // Active-layer indicator shows where ink goes (and not the conductor layer).
  await expect(page.getByTestId("active-layer-indicator")).toBeVisible();
  await expect(page.getByTestId("active-layer-indicator")).not.toContainText("Conductor cues");
});

// ===========================================================================
// Editable-layer filtering — RO layer is shown LOCKED in the panel, absent from
// the selector; the active layer is marked.
// ===========================================================================
test("editor: layers panel locks the RO layer; selector omits it", async ({ page }) => {
  await openConductorOnlySong(page);

  // Create a personal editable layer so we have a clear active selection.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  // The conductor layer item is marked locked.
  const conductorItem = page.getByTestId("layer-item").filter({ hasText: "Conductor cues" });
  await expect(conductorItem.getByTestId("layer-lock")).toBeVisible();

  // The selector lists only editable layers (no conductor layer).
  const labels = await page
    .getByTestId("active-layer")
    .locator("option")
    .allInnerTexts();
  expect(labels.join("|")).not.toContain("Conductor cues");

  await page.screenshot({ path: "/tmp/ed-active-layer.png", fullPage: true });
});

// ===========================================================================
// Rubber-band selection — drag a box over 2 objects, both selected (bboxes
// highlighted), Delete removes both.
// ===========================================================================
test("editor: rubber-band selects two objects and Delete removes both", async ({ page }) => {
  const { bandId, songId } = await openConductorOnlySong(page);

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  // Two rectangles in the upper region, side by side.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.1, 0.15, 0.35, 0.4);
  await expect.poll(() => objectCount(page)).toBe(1);
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.5, 0.15, 0.75, 0.4);
  await expect.poll(() => objectCount(page)).toBe(2);

  // Rubber-band: with Select, drag a box that encloses both.
  await page.getByTestId("tool-select").click();
  await dragOnPage(page, 0.05, 0.1, 0.85, 0.5, 12);

  // Both bounding boxes are highlighted.
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);

  await page.screenshot({ path: "/tmp/ed-selection.png", fullPage: true });

  // Delete via keyboard removes both.
  await page.keyboard.press("Delete");
  await expect.poll(() => objectCount(page)).toBe(0);

  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, bandId, songId);
      return doc.objects.length;
    })
    .toBe(0);
});

// ===========================================================================
// Annotation list — lists objects on the active layer; clicking selects one.
// ===========================================================================
test("editor: annotation list selects an object on the active layer", async ({ page }) => {
  await openConductorOnlySong(page);

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  // Draw a rect and a text so the list has labelled entries.
  await page.getByTestId("tool-rect").click();
  await dragOnPage(page, 0.15, 0.2, 0.45, 0.45);
  await expect.poll(() => objectCount(page)).toBe(1);

  page.once("dialog", (d) => d.accept("Cue!"));
  await page.getByTestId("tool-text").click();
  await clickOnPage(page, 0.6, 0.3);
  await expect.poll(() => objectCount(page)).toBe(2);

  // T27 stage 3: the annotation list lives on the drawer's Annotations tab.
  await openDrawer(page, "annotations");

  // The annotation list shows both objects (one labelled with the text).
  await expect(page.getByTestId("annotation-list")).toBeVisible();
  await expect(page.getByTestId("annotation-item")).toHaveCount(2);
  const textItem = page.getByTestId("annotation-item").filter({ hasText: "Cue!" });
  await expect(textItem).toHaveCount(1);

  // Clicking the text item selects it (Delete becomes enabled / bbox shows).
  await page.getByTestId("tool-select").click();
  await textItem.click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  // delete-object lives with layer management on the Layers tab; the selection
  // persists across tabs, so switch back to see it enabled.
  await openDrawer(page, "layers");
  await expect(page.getByTestId("delete-object")).toBeEnabled();

  await page.screenshot({ path: "/tmp/ed-annlist.png", fullPage: true });
});

// ===========================================================================
// Drawer tabs — T27 stage 3 (arch ruling 2026-07-10, "assertion-retirement set
// EXTENDED"): the old "Layers panel renders ABOVE the annotation list in the DOM"
// test asserted both panels co-visible + a compareDocumentPosition DOM order — a
// dead STACKED-SIDEBAR structure. The approved tabbed drawer (Q2/C-5) shows one tab
// at a time, so co-presence is impossible; those assertions are RETIRED per arch.
// Per arch's condition, the drawer's function stays tested: each tab is reachable.
// ===========================================================================
test("editor: both drawer tabs (Layers, Annotations) are reachable (T27 stage 3)", async ({ page }) => {
  await openConductorOnlySong(page);

  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");

  await openDrawer(page, "layers");
  await expect(page.getByTestId("layers-panel")).toBeVisible();

  await openDrawer(page, "annotations");
  await expect(page.getByTestId("annotation-list")).toBeVisible();
});

// ===========================================================================
// Scoped annotation list — clicking a layer row focuses it; the list shows ONLY
// that layer's objects and names it. A locked (RO) layer can be focused too:
// drawing is disabled with a hint, but its annotations stay browsable.
// ===========================================================================
test("editor: focused layer scopes the annotation list (editable and locked)", async ({
  page,
}) => {
  const { bandId, songId, me } = await openOwnerMultiLayerSong(page);

  // Seed one object per layer so each focus shows a distinct, single item.
  const style = { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.03 };
  const base = ownerMultiLayerDoc(await firstFileId(page, bandId, songId), me);
  const seeded = {
    layers: base.layers,
    objects: [
      {
        uuid: "obj-shared",
        layerId: "layer-shared",
        type: "rect",
        points: [{ x: 0.1, y: 0.1 }, { x: 0.3, y: 0.3 }],
        page: 0,
        text: "",
        style,
      },
      {
        uuid: "obj-personal",
        layerId: "layer-mine-personal",
        type: "line",
        points: [{ x: 0.4, y: 0.4 }, { x: 0.6, y: 0.6 }],
        page: 0,
        text: "",
        style,
      },
      {
        uuid: "obj-locked",
        layerId: "layer-other-conductor",
        type: "text",
        points: [{ x: 0.5, y: 0.2 }],
        page: 0,
        text: "LockedCue",
        style,
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, seeded)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  const row = (name: string) =>
    page.getByTestId("layer-item").filter({ hasText: name }).getByTestId("layer-row");

  // Focus the shared (editable) layer → list names it and shows only its rect.
  // (T27 stage 3: layer rows live on the Layers tab, the scoped list on the
  // Annotations tab — switch between them; focus state persists across tabs.)
  await row("Shared band notes").click();
  await openDrawer(page, "annotations");
  await expect(page.getByTestId("annotation-list-title")).toContainText("Shared band notes");
  await expect(page.getByTestId("annotation-item")).toHaveCount(1);
  await expect(page.getByTestId("annotation-item")).toContainText("rect");
  // Editable focus also drives the active draw layer + indicator.
  await openDrawer(page, "layers");
  await expect(page.getByTestId("active-layer-indicator")).toContainText("Shared band notes");
  await expect(page.getByTestId("active-layer")).toHaveValue("layer-shared");

  await page.screenshot({ path: "/tmp/ed2-scoped-list.png", fullPage: true });

  // Focus my personal layer → list re-scopes to only its line.
  await row("My personal notes").click();
  await openDrawer(page, "annotations");
  await expect(page.getByTestId("annotation-list-title")).toContainText("My personal notes");
  await expect(page.getByTestId("annotation-item")).toHaveCount(1);
  await expect(page.getByTestId("annotation-item")).toContainText("line");

  // Focus the LOCKED (someone else's conductor) layer → drawing disabled with a
  // hint, but its annotation is still listed and selectable.
  await openDrawer(page, "layers");
  await row("Other conductor cues").click();
  await openDrawer(page, "annotations");
  await expect(page.getByTestId("annotation-list-title")).toContainText("Other conductor cues");
  await expect(page.getByTestId("annotation-list-locked")).toContainText("read-only layer");
  await expect(page.getByTestId("draw-locked-hint")).toBeVisible();
  // Draw tools are disabled while a locked layer is focused.
  await expect(page.getByTestId("tool-rect")).toBeDisabled();
  // The locked layer's annotation is browsable + selectable from the list.
  const lockedItem = page.getByTestId("annotation-item").filter({ hasText: "LockedCue" });
  await expect(lockedItem).toHaveCount(1);
  await page.getByTestId("tool-select").click();
  await lockedItem.click();
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // No write happened to the locked layer.
  const after = await getAnnotations(page, bandId, songId);
  expect(after.objects.filter((o) => o.layerId === "layer-other-conductor")).toHaveLength(1);
});

// ===========================================================================
// Style value readouts — each control shows its current value (hex / % / number)
// and updates live as the control changes.
// ===========================================================================
test("editor: style controls show live value readouts", async ({ page }) => {
  await openConductorOnlySong(page);

  // T27 stage 3: the style row is contextual (shown only while a draw tool is
  // active or an object is selected). This song has only a conductor RO layer, so
  // create an editable layer first, then activate a draw tool — the style row
  // renders. Sanctioned flow change (arch ruling 2026-07-10, Q3); assertions same.
  await page.getByTestId("new-layer").click();
  await expect(page.getByTestId("active-layer")).not.toHaveValue("");
  await page.getByTestId("tool-rect").click();

  // Readouts are present with the default style values. The style row is now
  // per-type (a shape tool shows width, a text tool shows font size — never both
  // at once), so assert width under the rect tool, then switch to the text tool
  // for the font readout. Steps change; the assertions themselves are unchanged.
  await closeDrawer(page); // the drawer overlays the ctx-bar's right-end ⋯ — close it to reach the popover
  await page.getByTestId("style-more").click(); // T33: hex readout lives in the ⋯ popover
  await expect(page.getByTestId("style-color-value")).toBeVisible();
  await expect(page.getByTestId("style-opacity-value")).toHaveText(/%$/);
  await expect(page.getByTestId("style-width-value")).toBeVisible();
  await page.getByTestId("tool-text").click();
  await expect(page.getByTestId("style-font-value")).toBeVisible();
  await page.getByTestId("tool-rect").click();

  // Default opacity is 1 → 100%.
  await expect(page.getByTestId("style-opacity-value")).toHaveText("100%");

  // Picking a swatch updates the color hex readout live (#e11d48 → #E11D48).
  await page.getByLabel("Color #2563eb").click();
  await page.getByTestId("style-more").click(); // reopen (tool switches above dismissed it)
  await expect(page.getByTestId("style-color-value")).toHaveText("#2563EB");

  // Set the inputs via React's native value setter so the controlled onChange
  // fires, then confirm the readouts follow live. (Runs in the browser; uses
  // the element's own prototype to avoid naming DOM globals in node-typed e2e.)
  const setInput = (testid: string, value: string) =>
    page.getByTestId(testid).evaluate((el, v) => {
      const setter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(el), "value")!.set!;
      setter.call(el, v);
      el.dispatchEvent(new Event("input", { bubbles: true }));
      el.dispatchEvent(new Event("change", { bubbles: true }));
    }, value);

  await setInput("style-color", "#e53935");
  await expect(page.getByTestId("style-color-value")).toHaveText("#E53935");

  // Drag opacity down and confirm the % readout changes off 100%.
  await setInput("style-opacity", "0.5");
  await expect(page.getByTestId("style-opacity-value")).toHaveText("50%");

  await page.screenshot({ path: "/tmp/ed2-readouts.png", fullPage: true });
});

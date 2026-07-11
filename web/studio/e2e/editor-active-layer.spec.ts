/**
 * BUG #2 — editing must apply ONLY to objects on the ACTIVE editing layer.
 *
 * Real-world case: Marie OWNS a conductor layer C (so it is editable for her),
 * but her ACTIVE layer is a different editable layer. Selecting one of her own
 * conductor annotations on C and dragging / pressing Delete must be a NO-OP:
 * editing is scoped to the active layer. "Edit this layer" then activates C and
 * makes it editable.
 *
 * Reproduce-first: with C focused-but-not-active, drag + Delete the object →
 * assert it is unchanged (GET annotations). Then click "Edit this layer" and
 * confirm the object becomes movable (its layer is now active).
 *
 * Screenshot: /tmp/ed4-active-edit.png.
 */
import { test, expect, type Page } from "@playwright/test";
import { clearBand, openDrawer } from "./fullscreen-helpers";
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

async function clickOnPage(page: Page, fx: number, fy: number) {
  const pageEl = page.getByTestId("pdf-page").first();
  await pageEl.scrollIntoViewIfNeeded();
  const box = (await pageEl.boundingBox())!;
  const { top, bottom } = await clearBand(page);
  const bandH = Math.max(0, bottom - top) * 0.9;
  await page.mouse.click(box.x + box.width * fx, top + bandH * fy);
}

async function dragOnPage(page: Page, fx: number, fy: number, tx: number, ty: number, steps = 10) {
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

const STYLE = { color: "#e11d48", opacity: 1, width: 0.004, fontSize: 0.03 };

/** Two layers, BOTH editable for `me`: a personal RW layer (A) and a personal RO
 *  layer I own (C, ro+mandatory but editable since I own it). C holds a rect.
 *  (C is a personal layer, not conductor: conductor-zone write now needs the
 *  conductor ROLE not ownership, #3 — so an owned conductor layer wouldn't be
 *  editable by this plain member; the active-layer-scoping rule under test is
 *  orthogonal to the zone.) */
function twoEditableLayersDoc(fileId: string, me: string) {
  return {
    layers: [
      {
        id: "layer-a",
        fileId,
        name: "My personal A",
        ownerId: me,
        zone: "personal",
        order: 0,
        access: "rw",
        mandatory: false,
        roleTag: "",
      },
      {
        id: "layer-c",
        fileId,
        name: "My conductor C",
        ownerId: me,
        zone: "personal",
        order: 1,
        access: "ro",
        mandatory: true,
        roleTag: "conductor",
      },
    ],
    objects: [
      {
        uuid: "obj-on-c",
        layerId: "layer-c",
        type: "rect",
        points: [
          { x: 0.15, y: 0.12 },
          { x: 0.45, y: 0.4 },
        ],
        page: 0,
        text: "",
        style: STYLE,
      },
    ],
  };
}

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer controls live in the on-demand drawer — open it (Layers).
  await openDrawer(page, "layers");
}

test("editor: an object on a non-active (but owned) layer is inspect-only until 'Edit this layer'", async ({
  page,
}) => {
  await register(page, `act_${stamp()}`);
  const band = await createBandAndOpen(page, `ActBand ${stamp()}`);
  const songId = await createSongAndOpen(page, `ActSong ${stamp()}`);
  await uploadPdf(page);
  const fileId = await firstFileId(page, band.id, songId);
  const me = await myUserId(page);
  expect((await importDoc(page, band.id, songId, twoEditableLayersDoc(fileId, me))).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);

  const before = await getAnnotations(page, band.id, songId);
  const cBefore = before.objects.find((o) => o.uuid === "obj-on-c")!;
  expect(cBefore).toBeTruthy();

  // Make layer A the ACTIVE edit layer (C is owned/editable but NOT active).
  await page.getByTestId("active-layer").selectOption("layer-a");
  await expect(page.getByTestId("active-layer-indicator")).toContainText("My personal A");

  // Select the object on C by clicking it (focus-only, does NOT activate C).
  await page.getByTestId("tool-select").click();
  await clickOnPage(page, 0.3, 0.25); // click the C rect
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);

  // It is inspect-only: Delete disabled, the "edit this layer" CTA + hint show.
  await expect(page.getByTestId("delete-object")).toBeDisabled();
  await expect(page.getByTestId("edit-this-layer")).toBeVisible();
  await expect(page.getByTestId("edit-layer-hint")).toBeVisible();
  // No resize handles while inactive (not editable-now).
  await expect(page.getByTestId("resize-handle")).toHaveCount(0);

  await page.screenshot({ path: "/tmp/ed4-active-edit.png", fullPage: true });

  // Try to MOVE it by dragging from its center, and press Delete.
  await dragOnPage(page, 0.3, 0.25, 0.55, 0.5, 12);
  await page.keyboard.press("Delete");
  await page.waitForTimeout(700);

  // No write happened — the object on C is byte-for-byte unchanged + still there.
  await expect(page.getByTestId("reject-notice")).toHaveCount(0);
  const mid = await getAnnotations(page, band.id, songId);
  const cMid = mid.objects.find((o) => o.uuid === "obj-on-c");
  expect(cMid, "object on C still present").toBeTruthy();
  expect(cMid!.points).toEqual(cBefore.points);
  expect(mid.objects.length).toBe(before.objects.length);

  // Now activate C via "Edit this layer" → it becomes the active edit target.
  await page.getByTestId("edit-this-layer").click();
  await expect(page.getByTestId("active-layer-indicator")).toContainText("My conductor C");
  await expect(page.getByTestId("active-layer")).toHaveValue("layer-c");

  // Re-select the object on C (still in its original place) and move it. Now it
  // IS on the active layer, so the move sticks.
  await clickOnPage(page, 0.3, 0.25);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await expect(page.getByTestId("delete-object")).toBeEnabled();
  await dragOnPage(page, 0.3, 0.25, 0.5, 0.5, 12);

  await expect
    .poll(async () => {
      const doc = await getAnnotations(page, band.id, songId);
      const o = doc.objects.find((x) => x.uuid === "obj-on-c");
      return o ? JSON.stringify(o.points) !== JSON.stringify(cBefore.points) : false;
    })
    .toBeTruthy();
});

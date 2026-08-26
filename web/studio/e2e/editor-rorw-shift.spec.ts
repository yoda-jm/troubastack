/**
 * BUG (reproduce-first): focusing a READ-ONLY layer shifts the editor layout.
 *
 * Same class as the style-toolbar stability fix (ec8dd3f) and the toolbar/annotation
 * hint reservations (772be41): content that appears in only ONE state changes a
 * footprint. The confirmed offender (T13) is the Layers-panel row pills — the
 * `drawing` (active) and `viewing` (focused) pills move between rows as focus/active
 * changes; when mounted conditionally, focusing the RO layer added a pill to its
 * (longer-named) row, which wrapped onto an extra line under CI's wider fallback
 * font, growing the sidebar and — since it stacks above the viewer at narrow widths
 * — pushing `pdf-page` down ~27px. Fixed by always mounting those pills with space
 * reserved, so every row's footprint is focus-independent.
 *
 * Reproduce-first: seed a song with BOTH a RW personal layer (mine, editable)
 * and a RO shared layer (someone else's, with an object). With the Select tool,
 * measure (a) `editor-toolbar` boundingBox height and (b) the `pdf-page` top
 * offset at a fixed scroll position, in two states:
 *   (a) RW layer active   (b) RO layer focused (click its object/row)
 * Assert both measurements are EQUAL across the two states.
 *
 * Screenshots: /tmp/ro-rw-rw.png (RW active), /tmp/ro-rw-ro.png (RO focused).
 */
import { test, expect, type Page } from "@playwright/test";
import { openDrawer } from "./fullscreen-helpers";
import { stamp, register, createBandAndOpen, createSongAndOpen, uploadPdf } from "./setup-helpers";

async function myUserId(page: Page): Promise<string> {
  return page.evaluate(async (): Promise<string> => {
    const r = await fetch("/api/me", { credentials: "include" });
    const j = (await r.json()) as { user: { id: string } };
    return j.user.id;
  });
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

async function openEditorReady(page: Page) {
  await expect(page.getByTestId("pdf-page").first()).toBeVisible();
  await expect(page.getByTestId("edit-canvas").first()).toBeVisible();
  await expect(page.getByTestId("conn-status")).toHaveText("live", { timeout: 10_000 });
  // T27 stage 3: layer rows live in the on-demand drawer — open it (Layers).
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

/** Toolbar height + viewer top offset at a FIXED scroll position (top). */
async function measure(page: Page): Promise<{ pageTop: number }> {
  // Pin scroll to the top so the pdf-page's viewport-relative top is comparable
  // across states. (globalThis avoids a DOM-lib dep in the e2e tsconfig.)
  await page.evaluate(() =>
    (globalThis as unknown as { scrollTo: (x: number, y: number) => void }).scrollTo(0, 0),
  );
  // T27 stage 3 (arch ruling 2026-07-10): the `editor-toolbar` height comparison is
  // RETIRED — the tool cluster is `display:contents` in the floating pill (no box),
  // so the metric is meaningless. `pageTop` (T13's real invariant — the score must
  // not move when a read-only layer is focused) STAYS asserted, per arch.
  const pg = (await page.getByTestId("pdf-page").first().boundingBox())!;
  return { pageTop: pg.y };
}

test("editor: focusing a read-only layer does NOT shift the layout (RO vs RW footprint identical)", async ({
  page,
}) => {
  // Fixed in T13 (was CI-headless-only): the shift was the Layers-panel row pills.
  // The `drawing`/`viewing` pills are the only per-row content that moves with
  // focus/active state; when they were mounted conditionally, focusing the RO layer
  // put an extra pill on its (longer-named) row, tipping its pills onto a wrapped
  // line under CI's wider fallback font — growing the panel ~27px and pushing the
  // viewer down. Both pills are now always mounted with space reserved (SidePanels
  // + .layer-pill-off), so every row's footprint is focus-independent.

  // A narrowed viewport (sidebar stacks above the viewer; tool-palette near its
  // wrap boundary) is where the shift showed — reproduced locally by forcing a
  // wide font. At default width there is slack that hides the regression.
  await page.setViewportSize({ width: 560, height: 900 });

  const { bandId, songId, fileId, me } = await setup(page, "RoRw");
  const doc = {
    layers: [
      // RW personal layer owned by ME → editable; will be the active edit target.
      {
        id: "layer-mine",
        fileId,
        name: "My notes",
        ownerId: me,
        zone: "personal",
        order: 0,
        access: "rw",
        mandatory: false,
        roleTag: "",
      },
      // RO shared layer owned by SOMEONE ELSE → visible but not editable by me.
      {
        id: "layer-ro",
        fileId,
        name: "Shared (locked)",
        ownerId: "someone-else",
        zone: "shared",
        order: 1,
        access: "ro",
        mandatory: false,
        roleTag: "",
      },
    ],
    objects: [
      // An object on MY (RW) layer.
      {
        uuid: "mine-box",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.2, y: 0.12 }, { x: 0.45, y: 0.3 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      // An object on the RO layer (so focusing it is meaningful / clickable).
      {
        uuid: "ro-box",
        layerId: "layer-ro",
        type: "rect",
        points: [{ x: 0.55, y: 0.12 }, { x: 0.8, y: 0.3 }],
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

  // ---- State (a): the RW layer is the active/focused target. ----
  await page.getByTestId("layer-row").filter({ hasText: "My notes" }).click();
  // The RW layer is active/focused → the locked hint is NOT shown (its space is
  // reserved by the fix, but it is not visible).
  await expect(page.getByTestId("draw-locked-hint")).toBeHidden();
  const rw = await measure(page);
  await page.screenshot({ path: "/tmp/ro-rw-rw.png", fullPage: true });

  // ---- State (b): focus the RO layer (click its row). ----
  await page.getByTestId("layer-row").filter({ hasText: "Shared (locked)" }).click();
  // Focusing the RO layer surfaces the locked hint (the shift trigger).
  await expect(page.getByTestId("draw-locked-hint")).toBeVisible();
  const ro = await measure(page);
  await page.screenshot({ path: "/tmp/ro-rw-ro.png", fullPage: true });

  // The score position must be IDENTICAL across the two states — focusing a
  // read-only layer must not move anything (T13's real invariant; the retired
  // toolbar-height comparison is gone per the 2026-07-10 arch ruling).
  expect(ro.pageTop).toBe(rw.pageTop);
});

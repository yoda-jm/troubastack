/**
 * Editor UX fixes cycle:
 *   #1+#2 — stable style toolbar (fixed footprint) + per-type relevant controls.
 *   #3 — move requires a STRONG/containment hit (not the generous select pad).
 *   #4 — multi-select: no resize handles, disabled style controls, move-all.
 *
 * Reuses the import + GET annotation pattern from editor-pick.spec.ts to assert
 * positions deterministically.
 *
 * Screenshots: /tmp/ui-toolbar-text.png, /tmp/ui-toolbar-shape.png,
 * /tmp/ui-multimove.png.
 */
import { test, expect, type Page } from "@playwright/test";
import { scrollFracIntoBand } from "./fullscreen-helpers";
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

async function getAnnotations(page: Page, bandId: string, songId: string) {
  return page.evaluate(
    async ([b, s]) => {
      const r = await fetch(`/api/bands/${b}/songs/${s}/annotations`, { credentials: "include" });
      return (await r.json()) as {
        objects: { uuid: string; points: { x: number; y: number }[] }[];
      };
    },
    [bandId, songId],
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

// Scroll the given page-Y fraction(s) into the visible band before returning the box
// (the full-viewport editor's page can exceed the scroll / sit under the floating
// chrome — T27 stage 3).
async function pageBox(page: Page, ...pys: number[]) {
  return scrollFracIntoBand(page, ...pys);
}

/** Drag from a TRUE page-relative fraction to another (single linear drag). */
async function dragPageFrac(page: Page, x0: number, y0: number, x1: number, y1: number, steps = 12) {
  const box = await pageBox(page, y0, y1);
  await page.mouse.move(box.x + box.width * x0, box.y + box.height * y0);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * x1, box.y + box.height * y1, { steps });
  await page.mouse.up();
}

async function clickPageFrac(page: Page, px: number, py: number) {
  const box = await pageBox(page, py);
  await page.mouse.click(box.x + box.width * px, box.y + box.height * py);
}

// ===========================================================================
// #3 — move requires a STRONG/containment hit (not the generous select pad).
// ===========================================================================
test("#3 move: press+drag OUTSIDE a thin line (in the old pad) does NOT move it", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "MoveDist");
  // A thin horizontal line at y=0.4 spanning x∈[0.3,0.7] on my own active layer.
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "line",
        layerId: "layer-mine",
        type: "line",
        points: [{ x: 0.3, y: 0.4 }, { x: 0.7, y: 0.4 }],
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

  const before = await getAnnotations(page, bandId, songId);
  const lineBefore = before.objects.find((o) => o.uuid === "line")!;

  // Press+drag starting in the WEAK proximity pad just OFF the segment (a couple
  // % below the thin line at y=0.4 — outside the body, inside the old select
  // pad). This still SELECTS the line, but must NOT start a move (points stay).
  // 0.01 below the line at y=0.4: inside the line's WEAK select zone (its pixel-based
  // proximity pad ≈ 0.0145 page-frac on the larger fullscreen page) but OUTSIDE the
  // STRONG move zone (~0.0048) — so it SELECTS but must NOT move. The old 0.02 gap
  // sat beyond the pad once the fullscreen page grew (the pad is pixel-based). Same
  // "weak pad selects, no move" intent; only the input coordinate is recalibrated.
  await clickPageFrac(page, 0.5, 0.41); // confirm the pad still selects it
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  await dragPageFrac(page, 0.5, 0.41, 0.5, 0.6);

  const afterPad = await getAnnotations(page, bandId, songId);
  const lineAfterPad = afterPad.objects.find((o) => o.uuid === "line")!;
  expect(lineAfterPad.points).toEqual(lineBefore.points);

  // Press+drag starting ON the body (the segment at y=0.4) → it DOES move.
  await dragPageFrac(page, 0.5, 0.4, 0.6, 0.55);
  await expect
    .poll(async () => {
      const d = await getAnnotations(page, bandId, songId);
      const l = d.objects.find((o) => o.uuid === "line")!;
      return l.points[0].y !== lineBefore.points[0].y;
    })
    .toBe(true);
});

// ===========================================================================
// #4 — multi-select: no resize handles, disabled controls, move-all.
// ===========================================================================
test("#4 multi-select: marquee two objects → no handles, controls disabled, move both", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "MultiMove");
  // Two rects on the active layer, side by side in the upper region.
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "a",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.15, y: 0.15 }, { x: 0.3, y: 0.3 }],
        page: 0,
        text: "",
        style: STYLE,
      },
      {
        uuid: "b",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.4, y: 0.15 }, { x: 0.55, y: 0.3 }],
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

  // Marquee around BOTH rects (drag from above-left of A to below-right of B).
  await dragPageFrac(page, 0.1, 0.08, 0.62, 0.36);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(2);

  // No resize handles while multiple are selected.
  await expect(page.getByTestId("resize-handle")).toHaveCount(0);

  // Style controls are disabled for a heterogeneous/multi selection.
  await expect(page.getByTestId("style-color")).toBeDisabled();
  await expect(page.getByTestId("style-opacity")).toBeDisabled();

  await page.screenshot({ path: "/tmp/ui-multimove.png", fullPage: true });

  const before = await getAnnotations(page, bandId, songId);
  const aBefore = before.objects.find((o) => o.uuid === "a")!;
  const bBefore = before.objects.find((o) => o.uuid === "b")!;

  // Press INSIDE the selection (on rect A's body) and drag → BOTH move by the
  // same delta.
  await dragPageFrac(page, 0.22, 0.22, 0.37, 0.42);

  await expect
    .poll(async () => {
      const d = await getAnnotations(page, bandId, songId);
      const a = d.objects.find((o) => o.uuid === "a")!;
      const b = d.objects.find((o) => o.uuid === "b")!;
      const dax = a.points[0].x - aBefore.points[0].x;
      const day = a.points[0].y - aBefore.points[0].y;
      const dbx = b.points[0].x - bBefore.points[0].x;
      const dby = b.points[0].y - bBefore.points[0].y;
      const moved = Math.abs(dax) > 0.05 && Math.abs(day) > 0.05;
      const sameDelta = Math.abs(dax - dbx) < 0.005 && Math.abs(day - dby) < 0.005;
      return moved && sameDelta ? "ok" : `da(${dax.toFixed(3)},${day.toFixed(3)}) db(${dbx.toFixed(3)},${dby.toFixed(3)})`;
    })
    .toBe("ok");
});

// ===========================================================================
// #1+#2 — stable toolbar footprint + per-type relevant controls.
// ===========================================================================
test("#1+#2 toolbar: stable footprint across none/text/shape, per-type controls", async ({
  page,
}) => {
  const { bandId, songId, fileId, me } = await setup(page, "Toolbar");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      {
        uuid: "txt",
        layerId: "layer-mine",
        type: "text",
        points: [{ x: 0.3, y: 0.3 }],
        page: 0,
        text: "HELLO",
        style: STYLE,
      },
      {
        uuid: "box",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.5, y: 0.12 }, { x: 0.7, y: 0.28 }],
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

  // T27 stage 3 (arch ruling 2026-07-10, reviews.md "extend the assertion-retirement
  // set"): the stable-toolbar-FOOTPRINT half (#1) is RETIRED here — the style row is
  // now the contextual `.ctx` pill (absent in the neutral state, floating over the
  // canvas), so the "identical footprint across none/text/shape" mechanism no longer
  // applies; the real invariant (the score never shifts) is guarded LIVE by
  // editor-zeroshift. The per-type control relevance (#2) below is unchanged.

  // Select the text object.
  await clickPageFrac(page, 0.31, 0.31);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  // For TEXT: size visible+usable, width control NOT usable.
  await expect(page.getByTestId("style-font")).toBeVisible();
  await expect(page.getByTestId("style-width")).toBeHidden();
  await page.screenshot({ path: "/tmp/ui-toolbar-text.png", fullPage: true });

  // Select the rect.
  await clickPageFrac(page, 0.6, 0.2);
  await expect(page.getByTestId("selected-bbox")).toHaveCount(1);
  // For SHAPE: width visible+usable, size control NOT usable; shape controls show.
  await expect(page.getByTestId("style-width")).toBeVisible();
  await expect(page.getByTestId("style-font")).toBeHidden();
  await page.getByTestId("style-more").click(); // T33: fill lives in the ⋯ popover now
  await expect(page.getByTestId("style-fill")).toBeVisible();
  await page.screenshot({ path: "/tmp/ui-toolbar-shape.png", fullPage: true });
});

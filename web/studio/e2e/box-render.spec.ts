/**
 * Filled-shape ("Box") rendering checks (temporary; for the inset-stroke +
 * single-opacity-composite fixes in @troubastack/ink). Seeds one page with a
 * fill+stroke Box (opacity ~0.4, thick width), an Outline rect, and a Highlight
 * rect, then screenshots and asserts the Box's rendered ink stays within bbox.
 *
 * Screenshot: /tmp/box-shot.png (renamed to before/after by the runner).
 */
import { test, expect, type Page } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { stamp, register, createBandAndOpen, createSongAndOpen } from "./setup-helpers";

const PDF_PATH = fileURLToPath(new URL("./fixtures/sample.pdf", import.meta.url));

async function myUserId(page: Page): Promise<string> {
  return page.evaluate(async (): Promise<string> => {
    const r = await fetch("/api/me", { credentials: "include" });
    const j = (await r.json()) as { user: { id: string } };
    return j.user.id;
  });
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

test("box render: thick fill+stroke box, outline rect, highlight rect", async ({ page }) => {
  const { bandId, songId, fileId, me } = await setup(page, "BoxRender");
  const doc = {
    layers: [personalLayer(fileId, me)],
    objects: [
      // Box: fill + stroke, translucent (0.4), THICK border (4% of page width).
      {
        uuid: "box",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.15, y: 0.12 }, { x: 0.55, y: 0.42 }],
        page: 0,
        text: "",
        style: { color: "#2563eb", opacity: 0.4, width: 0.04, fontSize: 0.04, fill: true, stroke: true, blend: "normal" },
      },
      // Outline-only rect (legacy look: stroke only).
      {
        uuid: "outline",
        layerId: "layer-mine",
        type: "rect",
        points: [{ x: 0.6, y: 0.12 }, { x: 0.9, y: 0.42 }],
        page: 0,
        text: "",
        style: { color: "#16a34a", opacity: 1, width: 0.006, fontSize: 0.04, stroke: true },
      },
      // Highlight rect (fill + multiply, no stroke).
      {
        uuid: "hl",
        layerId: "layer-mine",
        type: "highlight",
        points: [{ x: 0.15, y: 0.55 }, { x: 0.9, y: 0.7 }],
        page: 0,
        text: "",
        style: { color: "#facc15", opacity: 0.5, width: 0.004, fontSize: 0.04 },
      },
    ],
  };
  expect((await importDoc(page, bandId, songId, doc)).ok).toBeTruthy();
  await page.reload();
  await openEditorReady(page);
  // Poll until the Box's ink has actually rendered inside its bbox, rather than sleeping for it.
  // (Sampling a sub-region safely inside the 0.15–0.55 × 0.12–0.42 bbox.)
  await expect
    .poll(() =>
      page.evaluate(() => {
        const c = (globalThis as unknown as { document: { querySelector: (s: string) => any } }).document.querySelector(
          '[data-testid="annotation-overlay"]',
        );
        if (!c) return 0;
        const ctx = c.getContext("2d");
        const W = c.width;
        const H = c.height;
        const d = ctx.getImageData(Math.round(0.22 * W), Math.round(0.18 * H), Math.round(0.18 * W), Math.round(0.14 * H)).data;
        let painted = 0;
        for (let i = 0; i < d.length; i += 4) if (d[i + 3] > 20) painted++;
        return painted;
      }),
    )
    .toBeGreaterThan(0);

  // --- Assert: the Box's rendered ink stays WITHIN its [0,1] bbox. ---------
  // Read the overlay canvas pixels; for the Box's bbox + a guard margin OUTSIDE
  // it, no painted (non-transparent) pixel from the box may appear beyond the
  // bbox. The blue box is the only thing in the top-left region.
  const leaked = await page.evaluate(() => {
    // Bare DOM globals are typed via the Node tsconfig's `any` (no DOM lib here);
    // keep this loose so e2e type-checks without pulling DOM types into node cfg.
    const doc2 = (globalThis as unknown as { document: { querySelector: (s: string) => any } }).document;
    const canvas = doc2.querySelector('[data-testid="annotation-overlay"]');
    const ctx = canvas.getContext("2d");
    const W = canvas.width;
    const H = canvas.height;
    const img = ctx.getImageData(0, 0, W, H).data;
    // Box bbox in canvas px (geometry matches the seed; overlay canvas == page).
    const bx0 = Math.round(0.15 * W);
    const by0 = Math.round(0.12 * H);
    const bx1 = Math.round(0.55 * W);
    const by1 = Math.round(0.42 * H);
    const margin = Math.round(0.06 * W); // > half the 4% width so any real leak shows
    // Ignore a thin antialiasing fringe right at the bbox boundary (±2 device px):
    // the border is inset, so any ink beyond this fringe is a genuine leak.
    const aa = 2;
    let leakedPixels = 0;
    // Scan a ring OUTSIDE the (bbox + AA fringe), within margin, for blue painted px.
    for (let y = Math.max(0, by0 - margin); y < Math.min(H, by1 + margin); y++) {
      for (let x = Math.max(0, bx0 - margin); x < Math.min(W, bx1 + margin); x++) {
        const insideGuard = x >= bx0 - aa && x < bx1 + aa && y >= by0 - aa && y < by1 + aa;
        if (insideGuard) continue;
        const i = (y * W + x) * 4;
        const a = img[i + 3];
        const b = img[i + 2];
        const rr = img[i];
        // Blue-dominant painted pixel = box ink leaking outside the bbox.
        if (a > 20 && b > 80 && b > rr + 30) leakedPixels++;
      }
    }
    return { leakedPixels };
  });
  await page.screenshot({ path: "/tmp/box-shot.png", fullPage: true });
  // Allow a tiny tolerance for antialiasing on the bbox edge.
  expect(leaked.leakedPixels).toBeLessThan(50);
});

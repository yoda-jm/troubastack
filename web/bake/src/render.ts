/**
 * The bake overlay renderer — the real half of invariant I8.
 *
 * For each page and each layer, this draws that layer's live annotation objects
 * through @troubastack/ink (`renderObjects`) onto a TRANSPARENT Skia canvas and
 * encodes a PNG. It uses the SAME renderer studio's dry layer uses; the only
 * bake-specific glue is the canvas backend (@napi-rs/canvas) and pinning ink's
 * text font to a bundled file so text is deterministic across renderers (I8 parity).
 *
 * Scope (B01): overlays ONLY. PDF page rasters are core's job (B02); this module
 * never produces or composites them — it just needs each page's pixel size.
 */

import { createHash } from "node:crypto";
import { fileURLToPath } from "node:url";
import { createCanvas, GlobalFonts } from "@napi-rs/canvas";
import {
  renderObjects,
  setTextFontFamily,
  type Ctx2D,
  type InkObject,
  type PageRect,
} from "@troubastack/ink";

// --- Bundled text font (the bake contract) --------------------------------
//
// Text objects render with Roboto Regular, bundled at assets/Roboto-Regular.ttf
// (Apache-2.0). We register it under a PRIVATE family name so it can never
// collide with a differently-hinted "Roboto" that happens to be installed on the
// host, and pin ink to that family. The I8 parity harness loads the exact same
// file in the browser under the same family — identical glyph outlines on both
// Skia builds. This family name is part of the bake contract: any renderer that
// must match bake's text pixels loads THIS file under THIS name.
export const BAKE_TEXT_FONT_FAMILY = "TroubaBakeText";
export const BAKE_TEXT_FONT_FILE = fileURLToPath(
  new URL("../assets/Roboto-Regular.ttf", import.meta.url),
);

let fontReady = false;
function ensureFont(): void {
  if (fontReady) return;
  // registerFromPath(path, family) aliases the file to our private family.
  GlobalFonts.registerFromPath(BAKE_TEXT_FONT_FILE, BAKE_TEXT_FONT_FAMILY);
  setTextFontFamily(BAKE_TEXT_FONT_FAMILY);
  fontReady = true;
}

// --- OffscreenCanvas shim (I8 parity for fill+stroke shapes) ---------------
//
// ink's shape painter composites a fill+stroke "box" on an offscreen canvas so
// the overlapping rim isn't double-blended; it only takes that path when a global
// `OffscreenCanvas` (or `document`) exists — true in the browser, false in bare
// Node, where it would fall back to the double-blend path and DIVERGE from studio.
// Give Node an OffscreenCanvas backed by @napi-rs/canvas so ink takes the exact
// same composite path server-side (a napi Canvas is a valid drawImage source and
// exposes get/setTransform, so ink's offscreen logic works unchanged).
function ensureOffscreen(): void {
  const g = globalThis as { OffscreenCanvas?: unknown };
  if (g.OffscreenCanvas) return;
  g.OffscreenCanvas = class {
    constructor(w: number, h: number) {
      return createCanvas(Math.max(1, w), Math.max(1, h));
    }
  };
}

// --- Input shapes ----------------------------------------------------------
//
// `doc` is the annotations JSON exactly as
//   GET /api/bands/{b}/songs/{s}/annotations
// returns it (core/internal/httpapi/annotations.go) — the same shape studio
// consumes. No new format. `objects[].points` may carry optional `pressure`
// (ink honors it); the current GET response omits it, which renders identically.

/** A layer as the annotations API returns it (the fields bake needs for z-order + the manifest). */
export interface DocLayer {
  id: string;
  order: number;
  mandatory?: boolean;
  roleTag?: string;
  /** Present in the API shape; unused by the renderer. */
  [k: string]: unknown;
}

/** The annotations document: layers + objects, verbatim from the API. */
export interface AnnotationsDoc {
  layers: DocLayer[];
  objects: InkObject[];
}

/** Pixel size of one source PDF page, 0-based by array index or explicit `index`. */
export interface PageSize {
  index: number;
  width: number;
  height: number;
}

export interface RenderOpts {
  /** Target overlay width in device pixels. Height is width / pageAspect. */
  width: number;
}

/** One rendered overlay: a layer's objects on a page, as a transparent PNG. */
export interface PageOverlay {
  page: number;
  layerId: string;
  /** z-order, mirrored from Layer.order. */
  order: number;
  mandatory: boolean;
  roleTag: string;
  /** sha256 of the PNG bytes (content-addressed change detection, R10). */
  contentHash: string;
  /** Transparent PNG bytes. */
  png: Uint8Array;
}

// --- The renderer ----------------------------------------------------------

function sha256Hex(bytes: Uint8Array): string {
  return createHash("sha256").update(bytes).digest("hex");
}

/** The per-object z fields the annotations API carries (T27 stage 2); ink's
 *  InkObject doesn't declare them, but the doc arrives verbatim from the API. */
type ZObject = InkObject & { order?: number; createdAt?: number };

/** Within-layer z comparator — MUST mirror studio's `compareObjectZ` contract
 *  (`order → createdAt → uuid`, T31): the bake renders what studio shows (I8). */
function objectZ(a: ZObject, b: ZObject): number {
  const ao = a.order ?? 0;
  const bo = b.order ?? 0;
  if (ao !== bo) return ao - bo;
  const ac = a.createdAt ?? 0;
  const bc = b.createdAt ?? 0;
  if (ac !== bc) return ac - bc;
  const au = a.uuid ?? "";
  const bu = b.uuid ?? "";
  return au < bu ? -1 : au > bu ? 1 : 0;
}

/**
 * Render every (page, layer) overlay for a document.
 *
 * Layers are z-ordered by `Layer.order` (ascending) so the manifest and file
 * set list them bottom-to-top. Within a layer, objects render sorted by
 * `order → createdAt → uuid` (T31) — the same `compareObjectZ` contract studio's
 * dry layer uses, so a bring-to-front done in studio is honored in the bake (I8).
 * A layer with no objects on a page produces NO overlay for that page.
 */
export function renderOverlays(
  doc: AnnotationsDoc,
  pageSizes: PageSize[],
  opts: RenderOpts,
): PageOverlay[] {
  ensureFont();
  ensureOffscreen();

  const layers = [...(doc.layers ?? [])].sort((a, b) => a.order - b.order);
  const objects = doc.objects ?? [];
  // Bucket objects by layerId then page so we draw each overlay in one pass.
  const byLayerPage = new Map<string, Map<number, InkObject[]>>();
  for (const o of objects) {
    const layerId = o.layerId ?? "";
    const page = o.page ?? 0;
    let pages = byLayerPage.get(layerId);
    if (!pages) byLayerPage.set(layerId, (pages = new Map()));
    let list = pages.get(page);
    if (!list) pages.set(page, (list = []));
    list.push(o);
  }

  const out: PageOverlay[] = [];
  for (const size of pageSizes) {
    if (!(size.width > 0) || !(size.height > 0)) continue;
    const aspect = size.width / size.height;
    const w = Math.max(1, Math.round(opts.width));
    const h = Math.max(1, Math.round(opts.width / aspect));
    const pageRect: PageRect = { x: 0, y: 0, w, h };

    for (const layer of layers) {
      const objs = byLayerPage.get(layer.id)?.get(size.index);
      if (!objs || objs.length === 0) continue;

      const canvas = createCanvas(w, h);
      // @napi-rs/canvas's 2D context is Skia-based and structurally matches ink's
      // Ctx2D; the nominal TS types differ (it is not lib.dom's context), so this
      // is the one sanctioned cast — ink's drawing path never branches on it (I8).
      const ctx = canvas.getContext("2d") as unknown as Ctx2D;
      // Deterministic text across Skia builds: disable hinting so glyph outlines
      // rasterize from pure geometry (the parity harness sets the same on its
      // browser canvas). Without this, Node-Skia and Chromium-Skia hint text
      // differently (~1px glyph-edge drift) and I8 text parity can't converge.
      (ctx as { textRendering?: string }).textRendering = "geometricPrecision";
      renderObjects(ctx, [...objs].sort(objectZ), pageRect);

      const png = canvas.encodeSync("png");
      out.push({
        page: size.index,
        layerId: layer.id,
        order: layer.order,
        mandatory: layer.mandatory ?? false,
        roleTag: layer.roleTag ?? "",
        contentHash: sha256Hex(png),
        png,
      });
    }
  }
  return out;
}

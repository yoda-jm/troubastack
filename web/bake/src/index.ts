/**
 * @troubastack/bake — the server-side bake worker (Node).
 *
 * Core invokes this worker to turn authoritative truth into a flat, performable
 * artifact (07-boundaries-and-no-duplication.md). It is the ONLY legitimate
 * server-side JS runtime (06-tech-stack.md): the overlay raster has a parity
 * requirement (I8), so it MUST be drawn with @troubastack/ink — the same
 * renderer studio uses — never re-implemented in Go.
 *
 * Pipeline (derives from I8, I11, I12; see 04-publish-pipeline.md & 05-distribution.md):
 *  1. Receive a bake request: a setlist's PINNED songs (a revision is minted
 *     only by an explicit admin bake — I11; bake never auto-publishes).
 *  2. Rasterize each PDF page at ~2–3× display DPI for crisp modest zoom. The
 *     PDF rasterizer (MuPDF/pdfium/poppler) is invoked by CORE; bake composes
 *     against those page rasters.
 *  3. Render the TRANSPARENT annotation overlay for each page by calling
 *     @troubastack/ink (renderObjects) onto an offscreen
 *     node canvas — PIXEL-IDENTICAL to studio's dry layer (I8 parity). One
 *     overlay per layer-group if performance-time toggles are wanted, else one.
 *  4. Flatten to self-contained WebP images (page raster + overlay) — the
 *     presenter is offline, dumb, and carries no annotation model or
 *     access-control logic (I12). Transparent overlays compress to ~nothing;
 *     page rasters dominate.
 *
 * Dependencies point only toward the contract (I14): bake depends on
 * @troubastack/ink and on proto types (web/proto-gen, I1); it imports no other
 * client. Intended runtime deps (a node canvas lib like @napi-rs/canvas) are
 * NOT installed yet — scaffold.
 */

import { renderObjects } from "@troubastack/ink";

// Encode the I8 boundary: bake renders strokes ONLY through @troubastack/ink.
void renderObjects;

/** A bake request: the pinned songs of one setlist (I11). View of a proto message (I1). */
export interface BakeRequest {
  /** Setlist being baked; concert rev = f(song revs + structure) (05-distribution.md). */
  setlistId: string;
  /** The pinned song revisions to flatten. */
  songs: ReadonlyArray<{ songId: string; rev: number }>;
}

/** A self-contained flattened page image (I12). */
export interface BakedPage {
  songId: string;
  pageIndex: number;
  /** Flattened WebP bytes: PDF raster composited with the annotation overlay. */
  webp: Uint8Array;
}

/**
 * Bake a request into flattened, self-contained page images (I11, I12).
 * Renders overlays via @troubastack/ink for pixel-parity with studio (I8).
 */
export function bake(_request: BakeRequest): Promise<BakedPage[]> {
  throw new Error("TODO");
}

/**
 * @troubastack/bake — the server-side bake worker (Node).
 *
 * Core invokes this worker to turn authoritative truth into a flat, performable
 * artifact (07-boundaries-and-no-duplication.md). It is the ONLY legitimate
 * server-side JS runtime (06-tech-stack.md): the overlay raster has a parity
 * requirement (I8), so it MUST be drawn with @troubastack/ink — the same renderer
 * studio uses — never re-implemented in Go.
 *
 * B01 implements the overlay half: `renderOverlays` (and the `troubabake` CLI over
 * it) render the TRANSPARENT per-layer annotation overlays for each page, pixel-
 * identical to studio's dry layer (guarded by the I8 parity test in test/). PDF
 * page rasterization + bundle.json/zip assembly are CORE's job (B02); bake composes
 * overlays only (04-publish-pipeline.md).
 */

export {
  renderOverlays,
  BAKE_TEXT_FONT_FAMILY,
  BAKE_TEXT_FONT_FILE,
  type AnnotationsDoc,
  type DocLayer,
  type PageSize,
  type RenderOpts,
  type PageOverlay,
} from "./render.js";

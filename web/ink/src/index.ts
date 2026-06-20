/**
 * @troubastack/ink — THE single stroke renderer (ARCHITECTURE.md I8).
 *
 * Stroke geometry + rasterization lives here ONCE and is consumed by both
 * `@troubastack/studio` (browser: dry layer + in-browser wet ink) and
 * `@troubastack/bake` (Node: server-side transparent overlays). The mobile
 * app's native ink overlay is the ONLY sanctioned re-implementation and must
 * match this output pixel-for-pixel under a golden-image parity test (I8).
 * There is no third copy — Go never draws strokes.
 *
 * Why it must be one renderer: the editor, the baked image, and the wet
 * overlay all draw the same stroke. If any two disagree, strokes visibly jump
 * on commit (wet→dry) or on bake (studio→bake). See
 * ../../docs/design/03-rendering-and-ink.md.
 *
 * perfect-freehand is the intended geometry library for buildStrokeGeometry()
 * (variable-width pressure outline). It is NOT installed yet — this is a
 * scaffold.
 */

// ---------------------------------------------------------------------------
// Types
//
// The CANONICAL wire/domain types are generated from ../proto into
// web/proto-gen (I1) and are never hand-written. The types below are
// render-facing VIEWS of that data — the minimal shape this renderer needs to
// turn an annotation into pixels. When proto-gen exists, these should be
// derived from / assignable to the generated stroke + style messages, not
// duplicated as a parallel source of truth.
// ---------------------------------------------------------------------------

/**
 * A single sampled stroke point in PDF-relative coordinates (I3).
 * x and y are normalized to the page in [0, 1] — NEVER pixels. Pixels never
 * enter persistence or the wire; they are produced only at draw time by
 * applying a {@link ViewportTransform}.
 */
export interface NormPoint {
  /** Page-relative X in [0, 1] (I3). */
  x: number;
  /** Page-relative Y in [0, 1] (I3). */
  y: number;
  /** Optional stylus pressure in [0, 1]; drives variable stroke width. */
  pressure?: number;
}

/** Visual style of a stroke. A render-facing view of the proto style message (I1). */
export interface Style {
  /** CSS color string (e.g. "#101010" or "rgba(...)"). */
  color: string;
  /**
   * Base stroke width, also PDF-relative (fraction of page) so width tracks
   * zoom for free (I3). Scaled into pixels by the ViewportTransform at draw time.
   */
  width: number;
  /** Optional opacity in [0, 1]. */
  opacity?: number;
}

/**
 * Maps PDF-relative [0,1] coordinates to device pixels: screen = clamp01(coord)
 * × transform (../../docs/design/03-rendering-and-ink.md). The web layer owns
 * this; it is static for the lifetime of a single stroke (you don't zoom
 * mid-stroke), so the wet layer only needs it at stroke-start.
 */
export interface ViewportTransform {
  /** Pixel width the page [0,1] maps onto. */
  scaleX: number;
  /** Pixel height the page [0,1] maps onto. */
  scaleY: number;
  /** Pixel X offset of the page origin. */
  offsetX: number;
  /** Pixel Y offset of the page origin. */
  offsetY: number;
}

/**
 * Resolution-independent geometry of a stroke: the outline polygon (and any
 * derived data) computed from the input points + style, still expressed in
 * PDF-relative [0,1] space (I3). Computed once, then drawn at any transform.
 * The exact shape is owned by this package (it is an internal render artifact,
 * not a wire type).
 */
export interface StrokeGeometry {
  /** Closed outline of the variable-width stroke, in PDF-relative [0,1] coords. */
  outline: NormPoint[];
  /** The style carried through so renderStroke needs only the geometry. */
  style: Style;
}

// ---------------------------------------------------------------------------
// The renderer surface (the entire public contract of web/ink)
// ---------------------------------------------------------------------------

/**
 * Build resolution-independent stroke geometry from sampled points + style.
 *
 * Pure and deterministic: same inputs → same outline, on browser and Node
 * alike. This determinism is what lets studio, bake, and the native overlay
 * agree to the pixel (I8). Input points are PDF-relative [0,1] (I3); output is
 * also [0,1] — no pixels here.
 *
 * Intended implementation: perfect-freehand (not yet installed).
 */
export function buildStrokeGeometry(_points: NormPoint[], _style: Style): StrokeGeometry {
  throw new Error("TODO");
}

/**
 * Draw pre-built geometry onto a 2D canvas context, applying the viewport
 * transform to convert PDF-relative [0,1] geometry into device pixels (I3).
 *
 * Accepts both the browser `CanvasRenderingContext2D` and the worker
 * `OffscreenCanvasRenderingContext2D` so the IDENTICAL routine serves studio's
 * live canvas and bake's offscreen raster (I8). The drawing path must not
 * branch on environment — only the supplied context differs.
 */
export function renderStroke(
  _ctx: CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D,
  _geom: StrokeGeometry,
  _transform: ViewportTransform,
): void {
  throw new Error("TODO");
}

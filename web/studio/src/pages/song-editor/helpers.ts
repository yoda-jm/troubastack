/**
 * Small editor helpers shared by the wet canvas and the dry overlay (T10
 * extraction — moved verbatim from SongEditor.tsx). No behavior change.
 */
import type { AnnotationLayer, AnnotationObject, AnnotationStyle, Role } from "../../api";
import type { InkObject } from "@troubastack/ink";
import { pointsForTool, type DrawTool } from "../../editor";

export type PRPoint = { x: number; y: number };

/** Raster device-pixel-ratio, CLAMPED (T41). Every page stacks three full-res canvases
 *  (PDF raster + annotation overlay + wet EditCanvas); at a phone's native DPR (often
 *  3–4) the cumulative backing store exhausts the browser's canvas/GPU budget and later
 *  pages fail to allocate — rendering solid BLACK, worse after scrolling (backing-store
 *  eviction). Capping the raster DPR shrinks every canvas ~2–4× with imperceptible loss
 *  at phone reading distance; desktop "retina" (DPR 2) is unaffected (min(2,2)=2). ALL
 *  canvas sizing must use this one value so raster/overlay/wet stay pixel-aligned. */
const MAX_RASTER_DPR = 2;
export function rasterDpr(): number {
  return Math.min(window.devicePixelRatio || 1, MAX_RASTER_DPR);
}

// T44: two hard limits make a phone paint a page canvas solid BLACK (VLL, worse on
// zoom-in, recovers on zoom-out): (1) a single side over the GPU max-texture floor
// (~4096 px), and (2) the SUM of all page canvases' backing stores (× the ~3 canvases
// each page stacks) over the device's canvas-memory budget. budgetedRasterDpr caps the
// raster DPR so NEITHER is exceeded — scaling the raster DOWN (softer, never black) only
// when the requested resolution would blow a limit; 1:1 for the desktop / small-doc /
// fit-zoom common case. It changes RESOLUTION only, never display size, so T27's
// zero-shift + scrollIntoView are untouched. Pure (unit-shaped: CI asserts the derived
// dims via emulated dpr/zoom; the black-gone proof is on-device).
const MAX_CANVAS_SIDE = 4096; // device px; Android GPU max-texture floor
const CANVASES_PER_PAGE = 3; // raster + annotation overlay + wet EditCanvas
const TOTAL_CANVAS_BUDGET_PX = 32_000_000; // ≈128 MB RGBA summed across every canvas
export function budgetedRasterDpr(rawDpr: number, pageCssSizes: { w: number; h: number }[]): number {
  let sumArea = 0;
  let maxSide = 0;
  for (const p of pageCssSizes) {
    if (p.w <= 0 || p.h <= 0) continue;
    sumArea += p.w * p.h;
    maxSide = Math.max(maxSide, p.w, p.h);
  }
  if (sumArea <= 0 || maxSide <= 0) return rawDpr;
  const byBudget = Math.sqrt(TOTAL_CANVAS_BUDGET_PX / (CANVASES_PER_PAGE * sumArea));
  const bySide = MAX_CANVAS_SIDE / maxSide; // device-side cap, in DPR terms
  // Budget clamp with a readable floor (0.5), THEN the per-side cap as a HARD ceiling
  // (exceeding it blacks out regardless of memory, so it always wins).
  const soft = Math.max(0.5, Math.min(rawDpr, byBudget));
  return Math.min(soft, bySide);
}

/** Per-layer local visibility toggles: layerId → shown. */
export type LayerVisibility = Record<string, boolean>;

/** Render/hit-test z-order within a page (T27): layer z-rank first (via `layerRank`,
 *  the index of each layer in the sorted stack), then per-object `order`. Equal keys
 *  fall back to the caller's array order under a STABLE sort (= insertion/creation
 *  order), so untouched docs keep their original stacking. Ascending = back→front, so
 *  the LAST element paints on top (and is the topmost pick candidate). The SAME
 *  comparator drives the dry overlay paint and the wet-canvas hit-test so what looks
 *  on top is what a click selects. */
export function compareObjectZ(
  a: AnnotationObject,
  b: AnnotationObject,
  layerRank: Map<string, number>,
): number {
  const la = layerRank.get(a.layerId) ?? 0;
  const lb = layerRank.get(b.layerId) ?? 0;
  if (la !== lb) return la - lb;
  if ((a.order ?? 0) !== (b.order ?? 0)) return (a.order ?? 0) - (b.order ?? 0);
  // Spec tiebreak (T27): created_at, then uuid — array-order-independent, so equal
  // `order` objects stack deterministically across clients/passes (no reliance on
  // JS stable-sort insertion order). created_at ties → uuid is the final total order.
  if ((a.createdAt ?? 0) !== (b.createdAt ?? 0)) return (a.createdAt ?? 0) - (b.createdAt ?? 0);
  return a.uuid < b.uuid ? -1 : a.uuid > b.uuid ? 1 : 0;
}

/** Whether the given user/role may edit objects on a layer (conductor zone is
 *  conductor-only; else owner or shared-RW). Mirrors the WS write gate's intent. */
export function isEditableLayer(
  l: AnnotationLayer,
  myUserId: string | null,
  myRole: Role | null,
): boolean {
  if (l.zone === "conductor") return myRole === "conductor";
  return (myUserId != null && l.ownerId === myUserId) || l.access === "rw";
}

// A shared offscreen 2D context for text measurement (font matches web/ink).
let measureCtx: CanvasRenderingContext2D | null = null;
export function measureTextWidth(text: string, fontPx: number): number {
  if (!measureCtx) {
    const c = document.createElement("canvas");
    measureCtx = c.getContext("2d");
  }
  if (!measureCtx) return text.length * fontPx * 0.5; // crude fallback
  measureCtx.font = `${fontPx}px system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
  return measureCtx.measureText(text).width;
}

/** Build a wet preview object from an in-progress gesture (no uuid needed). */
export function buildWet(
  tool: DrawTool,
  path: PRPoint[],
  style: AnnotationStyle,
  // T51: per-type content for the WET preview — the icon tool needs its glyph id here
  // so the preview renders the chosen glyph mid-drag (else drawIcon falls back to
  // `note`, the eighth-note flag VLL saw). Empty for tools that ignore `text`.
  text = "",
): AnnotationObject | null {
  if (path.length === 0) return null;
  return {
    uuid: "wet",
    layerId: "wet",
    type: tool as AnnotationObject["type"],
    points: pointsForTool(tool, path),
    page: 0,
    text,
    order: 0, // transient preview; z-order/createdAt are irrelevant for the wet layer
    createdAt: 0,
    style,
  };
}

/** Map an API annotation object to the renderer's InkObject view (same shape). */
export function toInkObject(o: AnnotationObject): InkObject {
  return {
    uuid: o.uuid,
    layerId: o.layerId,
    type: o.type,
    points: o.points,
    page: o.page,
    text: o.text,
    style: o.style,
  };
}

/**
 * Small editor helpers shared by the wet canvas and the dry overlay (T10
 * extraction — moved verbatim from SongEditor.tsx). No behavior change.
 */
import type { AnnotationLayer, AnnotationObject, AnnotationStyle, Role } from "../../api";
import type { InkObject } from "@troubastack/ink";
import { pointsForTool, type DrawTool } from "../../editor";

export type PRPoint = { x: number; y: number };

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
): AnnotationObject | null {
  if (path.length === 0) return null;
  return {
    uuid: "wet",
    layerId: "wet",
    type: tool as AnnotationObject["type"],
    points: pointsForTool(tool, path),
    page: 0,
    text: "",
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

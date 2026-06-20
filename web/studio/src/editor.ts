/**
 * Editor primitives shared by SongEditor: the tool/style model, the
 * pointer→page-relative [0,1] mapping, and the object-builder that turns a
 * gesture into a wire AnnotationObject.
 *
 * Coordinates: persisted points are PAGE-RELATIVE [0,1] (I3). The render path
 * maps [0,1]→pixels via the page canvas box (web/ink's toPx). Here we invert
 * that: a pointer's clientX/Y is mapped against the page canvas's bounding rect,
 * which the browser already reports in CSS px AFTER zoom/scroll, so the result
 * is correct under any zoom level, scroll position, and devicePixelRatio (the
 * backing-store DPR scaling never enters because getBoundingClientRect is CSS px).
 */
import type { AnnotationObject, AnnotationStyle } from "./api";

export type Tool =
  | "select"
  | "freehand"
  | "line"
  | "rect"
  | "ellipse"
  | "highlight"
  | "text";

/** Tools that draw a new object (everything but select). */
export type DrawTool = Exclude<Tool, "select">;

/** The annotation object type a draw tool produces. */
export function toolObjectType(tool: DrawTool): AnnotationObject["type"] {
  return tool; // tool names line up 1:1 with object types
}

export const DEFAULT_STYLE: AnnotationStyle = {
  color: "#e11d48",
  opacity: 1,
  // width is a FRACTION OF PAGE WIDTH (I3); ~0.4% reads as a medium pen.
  width: 0.004,
  // fontSize is a FRACTION OF PAGE HEIGHT (text only).
  fontSize: 0.03,
};

/** Swatches offered in the palette. */
export const COLOR_SWATCHES = [
  "#e11d48",
  "#2563eb",
  "#059669",
  "#f59e0b",
  "#7c3aed",
  "#111827",
];

/** Map a pointer event to page-relative [0,1] against a page canvas's box. */
export function pointerToPageXY(
  clientX: number,
  clientY: number,
  pageEl: HTMLElement,
): { x: number; y: number } {
  const rect = pageEl.getBoundingClientRect();
  if (rect.width <= 0 || rect.height <= 0) return { x: 0, y: 0 };
  const x = (clientX - rect.left) / rect.width;
  const y = (clientY - rect.top) / rect.height;
  return { x: clamp01(x), y: clamp01(y) };
}

export function clamp01(n: number): number {
  if (Number.isNaN(n)) return 0;
  return n < 0 ? 0 : n > 1 ? 1 : n;
}

/**
 * Build the wire points for an in-progress gesture given the tool, the path of
 * page-relative points captured so far, and (for text) the anchor.
 *   freehand → the full path
 *   line     → [start, end]
 *   rect/ellipse/highlight → [start, end] bbox corners
 *   text     → [anchor]
 */
export function pointsForTool(
  tool: DrawTool,
  path: { x: number; y: number }[],
): { x: number; y: number }[] {
  if (path.length === 0) return [];
  if (tool === "freehand") return path;
  if (tool === "text") return [path[0]];
  const start = path[0];
  const end = path[path.length - 1];
  return [start, end];
}

/** Assemble a complete wire object for a finished gesture. */
export function buildObject(args: {
  tool: DrawTool;
  points: { x: number; y: number }[];
  page: number;
  layerId: string;
  style: AnnotationStyle;
  text?: string;
}): AnnotationObject {
  return {
    uuid: crypto.randomUUID(),
    layerId: args.layerId,
    type: toolObjectType(args.tool),
    points: args.points,
    page: args.page,
    text: args.text ?? "",
    style: { ...args.style },
  };
}

/** Is a gesture big enough to be a real object (avoids stray click artifacts)? */
export function isMeaningfulGesture(tool: DrawTool, path: { x: number; y: number }[]): boolean {
  if (tool === "text") return path.length >= 1;
  if (path.length < 2) return false;
  if (tool === "freehand") return path.length >= 2;
  const a = path[0];
  const b = path[path.length - 1];
  const span = Math.hypot(b.x - a.x, b.y - a.y);
  return span > 0.005; // ~0.5% of the page — a deliberate drag
}

/** Translate every point of an object by a page-relative delta (for move). */
export function translateObject(
  obj: AnnotationObject,
  dx: number,
  dy: number,
): AnnotationObject {
  return {
    ...obj,
    points: obj.points.map((p) => ({ x: clamp01(p.x + dx), y: clamp01(p.y + dy) })),
  };
}

/** Axis-aligned [0,1] bounding box of an object (for hit-testing). */
export function objectBBox(obj: AnnotationObject): {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
} {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const p of obj.points) {
    if (p.x < minX) minX = p.x;
    if (p.y < minY) minY = p.y;
    if (p.x > maxX) maxX = p.x;
    if (p.y > maxY) maxY = p.y;
  }
  if (!Number.isFinite(minX)) return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
  return { minX, minY, maxX, maxY };
}

/**
 * Hit-test a page-relative point against an object. Uses a padded bounding box
 * — the pad (in [0,1]) gives thin lines/strokes a grabbable tolerance and makes
 * a text anchor clickable. Good enough for select/move/delete.
 */
export function hitTest(
  obj: AnnotationObject,
  x: number,
  y: number,
  pad = 0.02,
): boolean {
  const b = objectBBox(obj);
  return (
    x >= b.minX - pad &&
    x <= b.maxX + pad &&
    y >= b.minY - pad &&
    y <= b.maxY + pad
  );
}

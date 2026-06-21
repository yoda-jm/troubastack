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

/** Axis-aligned [0,1] bounding box of an object. A point-set object's box is the
 *  extent of its points. A TEXT object has only an anchor point, so on its own
 *  it would be a zero-size box (impossible to click / no selection box). When a
 *  `measure` is supplied (the page aspect ratio + a canvas text measurer) a text
 *  object gets a REAL box derived from its measured width × line height, anchored
 *  top-left (matching the renderer's textBaseline:"top"). */
export function objectBBox(
  obj: AnnotationObject,
  measure?: TextMeasure,
): {
  minX: number;
  minY: number;
  maxX: number;
  maxY: number;
} {
  if (obj.type === "text" && measure) {
    return textBBox(obj, measure);
  }
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
 * A text measurer the editor injects: it reports a string's pixel width at a
 * given pixel font size (via canvas measureText), plus the page's pixel
 * dimensions so we can convert that width into page-relative [0,1] fractions.
 * Kept as an injected dependency so editor.ts stays DOM-free and unit-testable.
 */
export type TextMeasure = {
  /** Page box pixel size — width fractions and height fractions differ by it. */
  pageW: number;
  pageH: number;
  /** Measure a string's width in CSS px at the given pixel font size. */
  widthPx: (text: string, fontPx: number) => number;
};

/** The font's pixel size for a text object on a page of height `pageH` px.
 *  style.fontSize is a FRACTION OF PAGE HEIGHT (matches web/ink's fontPx). */
export function textFontPx(obj: AnnotationObject, pageH: number): number {
  return Math.max(1, obj.style.fontSize * pageH);
}

/** Real [0,1] bbox of a text object: anchor is top-left, width from measured
 *  text, height = fontSize (page-height fraction) × line count. */
export function textBBox(
  obj: AnnotationObject,
  m: TextMeasure,
): { minX: number; minY: number; maxX: number; maxY: number } {
  const anchor = obj.points[0] ?? { x: 0, y: 0 };
  const text = obj.text ?? "";
  const lines = text.length ? text.split("\n") : [""];
  const fontPx = textFontPx(obj, m.pageH);
  let maxWidthPx = 0;
  for (const line of lines) {
    const w = m.widthPx(line, fontPx);
    if (w > maxWidthPx) maxWidthPx = w;
  }
  // Width as a fraction of PAGE WIDTH; height as a fraction of PAGE HEIGHT.
  const wFrac = m.pageW > 0 ? maxWidthPx / m.pageW : 0;
  const lineHeightFrac = obj.style.fontSize * 1.2; // 1.2 line height
  const hFrac = lineHeightFrac * lines.length;
  // Give an empty/edge text a small minimum so it's still grabbable.
  const minX = anchor.x;
  const minY = anchor.y;
  const maxX = clamp01(anchor.x + Math.max(wFrac, 0.01));
  const maxY = clamp01(anchor.y + Math.max(hFrac, obj.style.fontSize));
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
  measure?: TextMeasure,
): boolean {
  const b = objectBBox(obj, measure);
  return (
    x >= b.minX - pad &&
    x <= b.maxX + pad &&
    y >= b.minY - pad &&
    y <= b.maxY + pad
  );
}

/** A page-relative [0,1] rectangle (rubber-band selection box). */
export type SelectRect = { x0: number; y0: number; x1: number; y1: number };

/** Normalize a drag (start→end) into a min/max rect. */
export function normalizeRect(a: { x: number; y: number }, b: { x: number; y: number }): SelectRect {
  return {
    x0: Math.min(a.x, b.x),
    y0: Math.min(a.y, b.y),
    x1: Math.max(a.x, b.x),
    y1: Math.max(a.y, b.y),
  };
}

/** Does an object's bounding box intersect the selection rect? */
export function intersectsRect(
  obj: AnnotationObject,
  r: SelectRect,
  measure?: TextMeasure,
): boolean {
  const b = objectBBox(obj, measure);
  return !(b.maxX < r.x0 || b.minX > r.x1 || b.maxY < r.y0 || b.minY > r.y1);
}

/** Is a rubber-band drag big enough to count as a marquee (vs. a stray click)? */
export function isMarquee(r: SelectRect): boolean {
  return Math.hypot(r.x1 - r.x0, r.y1 - r.y0) > 0.01;
}

// ---------------------------------------------------------------------------
// Resize handles
// ---------------------------------------------------------------------------

/** The four corner handles of a selection box. */
export type HandleId = "nw" | "ne" | "se" | "sw";

export const CORNER_HANDLES: HandleId[] = ["nw", "ne", "se", "sw"];

/** A handle's anchor point (page-relative [0,1]) on an object's bbox. The handle
 *  the user drags moves THIS point; the diagonally-opposite corner stays fixed. */
export function handlePoint(
  b: { minX: number; minY: number; maxX: number; maxY: number },
  h: HandleId,
): { x: number; y: number } {
  const x = h === "nw" || h === "sw" ? b.minX : b.maxX;
  const y = h === "nw" || h === "ne" ? b.minY : b.maxY;
  return { x, y };
}

/** The corner diagonally opposite a handle (the fixed anchor during a resize). */
function oppositeCorner(
  b: { minX: number; minY: number; maxX: number; maxY: number },
  h: HandleId,
): { x: number; y: number } {
  const opp: Record<HandleId, HandleId> = { nw: "se", ne: "sw", se: "nw", sw: "ne" };
  return handlePoint(b, opp[h]);
}

/**
 * Resize an object by dragging one of its corner handles by a page-relative
 * delta (dx,dy). The opposite corner stays fixed; the dragged corner moves by
 * the delta. All of the object's points are remapped from the OLD bbox into the
 * NEW bbox (uniform per-axis scaling), so:
 *   - rect/ellipse/highlight/line → their two points follow the new box,
 *   - freehand → its whole path scales into the new box,
 *   - text → its anchor follows and fontSize scales by the height ratio.
 * Guards against a degenerate (zero/near-zero) box so points don't collapse.
 */
export function resizeObject(
  obj: AnnotationObject,
  handle: HandleId,
  dx: number,
  dy: number,
  measure?: TextMeasure,
): AnnotationObject {
  const b = objectBBox(obj, measure);
  const fixed = oppositeCorner(b, handle);
  const dragged = handlePoint(b, handle);
  const newDragged = { x: clamp01(dragged.x + dx), y: clamp01(dragged.y + dy) };

  const oldW = b.maxX - b.minX;
  const oldH = b.maxY - b.minY;
  const newMinX = Math.min(fixed.x, newDragged.x);
  const newMaxX = Math.max(fixed.x, newDragged.x);
  const newMinY = Math.min(fixed.y, newDragged.y);
  const newMaxY = Math.max(fixed.y, newDragged.y);
  const newW = newMaxX - newMinX;
  const newH = newMaxY - newMinY;

  const sx = oldW > 1e-6 ? newW / oldW : 1;
  const sy = oldH > 1e-6 ? newH / oldH : 1;

  if (obj.type === "text") {
    // Text: move the anchor to the new top-left and scale fontSize by the
    // height ratio (so the box drag visibly resizes the glyphs).
    const nextFont = Math.max(0.005, obj.style.fontSize * (sy > 0 ? sy : 1));
    return {
      ...obj,
      points: [{ x: clamp01(newMinX), y: clamp01(newMinY) }],
      style: { ...obj.style, fontSize: nextFont },
    };
  }

  const points = obj.points.map((p) => ({
    x: clamp01(newMinX + (p.x - b.minX) * sx),
    y: clamp01(newMinY + (p.y - b.minY) * sy),
  }));
  return { ...obj, points };
}

/** A short human label for an annotation, for the per-layer annotation list. */
export function objectLabel(obj: AnnotationObject): string {
  if (obj.type === "text") {
    const t = obj.text.trim();
    return t ? `text: "${t.length > 20 ? `${t.slice(0, 20)}…` : t}"` : "text";
  }
  return obj.type;
}

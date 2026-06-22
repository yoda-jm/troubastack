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

// The Highlight TOOL is gone (#5): highlighting is now a STYLE preset on rect/ellipse.
// Tools map 1:1 to object types; the editor never creates a "highlight" object anymore
// (legacy "highlight" objects still render, see web/ink).
export type Tool =
  | "select"
  | "freehand"
  | "line"
  | "rect"
  | "ellipse"
  | "text";

/** Tools that draw a new object (everything but select). */
export type DrawTool = Exclude<Tool, "select">;

/** The annotation object type a draw tool produces. */
export function toolObjectType(tool: DrawTool): AnnotationObject["type"] {
  return tool; // tool names line up 1:1 with object types
}

// ---------------------------------------------------------------------------
// Shape style presets (#5) — fill/stroke/blend combos for rect/ellipse
// ---------------------------------------------------------------------------

/** The shape-style flags a preset sets (merged onto the current color/width/opacity). */
export type ShapeStyleFlags = {
  fill: boolean;
  stroke: boolean;
  blend: "normal" | "multiply";
};

export type PresetId = "outline" | "box" | "highlight";

/** The three one-click presets: Outline (stroke only), Box (stroke + fill), Highlight
 *  (fill only, multiply, no stroke — the old highlighter look as a rect/ellipse). */
export const SHAPE_PRESETS: Record<PresetId, ShapeStyleFlags> = {
  outline: { fill: false, stroke: true, blend: "normal" },
  box: { fill: true, stroke: true, blend: "normal" },
  highlight: { fill: true, stroke: false, blend: "multiply" },
};

/** Apply a preset's flags onto a style, preserving color/opacity/width/fontSize. */
export function applyPreset(style: AnnotationStyle, preset: PresetId): AnnotationStyle {
  return { ...style, ...SHAPE_PRESETS[preset] };
}

/** Which preset (if any) a style currently matches — drives the active preset button. */
export function matchPreset(style: AnnotationStyle): PresetId | null {
  for (const id of Object.keys(SHAPE_PRESETS) as PresetId[]) {
    const p = SHAPE_PRESETS[id];
    const fill = style.fill ?? false;
    const stroke = style.stroke ?? true;
    const blend = style.blend ?? "normal";
    if (fill === p.fill && stroke === p.stroke && blend === p.blend) return id;
  }
  return null;
}

export const DEFAULT_STYLE: AnnotationStyle = {
  color: "#e11d48",
  opacity: 1,
  // width is a FRACTION OF PAGE WIDTH (I3); ~0.4% reads as a medium pen.
  width: 0.004,
  // fontSize is a FRACTION OF PAGE HEIGHT (text only).
  fontSize: 0.03,
  // Shape defaults (rect/ellipse): the Outline preset — stroke only, normal blend.
  fill: false,
  stroke: true,
  blend: "normal",
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

// Bug #1: text is hard to click. A text object's GLYPH box is small, so we pad its
// hit area generously: a minimum of MIN_TEXT_HIT_PX CSS px AND a small fraction of the
// page on every side, so clicking on OR just around the glyphs selects it. The padding
// is per-axis (px→fraction differs by page width vs height).
const MIN_TEXT_HIT_PX = 14; // CSS px of grab slop around glyphs (a few px, generous)
const TEXT_HIT_FRAC = 0.03; // ...or this fraction of the page, whichever is larger

/** Per-axis generous hit pad (page-relative [0,1]) for a TEXT object: max(min px as a
 *  fraction of the page on that axis, a small page fraction). Falls back to the flat
 *  fraction when no measure (page dims) is available. */
export function textHitPad(measure?: TextMeasure): { padX: number; padY: number } {
  if (!measure || measure.pageW <= 0 || measure.pageH <= 0) {
    return { padX: TEXT_HIT_FRAC, padY: TEXT_HIT_FRAC };
  }
  return {
    padX: Math.max(MIN_TEXT_HIT_PX / measure.pageW, TEXT_HIT_FRAC),
    padY: Math.max(MIN_TEXT_HIT_PX / measure.pageH, TEXT_HIT_FRAC),
  };
}

/**
 * Hit-test a page-relative point against an object. Uses a padded bounding box
 * — the pad (in [0,1]) gives thin lines/strokes a grabbable tolerance and makes
 * a text anchor clickable. Good enough for select/move/delete.
 *
 * Bug #1: TEXT objects get an extra-generous per-axis pad (textHitPad) ON TOP of the
 * caller's base pad, so a click slightly outside the glyph extents still selects them.
 */
export function hitTest(
  obj: AnnotationObject,
  x: number,
  y: number,
  pad = 0.02,
  measure?: TextMeasure,
): boolean {
  const b = objectBBox(obj, measure);
  let padX = pad;
  let padY = pad;
  if (obj.type === "text") {
    const tp = textHitPad(measure);
    padX = Math.max(pad, tp.padX);
    padY = Math.max(pad, tp.padY);
  }
  return (
    x >= b.minX - padX &&
    x <= b.maxX + padX &&
    y >= b.minY - padY &&
    y <= b.maxY + padY
  );
}

// ---------------------------------------------------------------------------
// Unified pick (#refine) — one function decides what's under the pointer AND
// what the next gesture would do, so the hover cursor always predicts the drag.
// ---------------------------------------------------------------------------

/** Distance (page-relative [0,1]) from point p to segment a→b. */
function distToSegment(
  p: { x: number; y: number },
  a: { x: number; y: number },
  b: { x: number; y: number },
): number {
  const vx = b.x - a.x;
  const vy = b.y - a.y;
  const len2 = vx * vx + vy * vy;
  if (len2 <= 1e-12) return Math.hypot(p.x - a.x, p.y - a.y);
  let t = ((p.x - a.x) * vx + (p.y - a.y) * vy) / len2;
  t = t < 0 ? 0 : t > 1 ? 1 : t;
  const cx = a.x + t * vx;
  const cy = a.y + t * vy;
  return Math.hypot(p.x - cx, p.y - cy);
}

/** Min distance ([0,1]) from a point to a polyline (sequence of points). */
function distToPolyline(p: { x: number; y: number }, pts: { x: number; y: number }[]): number {
  if (pts.length === 0) return Infinity;
  if (pts.length === 1) return Math.hypot(p.x - pts[0].x, p.y - pts[0].y);
  let best = Infinity;
  for (let i = 1; i < pts.length; i++) {
    const d = distToSegment(p, pts[i - 1], pts[i]);
    if (d < best) best = d;
  }
  return best;
}

/** Is a point inside an axis-aligned box (with a per-axis pad)? */
function insideBox(
  b: { minX: number; minY: number; maxX: number; maxY: number },
  x: number,
  y: number,
  padX = 0,
  padY = 0,
): boolean {
  return x >= b.minX - padX && x <= b.maxX + padX && y >= b.minY - padY && y <= b.maxY + padY;
}

/**
 * The on-line tolerance (page-relative [0,1]) for a stroke object — at least the
 * stroke half-width, but never less than ~6px so a hairline is still grabbable.
 * style.width is a fraction of PAGE WIDTH; we convert to px via pageW.
 */
const MIN_STROKE_HIT_PX = 6;
function strokeHitFrac(
  obj: AnnotationObject,
  axis: "x" | "y",
  pageW: number,
  pageH: number,
): number {
  const px = axis === "x" ? pageW : pageH;
  if (px <= 0) return 0.01;
  const halfWidthPx = (obj.style.width * pageW) / 2; // width is a page-WIDTH fraction
  const tolPx = Math.max(halfWidthPx, MIN_STROKE_HIT_PX);
  return tolPx / px;
}

/** What `pickAt` decides should happen on the next press at a point. */
export type PickMode = "resize" | "move" | "select" | "none";

export type PickResult = {
  object: AnnotationObject | null;
  mode: PickMode;
  handle?: HandleId;
};

/** Context `pickAt` needs: the visible objects on the page (in z-order, earliest
 *  first → last is topmost), the page box px size, a text measurer, the current
 *  single selection (drives resize handles), and the "editable now" predicate. */
export type PickContext = {
  objects: AnnotationObject[];
  pageW: number;
  pageH: number;
  measure?: TextMeasure;
  /** The single selected object (or null) — only it shows resize handles. */
  selected: AnnotationObject | null;
  isEditableNow: (obj: AnnotationObject) => boolean;
};

/** A "strong" hit = the point is inside the object's TRUE shape, OR within only a
 *  SMALL fixed margin of it (a few px) — this is the MOVABLE region (Bug #3). A
 *  "weak" hit = only within the generous proximity pad beyond that, which stays
 *  SELECTABLE (so thin lines / text are easy to click) but never starts a move. */
type HitStrength = "strong" | "weak" | "none";

/** The small fixed margin (px) added around a containment shape's true body that
 *  still counts as a MOVE-eligible (strong) hit. A few px so a press right at the
 *  edge still grabs to move, without the generous select pad's overreach (#3). */
const MOVE_MARGIN_PX = 6;

/** The MOVE_MARGIN_PX margin as a page-relative [0,1] fraction on the given axis. */
function moveMarginFrac(axis: "x" | "y", pageW: number, pageH: number): number {
  const dim = axis === "x" ? pageW : pageH;
  return dim > 0 ? MOVE_MARGIN_PX / dim : 0.004;
}

/** Classify a point against ONE object's true geometry vs. its proximity pad. */
function classifyHit(
  obj: AnnotationObject,
  x: number,
  y: number,
  pageW: number,
  pageH: number,
  measure?: TextMeasure,
): HitStrength {
  const b = objectBBox(obj, measure);

  if (obj.type === "text") {
    // Text: inside the GLYPH box (+ a small fixed px move-margin) is strong
    // (movable, #3); within the generous text pad beyond that is weak (select
    // only — so clicking just around the glyphs still selects, never moves).
    const tp = textHitPad(measure);
    const mX = moveMarginFrac("x", pageW, pageH);
    const mY = moveMarginFrac("y", pageW, pageH);
    if (insideBox(b, x, y, mX, mY)) return "strong";
    if (insideBox(b, x, y, tp.padX, tp.padY)) return "weak";
    return "none";
  }

  if (obj.type === "line") {
    const a = obj.points[0];
    const c = obj.points[obj.points.length - 1];
    if (!a || !c) return "none";
    // Distance to the SEGMENT, in [0,1]; compare against the stroke tolerance.
    // (Use the larger per-axis frac so a near-vertical/horizontal hairline still
    // grabs symmetrically.) Strong = on the stroke; weak = within a small pad.
    const tol = Math.max(strokeHitFrac(obj, "x", pageW, pageH), strokeHitFrac(obj, "y", pageW, pageH));
    const d = distToSegment({ x, y }, a, c);
    if (d <= tol) return "strong";
    const weakPad = pageW > 0 ? (MIN_STROKE_HIT_PX + 6) / Math.min(pageW, pageH) : 0.02;
    if (d <= tol + weakPad) return "weak";
    return "none";
  }

  if (obj.type === "freehand") {
    const tol = Math.max(strokeHitFrac(obj, "x", pageW, pageH), strokeHitFrac(obj, "y", pageW, pageH));
    const d = distToPolyline({ x, y }, obj.points);
    if (d <= tol) return "strong";
    const weakPad = pageW > 0 ? (MIN_STROKE_HIT_PX + 6) / Math.min(pageW, pageH) : 0.02;
    if (d <= tol + weakPad) return "weak";
    return "none";
  }

  if (obj.type === "ellipse") {
    // Inside the ellipse equation (NOT the bbox corners). Centre + radii from bbox.
    const cx = (b.minX + b.maxX) / 2;
    const cy = (b.minY + b.maxY) / 2;
    const rx = (b.maxX - b.minX) / 2;
    const ry = (b.maxY - b.minY) / 2;
    // Strong (movable): inside the ellipse equation grown by the small fixed
    // px move-margin on each axis (so a press right at the edge still grabs).
    if (rx > 1e-6 && ry > 1e-6) {
      const mX = moveMarginFrac("x", pageW, pageH);
      const mY = moveMarginFrac("y", pageW, pageH);
      const nx = (x - cx) / (rx + mX);
      const ny = (y - cy) / (ry + mY);
      if (nx * nx + ny * ny <= 1.0) return "strong";
    }
    // Weak: within a small proximity pad of the bbox (so near-edge clicks grab).
    if (insideBox(b, x, y, 0.012, 0.012)) return "weak";
    return "none";
  }

  // rect / highlight / any filled shape: inside the rectangle + a SMALL fixed
  // px margin = strong (movable, #3); within the generous proximity pad beyond
  // that = weak (selectable only).
  const mX = moveMarginFrac("x", pageW, pageH);
  const mY = moveMarginFrac("y", pageW, pageH);
  if (insideBox(b, x, y, mX, mY)) return "strong";
  if (insideBox(b, x, y, 0.02, 0.02)) return "weak";
  return "none";
}

/** The effective area (for the smallest-wins tie-break) of an object's bbox. */
function bboxArea(obj: AnnotationObject, measure?: TextMeasure): number {
  const b = objectBBox(obj, measure);
  return Math.max(0, b.maxX - b.minX) * Math.max(0, b.maxY - b.minY);
}

/**
 * The unified pick: decide what's under `pt` and what the next press would do.
 *
 * Priority:
 *  1. RESIZE — over a visible resize handle of the CURRENT single selection
 *     (handles sit on top of the selection, so this wins outright).
 *  2. Among visible objects whose hit-region covers the point, rank by:
 *       a. containment tier — a STRONG hit (inside the true shape) beats a WEAK
 *          hit (only within the proximity pad just outside it);
 *       b. within a tier, the SMALLEST bbox area wins; tie → topmost in z-order.
 *     The winner's mode is "move" if it's editable-now, else "select".
 *  3. Nothing under the point → "none".
 */
export function pickAt(pt: { x: number; y: number }, ctx: PickContext): PickResult {
  // 1) Resize handles of the current selection take precedence.
  if (ctx.selected && ctx.isEditableNow(ctx.selected) && ctx.pageW > 0 && ctx.pageH > 0) {
    const b = objectBBox(ctx.selected, ctx.measure);
    const handle = handleAtPx(b, pt, ctx.pageW, ctx.pageH);
    if (handle) return { object: ctx.selected, mode: "resize", handle };
  }

  // 2) Candidate set: visible objects whose hit-region (strong OR weak) covers pt.
  type Cand = { obj: AnnotationObject; strong: boolean; area: number; z: number };
  const cands: Cand[] = [];
  for (let z = 0; z < ctx.objects.length; z++) {
    const obj = ctx.objects[z];
    const s = classifyHit(obj, pt.x, pt.y, ctx.pageW, ctx.pageH, ctx.measure);
    if (s === "none") continue;
    cands.push({ obj, strong: s === "strong", area: bboxArea(obj, ctx.measure), z });
  }
  if (cands.length === 0) return { object: null, mode: "none" };

  cands.sort((a, b) => {
    // Strong beats weak.
    if (a.strong !== b.strong) return a.strong ? -1 : 1;
    // Smallest effective area wins.
    if (a.area !== b.area) return a.area - b.area;
    // Tie → topmost (highest z) wins.
    return b.z - a.z;
  });

  const top = cands[0];
  const winner = top.obj;
  // Bug #3: MOVE requires a STRONG (containment) hit — the pointer inside the
  // object's true shape (or within only a tiny margin baked into `classifyHit`'s
  // strong test), NOT the generous proximity pad. A WEAK (pad-only) hit still
  // SELECTS (so thin lines / text stay easy to click), but never starts a move.
  // Rectangles use a tight strong tol, so they keep their exact feel.
  const mode: PickMode = ctx.isEditableNow(winner) && top.strong ? "move" : "select";
  return { object: winner, mode };
}

/** The CSS cursor for a pick result + the active tool's default fallback. */
export function cursorForPick(pick: PickResult, toolDefault: string): string {
  if (pick.mode === "resize" && pick.handle) {
    return pick.handle === "nw" || pick.handle === "se" ? "nwse-resize" : "nesw-resize";
  }
  if (pick.mode === "move") return "move";
  // A selectable-but-not-editable object (locked / non-active layer): you can
  // still click to inspect it, but the disabled cursor signals you can't edit here.
  if (pick.mode === "select") return "not-allowed";
  return toolDefault;
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

/** The combined [0,1] bbox of several objects (the union of their bboxes). Returns
 *  null for an empty set. Used by the multi-select group MOVE: a press anywhere
 *  inside this box (or on any one selected object) begins dragging the whole set. */
export function combinedBBox(
  objs: AnnotationObject[],
  measure?: TextMeasure,
): { minX: number; minY: number; maxX: number; maxY: number } | null {
  if (objs.length === 0) return null;
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const o of objs) {
    const b = objectBBox(o, measure);
    if (b.minX < minX) minX = b.minX;
    if (b.minY < minY) minY = b.minY;
    if (b.maxX > maxX) maxX = b.maxX;
    if (b.maxY > maxY) maxY = b.maxY;
  }
  if (!Number.isFinite(minX)) return null;
  return { minX, minY, maxX, maxY };
}

/** Would a press at `pt` grab a multi-selection for a group move? True when the
 *  point lands inside the combined bbox (small pad) OR on any one selected
 *  object's hit-region. Independent of per-object editability — the move itself
 *  only translates the editable-now members (the caller enforces that). */
export function hitsMultiSelection(
  pt: { x: number; y: number },
  selected: AnnotationObject[],
  pageW: number,
  pageH: number,
  measure?: TextMeasure,
): boolean {
  if (selected.length < 2) return false;
  // On any individual selected object (strong OR weak) → grab.
  for (const o of selected) {
    if (classifyHit(o, pt.x, pt.y, pageW, pageH, measure) !== "none") return true;
  }
  // Otherwise inside the combined bbox (with a small pad) → grab.
  const b = combinedBBox(selected, measure);
  if (!b) return false;
  return insideBox(b, pt.x, pt.y, 0.004, 0.004);
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

// Bug #2: move vs resize. A body-drag must MOVE; a resize starts ONLY when the press
// is within a small handle zone AT a corner, and handles are SUPPRESSED entirely when
// the object's on-screen bbox is too small to show them safely.
//
// The on-screen handle is 10px (styles.css .resize-handle); the grab zone is a touch
// larger. Handles are hidden unless the bbox is at least ~3× the handle size on BOTH
// axes — below that the corner zones would overlap the body and a move-drag would grab
// a handle (the exact bug). All px values convert to page-relative via the page dims.
export const HANDLE_PX = 10; // on-screen handle square
export const HANDLE_GRAB_PX = 12; // pointer must be within this of a corner to resize
export const HANDLE_MIN_BBOX_FACTOR = 3; // bbox must exceed this × handle to show handles

/** Are the corner resize handles safe to show for a bbox of this on-screen size?
 *  False for a small object → it becomes MOVE-ONLY (handles suppressed). */
export function handlesVisible(
  b: { minX: number; minY: number; maxX: number; maxY: number },
  pageW: number,
  pageH: number,
): boolean {
  if (pageW <= 0 || pageH <= 0) return false;
  const wPx = (b.maxX - b.minX) * pageW;
  const hPx = (b.maxY - b.minY) * pageH;
  const min = HANDLE_PX * HANDLE_MIN_BBOX_FACTOR;
  return wPx >= min && hPx >= min;
}

/** Which corner handle (if any) a press grabs, using a px grab zone — but ONLY when the
 *  bbox is large enough to show handles. Returns null for a small object so its whole
 *  body is a move zone (Bug #2). pageW/pageH convert the px tolerance to [0,1]. */
export function handleAtPx(
  b: { minX: number; minY: number; maxX: number; maxY: number },
  pt: { x: number; y: number },
  pageW: number,
  pageH: number,
): HandleId | null {
  if (!handlesVisible(b, pageW, pageH)) return null;
  const tolX = HANDLE_GRAB_PX / pageW;
  const tolY = HANDLE_GRAB_PX / pageH;
  for (const h of CORNER_HANDLES) {
    const hp = handlePoint(b, h);
    if (Math.abs(pt.x - hp.x) <= tolX && Math.abs(pt.y - hp.y) <= tolY) return h;
  }
  return null;
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
/** Minimum on-screen size (page-relative [0,1]) a resize may shrink an object to, so a
 *  tiny/overshooting drag can never collapse it to a point (Bug #2). */
export const MIN_OBJECT_SIZE = 0.01;

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
  let newMinX = Math.min(fixed.x, newDragged.x);
  let newMaxX = Math.max(fixed.x, newDragged.x);
  let newMinY = Math.min(fixed.y, newDragged.y);
  let newMaxY = Math.max(fixed.y, newDragged.y);
  // Clamp to a minimum size, growing AWAY from the fixed (opposite) corner so the
  // anchor stays put — the drag maps to the bbox EDGE additively, never a multiplier.
  if (newMaxX - newMinX < MIN_OBJECT_SIZE) {
    if (fixed.x <= newMinX) newMaxX = clamp01(fixed.x + MIN_OBJECT_SIZE), (newMinX = fixed.x);
    else newMinX = clamp01(fixed.x - MIN_OBJECT_SIZE), (newMaxX = fixed.x);
  }
  if (newMaxY - newMinY < MIN_OBJECT_SIZE) {
    if (fixed.y <= newMinY) newMaxY = clamp01(fixed.y + MIN_OBJECT_SIZE), (newMinY = fixed.y);
    else newMinY = clamp01(fixed.y - MIN_OBJECT_SIZE), (newMaxY = fixed.y);
  }
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

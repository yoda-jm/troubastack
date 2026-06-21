/**
 * @troubastack/ink — THE single annotation renderer (ARCHITECTURE.md I8).
 *
 * Object geometry + rasterization lives here ONCE and is consumed by both
 * `@troubastack/studio` (browser viewer/editor) and `@troubastack/bake`
 * (Node: server-side transparent overlays). The mobile app's native ink
 * overlay is the ONLY sanctioned re-implementation and must match this output
 * under a golden-image parity test (I8). There is no third copy — Go never
 * draws strokes.
 *
 * This module is framework-free: it only touches a Canvas2D context. It draws
 * one annotation object (freehand / rect / ellipse / line / text / highlight)
 * onto a context given the page's pixel rectangle. All persisted coordinates
 * are PAGE-RELATIVE in [0,1] (I3); pixels are produced only here at draw time.
 *
 * perfect-freehand provides the variable-width freehand outline.
 */

import { getStroke } from "perfect-freehand";

// ---------------------------------------------------------------------------
// Render-facing types
//
// These are the minimal shape this renderer needs to turn an annotation into
// pixels — a VIEW of the canonical wire/domain types (the annotation API in
// core/internal/httpapi/annotations.go). They are intentionally a structural
// match for that wire object so callers can pass API objects directly.
// ---------------------------------------------------------------------------

/** A single point in PAGE-RELATIVE coordinates: x,y in [0,1] (I3). */
export interface InkPoint {
  /** Page-relative X in [0,1]. */
  x: number;
  /** Page-relative Y in [0,1]. */
  y: number;
  /** Optional stylus pressure in [0,1]; drives variable freehand width. */
  pressure?: number;
}

/** Visual style of an object. A render-facing view of the proto style (I1). */
export interface InkStyle {
  /** CSS color string, e.g. "#RRGGBB". */
  color: string;
  /** Opacity in [0,1]. */
  opacity: number;
  /** Stroke width as a FRACTION OF PAGE WIDTH (I3) — scales with zoom for free. */
  width: number;
  /** Font size as a FRACTION OF PAGE HEIGHT (text only). */
  fontSize: number;
  /** Paint the interior (rect/ellipse). Absent → inferred from the object type. */
  fill?: boolean;
  /** Draw the border (rect/ellipse). Absent → inferred from the object type. */
  stroke?: boolean;
  /** Compositing for the fill: "multiply" blends like a highlighter. */
  blend?: "normal" | "multiply";
}

/** The kinds of object this renderer can draw. */
export type InkObjectType =
  | "freehand"
  | "rect"
  | "ellipse"
  | "line"
  | "text"
  | "highlight";

/**
 * One annotation object. `points` are page-relative [0,1]:
 *  - freehand  : the full sampled path
 *  - rect      : [topLeft, bottomRight]
 *  - ellipse   : [topLeft, bottomRight] (bbox)
 *  - highlight : [topLeft, bottomRight] (filled translucent marker)
 *  - line      : [a, b]
 *  - text      : [anchorTopLeft]
 */
export interface InkObject {
  uuid?: string;
  layerId?: string;
  type: InkObjectType;
  points: InkPoint[];
  /** 0-based page index this object belongs to. */
  page?: number;
  /** Text content (text objects). */
  text?: string;
  style: InkStyle;
}

/**
 * Pixel rectangle of the page on the target canvas. The renderer maps
 * page-relative [0,1] coordinates into this rect:
 *   px = x + p.x * w ;  py = y + p.y * h
 */
export interface PageRect {
  x: number;
  y: number;
  w: number;
  h: number;
}

/** Either canvas context flavor — the drawing path never branches on it (I8). */
type Ctx2D = CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;

// ---------------------------------------------------------------------------
// Coordinate helpers
// ---------------------------------------------------------------------------

function toPx(p: InkPoint, page: PageRect): [number, number] {
  return [page.x + p.x * page.w, page.y + p.y * page.h];
}

/** Stroke width in device px: style.width is a fraction of the page width. */
function strokePx(style: InkStyle, page: PageRect): number {
  // Guard a sane minimum so a hairline never disappears.
  return Math.max(0.5, style.width * page.w);
}

/** Font size in device px: style.fontSize is a fraction of the page height. */
function fontPx(style: InkStyle, page: PageRect): number {
  return Math.max(1, style.fontSize * page.h);
}

function clamp01(n: number): number {
  if (Number.isNaN(n)) return 1;
  return n < 0 ? 0 : n > 1 ? 1 : n;
}

// ---------------------------------------------------------------------------
// The public render surface
// ---------------------------------------------------------------------------

/**
 * Draw a single annotation object onto a Canvas2D context, mapping its
 * page-relative [0,1] geometry into the supplied page pixel rect.
 *
 * Always brackets its drawing in ctx.save()/ctx.restore(); honors opacity via
 * globalAlpha; uses round joins/caps for strokes.
 */
export function renderObject(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  if (!obj || !obj.points || obj.points.length === 0) return;

  ctx.save();
  try {
    ctx.globalAlpha = clamp01(obj.style.opacity ?? 1);
    ctx.lineJoin = "round";
    ctx.lineCap = "round";

    switch (obj.type) {
      case "freehand":
        drawFreehand(ctx, obj, page);
        break;
      case "line":
        drawLine(ctx, obj, page);
        break;
      case "rect":
        drawRect(ctx, obj, page);
        break;
      case "ellipse":
        drawEllipse(ctx, obj, page);
        break;
      case "highlight":
        drawHighlight(ctx, obj, page);
        break;
      case "text":
        drawText(ctx, obj, page);
        break;
      default:
        break;
    }
  } finally {
    ctx.restore();
  }
}

/** Convenience: render many objects in array order onto the same page rect. */
export function renderObjects(ctx: Ctx2D, objs: InkObject[], page: PageRect): void {
  for (const o of objs) renderObject(ctx, o, page);
}

// ---------------------------------------------------------------------------
// Per-type drawing
// ---------------------------------------------------------------------------

function drawFreehand(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const w = strokePx(obj.style, page);
  // perfect-freehand wants pixel-space input points; feed pressure if present.
  const input: number[][] = obj.points.map((p) => {
    const [px, py] = toPx(p, page);
    return p.pressure != null ? [px, py, p.pressure] : [px, py];
  });

  const outline = getStroke(input, {
    size: w,
    thinning: 0.5,
    smoothing: 0.5,
    streamline: 0.5,
    simulatePressure: obj.points.every((p) => p.pressure == null),
    last: true,
  });

  if (outline.length === 0) {
    // Degenerate (e.g. single point with no outline) — draw a dot.
    const [px, py] = toPx(obj.points[0], page);
    ctx.beginPath();
    ctx.arc(px, py, Math.max(0.5, w / 2), 0, Math.PI * 2);
    ctx.fillStyle = obj.style.color;
    ctx.fill();
    return;
  }

  ctx.beginPath();
  ctx.moveTo(outline[0][0], outline[0][1]);
  for (let i = 1; i < outline.length; i++) {
    ctx.lineTo(outline[i][0], outline[i][1]);
  }
  ctx.closePath();
  ctx.fillStyle = obj.style.color;
  ctx.fill();
}

function drawLine(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const [ax, ay] = toPx(a, page);
  const [bx, by] = toPx(b, page);
  ctx.beginPath();
  ctx.moveTo(ax, ay);
  ctx.lineTo(bx, by);
  ctx.lineWidth = strokePx(obj.style, page);
  ctx.strokeStyle = obj.style.color;
  ctx.stroke();
}

/** Normalize a two-point bbox into top-left + width/height (handles any order). */
function bbox(obj: InkObject, page: PageRect): { x: number; y: number; w: number; h: number } {
  const [a, b] = obj.points;
  const [ax, ay] = toPx(a, page);
  const [bx, by] = toPx(b, page);
  return {
    x: Math.min(ax, bx),
    y: Math.min(ay, by),
    w: Math.abs(bx - ax),
    h: Math.abs(by - ay),
  };
}

/**
 * Resolve the effective fill/stroke/blend for a shape, applying BACK-COMPAT defaults
 * so objects seeded before these flags existed render identically:
 *   - legacy type "highlight"           → fill, multiply, no stroke;
 *   - rect/ellipse with flags ABSENT    → stroke only, normal (the old outline look);
 *   - rect/ellipse with flags present   → honor them verbatim.
 * `fill` and `stroke` are only treated as defaulted when BOTH are absent (a caller
 * that sets just one — e.g. fill:true — implies the other is false).
 */
function shapeStyle(obj: InkObject): { fill: boolean; stroke: boolean; multiply: boolean } {
  const s = obj.style;
  if (obj.type === "highlight") {
    // Legacy highlighter: filled + multiply + no stroke, unless explicitly overridden.
    return {
      fill: s.fill ?? true,
      stroke: s.stroke ?? false,
      multiply: (s.blend ?? "multiply") === "multiply",
    };
  }
  const hasFlags = s.fill != null || s.stroke != null;
  if (!hasFlags) return { fill: false, stroke: true, multiply: false }; // legacy rect/ellipse
  return { fill: s.fill ?? false, stroke: s.stroke ?? false, multiply: s.blend === "multiply" };
}

/** Paint a shape's interior + border per its resolved fill/stroke/blend. `pathFn`
 *  builds the path (rect or ellipse); fill uses globalAlpha, stroke is always opaque-
 *  for-its-color (alpha already set on the context by renderObject). */
function paintShape(ctx: Ctx2D, obj: InkObject, page: PageRect, pathFn: () => void): void {
  const { fill, stroke, multiply } = shapeStyle(obj);
  if (fill) {
    ctx.save();
    if (multiply) ctx.globalCompositeOperation = "multiply";
    ctx.fillStyle = obj.style.color;
    pathFn();
    ctx.fill();
    ctx.restore();
  }
  if (stroke) {
    ctx.lineWidth = strokePx(obj.style, page);
    ctx.strokeStyle = obj.style.color;
    pathFn();
    ctx.stroke();
  }
}

function drawRect(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const r = bbox(obj, page);
  paintShape(ctx, obj, page, () => {
    ctx.beginPath();
    ctx.rect(r.x, r.y, r.w, r.h);
  });
}

function drawEllipse(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const r = bbox(obj, page);
  const cx = r.x + r.w / 2;
  const cy = r.y + r.h / 2;
  paintShape(ctx, obj, page, () => {
    ctx.beginPath();
    ctx.ellipse(cx, cy, r.w / 2, r.h / 2, 0, 0, Math.PI * 2);
  });
}

// Legacy "highlight" objects route through the same shape painter (shapeStyle infers
// fill+multiply+no-stroke for them), so old seeded data renders exactly as before.
function drawHighlight(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const r = bbox(obj, page);
  paintShape(ctx, obj, page, () => {
    ctx.beginPath();
    ctx.rect(r.x, r.y, r.w, r.h);
  });
}

function drawText(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const anchor = obj.points[0];
  if (!anchor || !obj.text) return;
  const [px, py] = toPx(anchor, page);
  const fpx = fontPx(obj.style, page);
  ctx.font = `${fpx}px system-ui, -apple-system, "Segoe UI", Roboto, sans-serif`;
  ctx.textBaseline = "top";
  ctx.fillStyle = obj.style.color;
  ctx.fillText(obj.text, px, py);
}

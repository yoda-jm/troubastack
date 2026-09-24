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
import { getGlyph } from "./glyphs.js";
import type { ObjectType } from "./objecttype.gen.js";

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
  /** T177 line pattern: applies to a `line` and to the BORDER of a rect/ellipse.
   *  Absent (or "solid") is exactly what every object drawn before T177 renders as. */
  dash?: InkDash;
  /** T177 end decoration — straight `line` only. Absent = an undecorated line. */
  ends?: InkEnds;
}

/** The closed set of line patterns (T177 D2). Deliberately NOT a dash-array: the
 *  editor and the baker rasterize at different pixel sizes, and an open numeric array
 *  is where the two would start to differ. */
export type InkDash = "solid" | "dashed" | "dotted";

/** The decoration on a straight line's ends: WHAT and WHERE, in one record so they
 *  cannot disagree. An empty `head` means the default head (an arrow), the way an
 *  empty blend means "normal"; an UNRECOGNISED head draws nothing (a newer client's
 *  head must not silently render as an arrow — a wrong mark on a chart reads as a
 *  musical instruction). */
export type InkEndShape = "arrow" | "circle" | "square";

/** The decoration at each END of a straight line — one shape per end (T179). This replaces T177's
 *  {head, side}: a "which end?" axis asked the reader a question the drawing gesture already answered
 *  (you finish toward the thing you are pointing at), and one-shape-per-end also expresses the case the
 *  old model could not — a DIFFERENT terminator at each end. Absent, "none", "" and any UNRECOGNISED
 *  name all draw nothing: a shape a newer client names and this renderer lacks must never be substituted
 *  by one we happen to have, because a wrong mark on a chart reads as a musical instruction. */
export interface InkEnds {
  start?: string;
  end?: string;
}

// The wire object-type set is GENERATED from proto ObjectType (objecttype.gen.ts, T09)
// — re-exported so consumers get the proto-authoritative union from ink.
export type { ObjectType } from "./objecttype.gen.js";

/** The kinds of object this renderer can draw: the wire set (proto `ObjectType`) PLUS
 *  the dev-only `"arrow"` (T07 — registered/drawn only when localStorage.devArrow is
 *  set; NOT on the wire until proto adds it, by construction). */
export type InkObjectType = ObjectType | "arrow";

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
export type Ctx2D = CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D;

// ---------------------------------------------------------------------------
// Coordinate helpers
// ---------------------------------------------------------------------------

// T110: exported for unit testing (pure, no ctx).
export function toPx(p: InkPoint, page: PageRect): [number, number] {
  return [page.x + p.x * page.w, page.y + p.y * page.h];
}

/** Stroke width in device px: style.width is a fraction of the page width. */
// T110: exported for unit testing (pure, no ctx).
export function strokePx(style: InkStyle, page: PageRect): number {
  // Guard a sane minimum so a hairline never disappears.
  return Math.max(0.5, style.width * page.w);
}

// ---------------------------------------------------------------------------
// T177 — line pattern + end decorations
//
// Both are expressed in STROKE WIDTHS, never in page units or raw pixels. The bake
// rasterizes a page at a different pixel size from the editor's canvas, so a dash
// measured in px would be a different-looking dash on the stand than on the screen —
// and an arrowhead measured against the page would grow when the page did. Widths are
// the one frame both renderers already agree on (D3: pin it here, once).
// ---------------------------------------------------------------------------

/** Dash period in stroke widths: [on, off]. Round caps (set by renderObject) turn the
 *  zero-length "on" of a dot into a round dot of exactly the stroke's diameter. */
const DASH_UNITS: Record<InkDash, number[]> = {
  solid: [],
  dashed: [3, 2],
  dotted: [0, 2],
};

/** The dash this style asks for, normalized. An unknown name reads as solid — the
 *  same "draw what you understand, nothing more" rule the heads follow. */
export function dashKind(style: InkStyle): InkDash {
  const d = style.dash;
  return d === "dashed" || d === "dotted" ? d : "solid";
}

/** The `setLineDash` array in device px for a style, given its stroke width.
 *  Empty = solid. Pure; exported for unit tests. */
export function dashArray(style: InkStyle, widthPx: number): number[] {
  return DASH_UNITS[dashKind(style)].map((u) => u * widthPx);
}

/** The shape at each end this renderer will draw, or null for a bare end. "", "none" and any name this
 *  renderer does not know all resolve to null — never a substituted shape (T177's refusal rule, kept). A
 *  bare `ends`, or any object that is not a straight line, is two bare ends. */
export function endShapes(obj: InkObject): { start: InkEndShape | null; end: InkEndShape | null } {
  const e = obj.style.ends;
  if (!e || obj.type !== "line") return { start: null, end: null };
  return { start: normEndShape(e.start), end: normEndShape(e.end) };
}

function normEndShape(s: string | undefined): InkEndShape | null {
  return s === "arrow" || s === "circle" || s === "square" ? s : null;
}

/** Axial extent of a terminator, in stroke widths — how far back along the shaft it reaches from the
 *  endpoint. The shaft is trimmed by exactly this so it never runs UNDER a filled terminator, which is the
 *  T179 fix: a shaft drawn tip-to-tip shows through the head as a darker spine at opacity < 1, lets a round
 *  cap poke past an arrow's point, and can land a dash GAP inside the head — a triangle with a hole. Trim
 *  the shaft to the terminator's base and all three cannot happen. */
const END_EXTENT_W: Record<InkEndShape, number> = {
  arrow: 3.2,  // ARROW_HEAD_LEN_W
  circle: 3.0, // diameter
  square: 2.6, // side
};
/** Arrowhead half-width across the shaft, in stroke widths (≈50° included angle). */
export const ARROW_HEAD_HALF_W = 1.5;
/** T177 name kept for callers/tests: the arrowhead's length along the shaft. */
export const ARROW_HEAD_LEN_W = END_EXTENT_W.arrow;

/** A terminator to fill, in device px. §4: every shape sits with its FAR EDGE at the endpoint and its
 *  body reaching INWARD — like the arrow's tip-at-endpoint — so nothing pokes past the line's end and the
 *  drawn length stays honest (Fable, T179 §4). */
export type EndGeom =
  | { shape: "arrow"; tip: [number, number]; left: [number, number]; right: [number, number] }
  | { shape: "circle"; cx: number; cy: number; r: number }
  | { shape: "square"; corners: [number, number][] };

/** The trimmed shaft and the terminators for a line, in device px. PURE — the geometry D3 pins once, so
 *  studio, the bake and any other consumer inherit one answer, including the degenerate ends:
 *
 *   - a ZERO-LENGTH line has no direction, so no terminator and no trim: the shaft's round cap is the dot
 *     the user tapped;
 *   - a line SHORTER than its terminators (a stray tap) shrinks every shape proportionally to the length
 *     available and splits it between two decorated ends, so a terminator is never longer than its line
 *     and two of them never swallow each other. */
export function lineEnds(
  ax: number,
  ay: number,
  bx: number,
  by: number,
  widthPx: number,
  startShape: InkEndShape | null,
  endShape: InkEndShape | null,
): { shaft: { ax: number; ay: number; bx: number; by: number }; terms: EndGeom[] } {
  const bare = { shaft: { ax, ay, bx, by }, terms: [] as EndGeom[] };
  const dx = bx - ax;
  const dy = by - ay;
  const len = Math.hypot(dx, dy);
  if (len === 0) return bare; // no direction → no terminator
  const ux = dx / len;
  const uy = dy / len;
  const px = -uy;
  const py = ux;

  let extS = startShape ? END_EXTENT_W[startShape] * widthPx : 0;
  let extE = endShape ? END_EXTENT_W[endShape] * widthPx : 0;
  // Shrink the WHOLE shape (extent and cross-size together via `scale`) so a clamped terminator is a
  // smaller version of itself, not a needle; split the length when both ends carry one.
  let scale = 1;
  if (extS + extE > len) {
    scale = len / (extS + extE);
    extS *= scale;
    extE *= scale;
  }

  const term = (
    shape: InkEndShape,
    // endpoint (the far edge sits here), and the INWARD unit direction
    tx: number,
    ty: number,
    ix: number,
    iy: number,
    ext: number,
  ): EndGeom => {
    if (shape === "arrow") {
      const half = ARROW_HEAD_HALF_W * widthPx * scale;
      const cx = tx + ix * ext; // base centre, `ext` inward from the tip
      const cy = ty + iy * ext;
      return { shape, tip: [tx, ty], left: [cx + px * half, cy + py * half], right: [cx - px * half, cy - py * half] };
    }
    if (shape === "circle") {
      const r = ext / 2;
      return { shape, cx: tx + ix * r, cy: ty + iy * r, r };
    }
    // square, oriented to the line, far edge at the endpoint
    const half = ext / 2;
    const cx = tx + ix * half;
    const cy = ty + iy * half;
    const corners: [number, number][] = [
      [cx + ix * half + px * half, cy + iy * half + py * half],
      [cx + ix * half - px * half, cy + iy * half - py * half],
      [cx - ix * half - px * half, cy - iy * half - py * half],
      [cx - ix * half + px * half, cy - iy * half + py * half],
    ];
    return { shape, corners };
  };

  const terms: EndGeom[] = [];
  if (endShape) terms.push(term(endShape, bx, by, -ux, -uy, extE));
  if (startShape) terms.push(term(startShape, ax, ay, ux, uy, extS));

  return {
    // Trim the shaft inward by each decorated end's extent, so it meets the terminator's base and never
    // runs under it.
    shaft: { ax: ax + ux * extS, ay: ay + uy * extS, bx: bx - ux * extE, by: by - uy * extE },
    terms,
  };
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
/** A per-type draw function. */
export type InkDrawFn = (ctx: Ctx2D, obj: InkObject, page: PageRect) => void;

// The ONE dispatch point: type → draw function. ink registers its built-ins at
// module load (below); studio may register extra types (e.g. the T07 arrow demo)
// via registerInkDraw at startup, so a new type needs NO edit here and ink stays
// dependency-free (it never imports studio). Replaces the old switch — there is
// no `switch (obj.type)` anywhere now.
const drawRegistry = new Map<string, InkDrawFn>();

/** Register (or override) the draw function for an object type. */
export function registerInkDraw(type: string, fn: InkDrawFn): void {
  drawRegistry.set(type, fn);
}

export function renderObject(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  if (!obj || !obj.points || obj.points.length === 0) return;
  const draw = drawRegistry.get(obj.type);
  if (!draw) return;

  ctx.save();
  try {
    ctx.globalAlpha = clamp01(obj.style.opacity ?? 1);
    ctx.lineJoin = "round";
    ctx.lineCap = "round";
    draw(ctx, obj, page);
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

export function drawFreehand(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
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

export function drawLine(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const [ax, ay] = toPx(a, page);
  const [bx, by] = toPx(b, page);
  const w = strokePx(obj.style, page);

  const { start, end } = endShapes(obj);
  const { shaft, terms } = lineEnds(ax, ay, bx, by, w, start, end);

  // The shaft is dashed as before, but drawn only between the terminators' bases (T179) so it never runs
  // under a filled terminator — no darker spine at opacity < 1, no round cap poking past an arrow point,
  // no dash gap landing inside a head.
  ctx.beginPath();
  ctx.moveTo(shaft.ax, shaft.ay);
  ctx.lineTo(shaft.bx, shaft.by);
  ctx.lineWidth = w;
  ctx.strokeStyle = obj.style.color;
  ctx.setLineDash(dashArray(obj.style, w));
  ctx.stroke();

  if (terms.length === 0) return;
  // Terminators are FILLED and UNDASHED: at a chart's stroke widths a stroked shape doubles its own weight
  // and aliases its joins, while a filled one matches the shaft's darkness exactly; a dashed terminator is
  // a pattern artifact nobody picks. Opacity is the object's, already on ctx.globalAlpha (renderObject).
  ctx.setLineDash([]);
  ctx.fillStyle = obj.style.color;
  for (const t of terms) {
    ctx.beginPath();
    if (t.shape === "circle") {
      ctx.arc(t.cx, t.cy, t.r, 0, Math.PI * 2);
    } else {
      const pts = t.shape === "arrow" ? [t.tip, t.left, t.right] : t.corners;
      ctx.moveTo(pts[0][0], pts[0][1]);
      for (let i = 1; i < pts.length; i++) ctx.lineTo(pts[i][0], pts[i][1]);
      ctx.closePath();
    }
    ctx.fill();
  }
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

/**
 * Build the FILL path for a shape — the full bbox interior. Used both for the
 * direct fill and, on the composite path, the offscreen fill.
 */
type ShapeGeom = {
  /** Append the interior (full bbox) path to `c`'s current sub-path list. */
  fillPath: (c: Ctx2D) => void;
  /** Append the INSET border path (shrunk by half the line width) to `c`. */
  strokePath: (c: Ctx2D, lineWidth: number) => void;
};

function rectGeom(r: { x: number; y: number; w: number; h: number }): ShapeGeom {
  return {
    fillPath: (c) => {
      c.beginPath();
      c.rect(r.x, r.y, r.w, r.h);
    },
    // Canvas strokes are CENTERED on the path, so half the line width spills
    // each way. Shrink the rect by half the line width per side so the border
    // grows INWARD and never leaks past the object's bbox. Clamp the inset so a
    // very large width can't invert the rectangle (size stays ≥ 0).
    strokePath: (c, lineWidth) => {
      const inset = lineWidth / 2;
      const x = r.x + inset;
      const y = r.y + inset;
      const w = Math.max(0, r.w - lineWidth);
      const h = Math.max(0, r.h - lineWidth);
      c.beginPath();
      c.rect(x, y, w, h);
    },
  };
}

function ellipseGeom(r: { x: number; y: number; w: number; h: number }): ShapeGeom {
  const cx = r.x + r.w / 2;
  const cy = r.y + r.h / 2;
  return {
    fillPath: (c) => {
      c.beginPath();
      c.ellipse(cx, cy, r.w / 2, r.h / 2, 0, 0, Math.PI * 2);
    },
    // Same inset reasoning as rect: pull the radii in by half the line width so
    // the centered stroke stays inside the bbox. Clamp radii ≥ 0.
    strokePath: (c, lineWidth) => {
      const inset = lineWidth / 2;
      const rx = Math.max(0, r.w / 2 - inset);
      const ry = Math.max(0, r.h / 2 - inset);
      c.beginPath();
      c.ellipse(cx, cy, rx, ry, 0, 0, Math.PI * 2);
    },
  };
}

/** The target ctx's current transform (for matching scale in the offscreen),
 *  or null if unavailable. We only read the axis scales (a, d). */
function getCtxTransform(ctx: Ctx2D): { a: number; d: number } | null {
  const g = ctx as { getTransform?: () => DOMMatrix };
  if (typeof g.getTransform !== "function") return null;
  try {
    const m = g.getTransform();
    return { a: m.a, d: m.d };
  } catch {
    return null;
  }
}

/** Allocate an OffscreenCanvas/HTMLCanvas-agnostic 2D context for compositing. */
function makeOffscreen(w: number, h: number): { ctx: Ctx2D; canvas: CanvasImageSource } | null {
  const iw = Math.max(1, Math.ceil(w));
  const ih = Math.max(1, Math.ceil(h));
  if (typeof OffscreenCanvas !== "undefined") {
    const oc = new OffscreenCanvas(iw, ih);
    const c = oc.getContext("2d");
    if (!c) return null;
    return { ctx: c, canvas: oc };
  }
  if (typeof document !== "undefined") {
    const el = document.createElement("canvas");
    el.width = iw;
    el.height = ih;
    const c = el.getContext("2d");
    if (!c) return null;
    return { ctx: c, canvas: el };
  }
  return null;
}

/**
 * Paint a shape's interior + border per its resolved fill/stroke/blend.
 *
 * Two paths:
 *  - FILL-ONLY or STROKE-ONLY (incl. Highlight): the simple direct path — paint
 *    straight onto `ctx` at the opacity/blend already set by renderObject. This
 *    keeps every legacy look (outline rect/ellipse, Highlight multiply) byte-for-byte.
 *  - FILL **and** STROKE (the "Box"): render the OPAQUE fill + OPAQUE inset stroke
 *    onto an offscreen canvas first, then blit the result onto `ctx` ONCE at the
 *    object's opacity (and blend). The shape then reads as a single uniform-opacity
 *    mark — no double-blended (darker) rim where fill and stroke overlap.
 */
function paintShape(ctx: Ctx2D, obj: InkObject, page: PageRect, geom: ShapeGeom): void {
  const { fill, stroke, multiply } = shapeStyle(obj);
  const color = obj.style.color;
  const lineWidth = strokePx(obj.style, page);

  // --- Combined fill+stroke: single-opacity offscreen composite. ----------
  if (fill && stroke) {
    const opacity = clamp01(obj.style.opacity ?? 1);
    // The target ctx may carry a scale transform (callers scale by devicePixel-
    // Ratio so [0,1] coords map to logical CSS px while the backing store is at
    // device resolution). Allocate the offscreen at DEVICE resolution and apply
    // the SAME scale, so the composite is as crisp as a direct draw, then blit it
    // back at the matching device pixels. Only the shape's bbox is composited, so
    // the (multiply) blend touches only the shape — never the whole page.
    const m = getCtxTransform(ctx);
    const sx = m ? m.a : 1;
    const sy = m ? m.d : 1;
    const r = bbox(obj, page);
    // Offscreen origin in LOGICAL space (floor for a stable integer device map).
    const ox = Math.floor(r.x);
    const oy = Math.floor(r.y);
    // Device-pixel offscreen size, padded by 1 logical px each side for AA.
    const dox = Math.floor(ox * sx);
    const doy = Math.floor(oy * sy);
    const off = makeOffscreen((r.w + 2) * sx, (r.h + 2) * sy);
    if (off) {
      const o = off.ctx;
      o.save();
      // Replicate the target's scale; map logical (ox,oy) → offscreen (0,0).
      o.setTransform(sx, 0, 0, sy, -dox, -doy);
      o.lineJoin = "round";
      o.lineCap = "round";
      // Paint OPAQUE onto the offscreen; opacity is applied once on the blit.
      o.fillStyle = color;
      geom.fillPath(o);
      o.fill();
      o.lineWidth = lineWidth;
      o.strokeStyle = color;
      o.setLineDash(dashArray(obj.style, lineWidth));
      geom.strokePath(o, lineWidth);
      o.stroke();
      o.restore();

      ctx.save();
      // Blit in DEVICE space (identity) so the offscreen's device pixels land
      // 1:1 on the target's backing store — no resampling, no edge spread.
      ctx.setTransform(1, 0, 0, 1, 0, 0);
      ctx.globalAlpha = opacity;
      if (multiply) ctx.globalCompositeOperation = "multiply";
      (ctx as CanvasRenderingContext2D).drawImage(off.canvas as CanvasImageSource, dox, doy);
      ctx.restore();
      return;
    }
    // Offscreen unavailable (no OffscreenCanvas/document): fall through to the
    // direct path below so the shape still renders (with the old double-blend).
  }

  // --- Direct path: fill-only, stroke-only, or composite fallback. --------
  if (fill) {
    ctx.save();
    if (multiply) ctx.globalCompositeOperation = "multiply";
    ctx.fillStyle = color;
    geom.fillPath(ctx);
    ctx.fill();
    ctx.restore();
  }
  if (stroke) {
    ctx.lineWidth = lineWidth;
    ctx.strokeStyle = color;
    ctx.setLineDash(dashArray(obj.style, lineWidth));
    // Inset the border so the centered stroke stays inside the bbox.
    geom.strokePath(ctx, lineWidth);
    ctx.stroke();
  }
}

export function drawRect(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  paintShape(ctx, obj, page, rectGeom(bbox(obj, page)));
}

export function drawEllipse(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  paintShape(ctx, obj, page, ellipseGeom(bbox(obj, page)));
}

// Legacy "highlight" objects route through the same shape painter (shapeStyle infers
// fill+multiply+no-stroke for them), so old seeded data renders exactly as before.
export function drawHighlight(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  paintShape(ctx, obj, page, rectGeom(bbox(obj, page)));
}

// T51 — icon stamp: a tinted glyph placed in page space. The glyph id rides in
// `obj.text` (an icon's "content" IS its glyph id); geometry comes from the shared
// glyphs.json (getGlyph, unknown id → `note` fallback). The 1×1 glyph is fit into the
// object's bbox preserving aspect + centered, filled and stroked in `style.color`
// (opacity handled by renderObject's globalAlpha). Studio and the bake share this fn.
export function drawIcon(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const [a, b] = obj.points;
  if (!a || !b) return;
  const g = getGlyph(obj.text ?? "");
  const box = bbox(obj, page);
  const s = Math.min(box.w, box.h);
  if (s <= 0) return;
  const ox = box.x + (box.w - s) / 2;
  const oy = box.y + (box.h - s) / 2;
  const trace = (poly: number[][]) => {
    for (let i = 0; i < poly.length; i++) {
      const px = ox + poly[i][0] * s;
      const py = oy + poly[i][1] * s;
      if (i === 0) ctx.moveTo(px, py);
      else ctx.lineTo(px, py);
    }
  };
  ctx.fillStyle = obj.style.color;
  ctx.strokeStyle = obj.style.color;
  ctx.lineWidth = Math.max(0.5, g.strokeWidth * s);
  for (const poly of g.fills) {
    ctx.beginPath();
    trace(poly);
    ctx.closePath();
    ctx.fill();
  }
  for (const poly of g.strokes) {
    ctx.beginPath();
    trace(poly);
    ctx.stroke();
  }
}

// Text font family. The DEFAULT is studio's on-screen stack (unchanged) — browsers
// resolve `system-ui` to the OS UI font, which is right for the editor. But that
// makes text NON-DETERMINISTIC across renderers: the bake worker (Node/Skia) and a
// headless browser resolve `system-ui` to different fonts, so the I8 parity test
// could never converge on text. So the family is configurable: bake registers a
// BUNDLED font and calls setTextFontFamily() with it, and the parity harness loads
// that same font in the browser and does likewise — identical glyph outlines on both
// Skia builds. Studio never calls it, so its rendering is byte-for-byte as before.
let textFontFamily = 'system-ui, -apple-system, "Segoe UI", Roboto, sans-serif';

/**
 * Override the CSS font-family used for text objects (the part after the size in
 * `ctx.font`). Pass a family that resolves to the SAME font file on every renderer
 * you need to match under I8 (see the note above). Studio leaves this at its default.
 */
export function setTextFontFamily(family: string): void {
  textFontFamily = family;
}

/** The font-family currently used for text draws (for callers that mirror it). */
export function getTextFontFamily(): string {
  return textFontFamily;
}

export function drawText(ctx: Ctx2D, obj: InkObject, page: PageRect): void {
  const anchor = obj.points[0];
  if (!anchor || !obj.text) return;
  const [px, py] = toPx(anchor, page);
  const fpx = fontPx(obj.style, page);
  ctx.font = `${fpx}px ${textFontFamily}`;
  ctx.textBaseline = "top";
  ctx.fillStyle = obj.style.color;
  ctx.fillText(obj.text, px, py);
}

// ---------------------------------------------------------------------------
// Built-in dispatch registration — ink & bake render these standalone.
// ---------------------------------------------------------------------------
registerInkDraw("freehand", drawFreehand);
registerInkDraw("line", drawLine);
registerInkDraw("rect", drawRect);
registerInkDraw("ellipse", drawEllipse);
registerInkDraw("highlight", drawHighlight);
registerInkDraw("text", drawText);
registerInkDraw("icon", drawIcon);

// T50/T51 — the shared cue/stamp glyph geometry (generated contract; see src/glyphs.ts).
export {
  getGlyph,
  resolveGlyphId,
  GLYPH_IDS,
  CUE_GLYPH_IDS,
  LANDMARK_GLYPH_IDS,
  FALLBACK_GLYPH_ID,
  type Glyph,
} from "./glyphs.js";

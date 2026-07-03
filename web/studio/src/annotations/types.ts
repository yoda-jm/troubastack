/**
 * Annotation-type descriptor registry (T07). One descriptor per drawable/legacy
 * type collects that type's already-existing per-type functions; editor.ts and
 * SongEditor dispatch through the registry instead of scattered `if (obj.type…)`
 * ladders. Adding a type = one descriptor file + one registry entry (+ the wire
 * string in ink's InkObjectType and, later, proto/Go per T09).
 *
 * Import discipline (keeps the editor↔registry↔descriptors cycle safe): a
 * descriptor may import only LEAF helpers from editor.ts (pure geometry, no
 * registry use) and the generic helpers below — never editor's dispatchers
 * (objectBBox/classifyHit/resizeObject/…), which are the functions that call the
 * registry. Everything here is hoisted functions / object literals, so the
 * cyclic import resolves fine at runtime.
 */
import type { ReactNode } from "react";
import type { AnnotationObject } from "../api";
import type { InkObjectType, InkDrawFn } from "@troubastack/ink";
import {
  type DrawTool,
  type HandleId,
  type TextMeasure,
  type HitStrength,
  clamp01,
  MIN_OBJECT_SIZE,
} from "../editor";

export type Pt = { x: number; y: number };
export type BBox = { minX: number; minY: number; maxX: number; maxY: number };

/** The style controls a type exposes in the toolbar (drives contextual visibility). */
export type StyleControl =
  | "color"
  | "width"
  | "opacity"
  | "shapePreset"
  | "fillBorder"
  | "blend"
  | "textSize";

/** Toolbar presence for a drawable type. Absent → render-only (legacy highlight). */
export interface ToolDef {
  id: DrawTool;
  label: string;
  icon: ReactNode;
  /** edit-canvas cursor class suffix, e.g. "tool-freehand". */
  cursorClass: string;
}

export interface AnnotationTypeDescriptor {
  type: InkObjectType;
  tool?: ToolDef;
  /** ink per-type draw fn (registered into ink at startup — see registry.ts). */
  draw: InkDrawFn;
  styleControls: StyleControl[];
  /** Wire points for an in-progress gesture (was pointsForTool). */
  pointsForGesture(path: Pt[]): Pt[];
  /** Big enough to be a real object, not a stray click (was isMeaningfulGesture). */
  isMeaningfulGesture(path: Pt[]): boolean;
  /** [0,1] bounding box (was the per-type objectBBox). */
  bbox(obj: AnnotationObject, measure?: TextMeasure): BBox;
  /** Point-vs-object hit strength (was the classifyHit per-type branch). */
  classifyHit(
    obj: AnnotationObject,
    x: number,
    y: number,
    pageW: number,
    pageH: number,
    measure?: TextMeasure,
  ): HitStrength;
  /** Resize by dragging a corner handle (was the resizeObject per-type logic). */
  resize(
    obj: AnnotationObject,
    handle: HandleId,
    dx: number,
    dy: number,
    measure?: TextMeasure,
  ): AnnotationObject;
}

// ---- generic per-type building blocks (shared by most descriptors) --------

export const pointsFull = (path: Pt[]): Pt[] => path;
export const pointsStartEnd = (path: Pt[]): Pt[] =>
  path.length === 0 ? [] : [path[0], path[path.length - 1]];
export const pointsAnchor = (path: Pt[]): Pt[] => (path.length === 0 ? [] : [path[0]]);

/** A two-endpoint drag is meaningful when its span exceeds ~0.5% of the page. */
export function meaningfulSpan(path: Pt[]): boolean {
  if (path.length < 2) return false;
  const a = path[0];
  const b = path[path.length - 1];
  return Math.hypot(b.x - a.x, b.y - a.y) > 0.005;
}

/** Point-extent bbox (every non-text type). Text has its own measured bbox. */
export function genericBBox(obj: AnnotationObject): BBox {
  let minX = Infinity,
    minY = Infinity,
    maxX = -Infinity,
    maxY = -Infinity;
  for (const p of obj.points) {
    if (p.x < minX) minX = p.x;
    if (p.y < minY) minY = p.y;
    if (p.x > maxX) maxX = p.x;
    if (p.y > maxY) maxY = p.y;
  }
  if (!Number.isFinite(minX)) return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
  return { minX, minY, maxX, maxY };
}

/** The shared bbox→new-bbox math for a corner-handle resize (from resizeObject).
 *  Grows away from the fixed opposite corner and clamps to MIN_OBJECT_SIZE. */
export function resizeBox(
  b: BBox,
  handle: HandleId,
  dx: number,
  dy: number,
): { minX: number; minY: number; maxX: number; maxY: number; sx: number; sy: number } {
  const corner = (h: HandleId) => ({
    x: h === "nw" || h === "sw" ? b.minX : b.maxX,
    y: h === "nw" || h === "ne" ? b.minY : b.maxY,
  });
  const opp: Record<HandleId, HandleId> = { nw: "se", ne: "sw", se: "nw", sw: "ne" };
  const fixed = corner(opp[handle]);
  const dragged = corner(handle);
  const nd = { x: clamp01(dragged.x + dx), y: clamp01(dragged.y + dy) };
  const oldW = b.maxX - b.minX;
  const oldH = b.maxY - b.minY;
  let minX = Math.min(fixed.x, nd.x);
  let maxX = Math.max(fixed.x, nd.x);
  let minY = Math.min(fixed.y, nd.y);
  let maxY = Math.max(fixed.y, nd.y);
  if (maxX - minX < MIN_OBJECT_SIZE) {
    if (fixed.x <= minX) (maxX = clamp01(fixed.x + MIN_OBJECT_SIZE)), (minX = fixed.x);
    else (minX = clamp01(fixed.x - MIN_OBJECT_SIZE)), (maxX = fixed.x);
  }
  if (maxY - minY < MIN_OBJECT_SIZE) {
    if (fixed.y <= minY) (maxY = clamp01(fixed.y + MIN_OBJECT_SIZE)), (minY = fixed.y);
    else (minY = clamp01(fixed.y - MIN_OBJECT_SIZE)), (maxY = fixed.y);
  }
  const sx = oldW > 1e-6 ? (maxX - minX) / oldW : 1;
  const sy = oldH > 1e-6 ? (maxY - minY) / oldH : 1;
  return { minX, minY, maxX, maxY, sx, sy };
}

/** Generic resize: remap every point from the old bbox into the new one. */
export function genericResize(
  obj: AnnotationObject,
  handle: HandleId,
  dx: number,
  dy: number,
): AnnotationObject {
  const b = genericBBox(obj);
  const r = resizeBox(b, handle, dx, dy);
  return {
    ...obj,
    points: obj.points.map((p) => ({
      x: clamp01(r.minX + (p.x - b.minX) * r.sx),
      y: clamp01(r.minY + (p.y - b.minY) * r.sy),
    })),
  };
}

/** Strong (inside the shape + a small move-margin) / weak (proximity pad) / none,
 *  for a filled-box shape (rect and legacy highlight). Factored out so both share it. */
export function filledBoxClassify(
  b: BBox,
  x: number,
  y: number,
  pageW: number,
  pageH: number,
): HitStrength {
  const mX = pageW > 0 ? 6 / pageW : 0.004;
  const mY = pageH > 0 ? 6 / pageH : 0.004;
  const inside = (px: number, py: number) =>
    x >= b.minX - px && x <= b.maxX + px && y >= b.minY - py && y <= b.maxY + py;
  if (inside(mX, mY)) return "strong";
  if (inside(0.02, 0.02)) return "weak";
  return "none";
}

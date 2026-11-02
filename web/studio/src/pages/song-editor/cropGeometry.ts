// T198 crop geometry — pure functions for the crop window, factored out of CropOverlay so the policy-A
// aspect lock (the rect is a SQUARE in page-fraction space: w == h, giving it the page's aspect in pixels —
// a pure zoom) is unit-testable and a sabotage turns the test red.
//
// All rects are page fractions, origin top-left. The window "never leaves the real PDF page": a drag that
// would push it out is CLAMPED, never refused. T199 adds rotation about the window centre; U1 is angle 0, so
// "all corners in [0,1]" reduces to "the box in [0,1]" — the clamp is written so T199 generalises it.

import type { CropRect, RehearsalNote } from "../../api";

export type Rect = { x: number; y: number; w: number; h: number };

export const MIN_SIDE = 0.2; // matches the server's minCropSide — a crop cannot be a sliver

// T199 deskew angle, in degrees. + is a counter-clockwise CORRECTION (matches the crop model). The server
// validates |angle| <= MAX_ANGLE and rounds to ANGLE_STEP; the client snaps the same way so the field never
// shows a value the save would quietly change.
export const MAX_ANGLE = 10;
export const ANGLE_STEP = 0.05;

// snapAngle clamps to +/-MAX_ANGLE and snaps to the ANGLE_STEP grid, returning a clean 2-decimal number (grid
// values never need more). A non-finite input is 0.
export function snapAngle(deg: number): number {
  if (!Number.isFinite(deg)) return 0;
  const clamped = Math.max(-MAX_ANGLE, Math.min(MAX_ANGLE, deg));
  return Number((Math.round(clamped / ANGLE_STEP) * ANGLE_STEP).toFixed(2));
}

export function clamp01(v: number): number {
  return Math.max(0, Math.min(1, v));
}

// squareAt builds a w==h square anchored at a FIXED corner (ax,ay) reaching toward a dragged point (px,py):
// the side is the larger axis delta, clamped so the square stays on-page and is at least MIN_SIDE.
export function squareAt(ax: number, ay: number, px: number, py: number): Rect {
  const signX = px >= ax ? 1 : -1;
  const signY = py >= ay ? 1 : -1;
  let side = Math.max(Math.abs(px - ax), Math.abs(py - ay));
  const roomX = signX > 0 ? 1 - ax : ax;
  const roomY = signY > 0 ? 1 - ay : ay;
  side = Math.min(side, roomX, roomY);
  side = Math.max(side, MIN_SIDE);
  const x0 = signX > 0 ? ax : ax - side;
  const y0 = signY > 0 ? ay : ay - side;
  return clampSquareOnPage({ x: x0, y: y0, w: side, h: side });
}

// moveRect translates a rect by (dx,dy), clamped so it stays fully on-page (its side unchanged).
export function moveRect(r: Rect, dx: number, dy: number): Rect {
  return clampSquareOnPage({ x: r.x + dx, y: r.y + dy, w: r.w, h: r.h });
}

// clampSquareOnPage shifts a square inside [0,1] without shrinking it (the "never leaves the page" rule). If
// the square is larger than the page it is capped to the page. For T199 this becomes a rotated-corner clamp.
export function clampSquareOnPage(r: Rect): Rect {
  let side = Math.min(r.w, 1); // a square can be at most the shorter page dimension (1 in fractions)
  side = Math.max(side, Math.min(MIN_SIDE, 1));
  let x = r.x;
  let y = r.y;
  if (x < 0) x = 0;
  if (y < 0) y = 0;
  if (x + side > 1) x = 1 - side;
  if (y + side > 1) y = 1 - side;
  return { x, y, w: side, h: side };
}

// defaultRect centres a square of the given side on the page — the rect a fresh "crop this page" starts from
// when the user has not drawn one.
export function defaultRect(side = 0.9): Rect {
  return clampSquareOnPage({ x: (1 - side) / 2, y: (1 - side) / 2, w: side, h: side });
}

// applyPageAngle computes a page's rect after setting its deskew angle to `deg` (snapped). `prev` is the page's
// current draft rect (undefined = uncropped: a straighten gets a full-page 0,0,1,1 window). `saved` is the
// page's SAVED rect. Returns null when the entry should be DROPPED: the angle is cleared back to 0 on a
// synthetic full-page box that was never saved — leaving it would re-raster the page (pushing its notes to
// "page changed") for a crop that trims nothing. A real crop, or a 0,0,1,1 the user actually saved, is kept.
export function applyPageAngle(prev: CropRect | undefined, saved: CropRect | undefined, deg: number): CropRect | null {
  const a = snapAngle(deg);
  const base = prev ?? { x: 0, y: 0, w: 1, h: 1, source: "manual" };
  const rect: CropRect = { x: base.x, y: base.y, w: base.w, h: base.h, source: base.source };
  if (a !== 0) rect.angle = a;
  const isFullPage = rect.x === 0 && rect.y === 0 && rect.w === 1 && rect.h === 1;
  if (a === 0 && isFullPage && saved == null) return null;
  return rect;
}

// sameRect: would two crop rects bake the same page? Compares x,y,w,h and the deskew angle (source is
// irrelevant to the bake). Both absent (no crop) counts as same; one absent is a change.
export function sameRect(a?: CropRect, b?: CropRect): boolean {
  if (!a && !b) return true;
  if (!a || !b) return false;
  return a.x === b.x && a.y === b.y && a.w === b.w && a.h === b.h && (a.angle ?? 0) === (b.angle ?? 0);
}

// notesNewlyChanged (T199) counts how many of the signed-in user's rehearsal notes THIS crop edit would newly
// push to "page changed" on the tablet. A note counts when: it is RESOLVED to the file being cropped (fileId +
// filePage known), its page's DRAFT rect differs from the SAVED rect (angle included), and it is not ALREADY
// page-changed. An unresolved note (no fileId) is never counted — we say nothing rather than guess. Notes are
// owner-scoped, so this is only the cropper's own notes. Pure, so it is unit-tested directly.
export function notesNewlyChanged(
  notes: RehearsalNote[],
  fileId: string,
  draft: Record<string, CropRect>,
  saved: Record<string, CropRect>,
): number {
  let n = 0;
  for (const note of notes) {
    if (note.fileId == null || note.filePage == null) continue; // unresolved → say nothing
    if (note.fileId !== fileId) continue; // a note on another file
    if (note.pageChanged === true) continue; // already page-changed — nothing new
    const key = String(note.filePage);
    if (sameRect(draft[key], saved[key])) continue; // this page's bake area is unchanged
    n++;
  }
  return n;
}

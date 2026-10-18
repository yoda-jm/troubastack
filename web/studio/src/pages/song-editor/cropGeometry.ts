// T198 crop geometry — pure functions for the crop window, factored out of CropOverlay so the policy-A
// aspect lock (the rect is a SQUARE in page-fraction space: w == h, giving it the page's aspect in pixels —
// a pure zoom) is unit-testable and a sabotage turns the test red.
//
// All rects are page fractions, origin top-left. The window "never leaves the real PDF page": a drag that
// would push it out is CLAMPED, never refused. T199 adds rotation about the window centre; U1 is angle 0, so
// "all corners in [0,1]" reduces to "the box in [0,1]" — the clamp is written so T199 generalises it.

export type Rect = { x: number; y: number; w: number; h: number };

export const MIN_SIDE = 0.2; // matches the server's minCropSide — a crop cannot be a sliver

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

/**
 * Pure viewport/page LAYOUT geometry, shared by the app (the beat frame, the icon palette) and the
 * e2e unit tests. It lives in its own compile unit (tsconfig.contract) so a spec can import and
 * unit-test it directly, without pulling an app-owned file into the e2e TS project — the same
 * project-boundary reason `beatPhase` has its own unit (T85/T88). No DOM, no React: just arithmetic.
 */
export interface Edges {
  left: number;
  top: number;
  right: number;
  bottom: number;
}

/**
 * T85b — the beat frame's box: hug the PAGE (± `gap`) on each side, but never past the VIEWPORT.
 * Per-side `min`/`max` means a wide monitor keeps the rail next to the music, while any side
 * scrolled off-screen (zoomed in) falls back to the viewport edge — the beat stays framed.
 */
export function frameBox(page: Edges, viewport: Edges, gap: number): Edges {
  return {
    left: Math.max(page.left - gap, viewport.left),
    top: Math.max(page.top - gap, viewport.top),
    right: Math.min(page.right + gap, viewport.right),
    bottom: Math.min(page.bottom + gap, viewport.bottom),
  };
}

/**
 * T88 — the icon palette's left edge (viewport coordinates). Preferred: just OUTSIDE the page's left
 * edge (`page.left - gap - paletteWidth`), so it sits beside the score on a wide monitor. Floor:
 * `viewport.left + margin`, so when zoom pushes the page to/past the viewport edge the palette clamps
 * on-screen (overlapping the page — intended; the palette is click-through except on its buttons).
 * The same one-axis clamp as `frameBox`. Never returns NaN or a negative left for a degenerate viewport.
 */
export function iconPaletteLeft(
  page: Edges,
  viewport: Edges,
  paletteWidth: number,
  gap: number,
  margin: number,
): number {
  const preferred = page.left - gap - paletteWidth;
  const floor = viewport.left + margin;
  const left = Math.max(preferred, floor);
  return Number.isFinite(left) ? Math.max(0, left) : Math.max(0, margin);
}

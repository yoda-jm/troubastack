/**
 * T84 — stroke width as a GEOMETRIC ladder.
 *
 * Width is a fraction of page width (I3). The old control was linear (min 0.001 / max 0.02 /
 * step 0.001): the bottom decade (0.001→0.010, a 10× range) was crammed into a few notches while the
 * top half spanned only 2×, so most notches felt inert — and the 0.02 ceiling was only ~1.08× chart
 * body text, leaving no headroom over large-print PDFs.
 *
 * Here each notch is a constant RATIO, generated from (FIRST, RATIO, COUNT) so the ratio is exact by
 * construction. The slider's position is an INDEX into WIDTH_STOPS, not the raw width — which is what
 * lets a stored off-table width (e.g. a legacy 0.0037) keep its exact value while the slider merely
 * shows the nearest stop (T84 §4).
 */
const FIRST = 0.0008; // ≈0.17 mm on A4 — a true hairline for dense engravings
const RATIO = 1.3; // ~+30% per notch: a step feels the same anywhere on the slider
const COUNT = 17; // last ≈ 0.0533 (≈11.2 mm) — a genuine marker, ~2.7× the old max

export const WIDTH_STOPS: readonly number[] = Array.from({ length: COUNT }, (_, i) => FIRST * RATIO ** i);

// A4 page width; width is a fraction of it, so physical stroke width (mm) = width * PAGE_WIDTH_MM.
export const PAGE_WIDTH_MM = 210;

/** Physical stroke width in mm for a given page-fraction width (for the human-readable label). */
export function widthToMm(width: number): number {
  return width * PAGE_WIDTH_MM;
}

/**
 * The nearest stop INDEX for an arbitrary stored width — used only for the slider's POSITION. It
 * never rewrites the stored width: an off-table value (legacy charts use 0.003–0.005) renders and
 * persists unchanged; the stored value changes only when the user actually moves the slider (§4).
 */
export function nearestStopIndex(width: number): number {
  let best = 0;
  let bestDelta = Infinity;
  for (let i = 0; i < WIDTH_STOPS.length; i++) {
    const delta = Math.abs(WIDTH_STOPS[i] - width);
    if (delta < bestDelta) {
      bestDelta = delta;
      best = i;
    }
  }
  return best;
}

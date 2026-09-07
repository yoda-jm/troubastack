/**
 * Text size as a DISCRETE ladder — VLL: a slider couldn't reliably land on a value ("font size 8 and some
 * others cannot be selected"), so the control is a dropdown, one option per stop.
 *
 * fontSize is a FRACTION OF PAGE HEIGHT (matches web/ink's fontPx and editor.ts fontToPx). The number the
 * user sees — and the option label — is `fontSize * 1000` (so the default 0.03 reads "30"). The ladder runs
 * smaller than the old slider's floor (0.015 = "15") so small captions like "8" are now reachable.
 */
export const FONT_LABELS: readonly number[] = [8, 10, 12, 14, 16, 18, 20, 24, 28, 30, 36, 42, 48, 56, 64, 72, 80];

/** fontSize (page-height fraction) for each label, e.g. label 8 → 0.008. */
export const FONT_STOPS: readonly number[] = FONT_LABELS.map((n) => n / 1000);

/** The label (the number the user picks/sees) for a page-height-fraction font size. */
export function fontToLabel(fontSize: number): number {
  return Math.round(fontSize * 1000);
}

/**
 * The nearest stop INDEX for an arbitrary stored fontSize — used only for the dropdown's shown value. It
 * never rewrites the stored size: an off-ladder value (a legacy annotation, or one resized freehand)
 * renders and persists unchanged; the value changes only when the user actually picks a stop.
 */
export function nearestFontStopIndex(fontSize: number): number {
  let best = 0;
  let bestDelta = Infinity;
  for (let i = 0; i < FONT_STOPS.length; i++) {
    const delta = Math.abs(FONT_STOPS[i] - fontSize);
    if (delta < bestDelta) {
      bestDelta = delta;
      best = i;
    }
  }
  return best;
}

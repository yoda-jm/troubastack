/**
 * T50 — the studio cue glyph picker's rendering + labels. The GEOMETRY is NOT here:
 * it comes from the shared generated contract `@troubastack/ink` (`glyphs.json`, built
 * from `web/ink/glyphs.authoring.mjs`) — one source, consumed identically by the app
 * (A20) and the T51 stamp tool, no SVG parser at runtime. This file only maps the ids
 * to English labels and renders a glyph's normalized polylines as a tintable SVG.
 *
 * Everything draws in `currentColor`, so a tint is a CSS `color`; an unknown icon id
 * renders as the `note` fallback (resolveGlyphId), never an error.
 */
import type { JSX } from "react";
import { getGlyph, GLYPH_IDS, resolveGlyphId } from "@troubastack/ink";

export const CUE_ICON_IDS = GLYPH_IDS;

/** Human labels for the picker (the app localizes its own; studio is en). */
export const CUE_ICON_LABELS: Record<string, string> = {
  "guitar-electric": "Electric guitar",
  "guitar-acoustic": "Acoustic guitar",
  "guitar-classical": "Classical guitar",
  bass: "Bass",
  ukulele: "Ukulele",
  autoharp: "Autoharp",
  melodica: "Melodica",
  keys: "Keys",
  cajon: "Cajón",
  bongo: "Bongos",
  djembe: "Djembe",
  guiro: "Güiro",
  cuica: "Cuíca",
  shaker: "Shaker",
  "egg-shaker": "Egg shaker",
  tambourine: "Tambourine",
  mic: "Microphone",
  warning: "Warning",
  note: "Note",
};

/** Resolve an icon id to a known glyph, falling back to `note` for unknowns. */
export const resolveCueIcon = resolveGlyphId;

const labelFor = (id: string) => CUE_ICON_LABELS[id] ?? id;
const pts = (poly: number[][]) => poly.map(([x, y]) => `${x},${y}`).join(" ");

/**
 * A single tintable cue glyph rendered from the shared polyline contract. `color`
 * sets the tint ("" / undefined = currentColor — the neutral case). An unknown icon
 * id renders as `note`.
 */
export function CueGlyph({
  icon,
  color,
  size = 20,
  title,
}: {
  icon: string;
  color?: string;
  size?: number;
  title?: string;
}): JSX.Element {
  const resolved = resolveCueIcon(icon);
  const g = getGlyph(icon);
  const label = title ?? labelFor(resolved);
  return (
    <svg
      className="cue-glyph"
      width={size}
      height={size}
      viewBox="0 0 1 1"
      role="img"
      aria-label={label}
      data-icon={resolved}
      style={color ? { color } : undefined}
    >
      <title>{label}</title>
      {g.fills.map((poly, i) => (
        <polygon key={`f${i}`} points={pts(poly)} fill="currentColor" stroke="none" />
      ))}
      {g.strokes.map((poly, i) => (
        <polyline
          key={`s${i}`}
          points={pts(poly)}
          fill="none"
          stroke="currentColor"
          strokeWidth={g.strokeWidth}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      ))}
    </svg>
  );
}

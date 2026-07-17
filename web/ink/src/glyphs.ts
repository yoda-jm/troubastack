/**
 * T50/T51 — the runtime glyph API over the generated contract `glyphs.json` (built
 * by ../gen-glyphs.mjs from ../glyphs.authoring.mjs; NEVER parse SVG at runtime).
 * Coords are polylines normalized to a 1×1 box (y-down); `strokes` are stroked with
 * round caps/joins at `strokeWidth × renderSize`, `fills` are filled non-zero. Studio
 * (browser) and, later, the app renders from exactly this data. Unknown ids resolve
 * to `note` (the pinned fallback) — new ids can ship before every consumer knows them.
 */
import glyphsJson from "../glyphs.json";

export interface Glyph {
  /** Open polylines to stroke. Each is a list of [x,y] in a 1×1 box (y-down). */
  strokes: number[][][];
  /** Closed polygons to fill (non-zero winding). */
  fills: number[][][];
  /** Stroke width in box units; a consumer multiplies by its render size. */
  strokeWidth: number;
}

const DATA = glyphsJson as { version: number; glyphs: Record<string, Glyph> };

/** The glyph ids in authoring/picker order. */
export const GLYPH_IDS: string[] = Object.keys(DATA.glyphs);
/** The required fallback id for unknown/future icons. */
export const FALLBACK_GLYPH_ID = "note";

/** Resolve an id to a known glyph id, falling back to `note`. */
export function resolveGlyphId(id: string): string {
  return Object.prototype.hasOwnProperty.call(DATA.glyphs, id) ? id : FALLBACK_GLYPH_ID;
}

/** Get a glyph's geometry (resolving unknown ids to the `note` fallback). */
export function getGlyph(id: string): Glyph {
  return DATA.glyphs[resolveGlyphId(id)];
}

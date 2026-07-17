/**
 * T51 — the floating glyph palette shown while the "Icon" tool is active. Pick which
 * glyph the next stamp places (its id becomes the object's `text`); the current tool
 * color tints the preview. Kept OUT of the footprint-tuned style toolbar (T27/T33) as a
 * small separate panel over the canvas. The color/opacity live in the style row.
 */
import { CueGlyph, CUE_ICON_IDS, CUE_ICON_LABELS } from "../../components/CueGlyphs";

export function IconGlyphPalette({
  active,
  color,
  onPick,
}: {
  active: string;
  color: string;
  onPick: (glyph: string) => void;
}) {
  return (
    <div className="icon-palette" data-testid="icon-palette" role="group" aria-label="Stamp icon">
      {CUE_ICON_IDS.map((id) => (
        <button
          key={id}
          type="button"
          className={`icon-pick${active === id ? " active" : ""}`}
          data-testid={`icon-pick-${id}`}
          title={CUE_ICON_LABELS[id] ?? id}
          aria-label={CUE_ICON_LABELS[id] ?? id}
          aria-pressed={active === id}
          onClick={() => onPick(id)}
        >
          <CueGlyph icon={id} color={color} size={22} />
        </button>
      ))}
    </div>
  );
}

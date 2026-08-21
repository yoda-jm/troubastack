/**
 * T51 — the floating glyph palette shown while the "Icon" tool is active. Pick which
 * glyph the next stamp places (its id becomes the object's `text`); the current tool
 * color tints the preview. Kept OUT of the footprint-tuned style toolbar (T27/T33) as a
 * small separate panel over the canvas. The color/opacity live in the style row.
 *
 * T88 — it now HUGS the page: `position:fixed`, left computed just outside the score's left
 * edge (`iconPaletteLeft`), clamping to the viewport edge only when zoom pushes the page past it.
 * Repositions on zoom/scroll/resize/page change — no rAF (it is static between those events).
 */
import { useLayoutEffect, useRef, useState } from "react";
import { iconPaletteLeft } from "../../layout";
import { CueGlyph, CUE_ICON_IDS, CUE_ICON_LABELS } from "../../components/CueGlyphs";

const PALETTE_GAP = 10; // px between the palette's right edge and the page's left edge
const PALETTE_MARGIN = 8; // px floor from the viewport edge when zoomed in

export function IconGlyphPalette({
  active,
  color,
  onPick,
  reflowKey,
}: {
  active: string;
  color: string;
  onPick: (glyph: string) => void;
  /** Changes when something that moves the page changes (zoom / page / file) → re-measure. */
  reflowKey?: string;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [left, setLeft] = useState<number | null>(null);

  useLayoutEffect(() => {
    const el = ref.current;
    if (!el) return;
    const place = () => {
      const scroll = document.querySelector<HTMLElement>(".viewer-scroll");
      const page = scroll?.querySelector<HTMLElement>(".pdf-page"); // all pages share the centred
      if (!scroll || !page) {
        setLeft(null); // no page → fall back to the CSS default
        return;
      }
      const p = page.getBoundingClientRect(); // one read; left is the same for every page
      const v = scroll.getBoundingClientRect();
      setLeft(
        iconPaletteLeft(
          { left: p.left, top: p.top, right: p.right, bottom: p.bottom },
          { left: v.left, top: v.top, right: v.right, bottom: v.bottom },
          el.offsetWidth,
          PALETTE_GAP,
          PALETTE_MARGIN,
        ),
      );
    };
    place();
    window.addEventListener("scroll", place, true); // capture: nested scrollers count
    window.addEventListener("resize", place);
    return () => {
      window.removeEventListener("scroll", place, true);
      window.removeEventListener("resize", place);
    };
  }, [reflowKey]);

  return (
    <div
      className="icon-palette"
      data-testid="icon-palette"
      role="group"
      aria-label="Stamp icon"
      ref={ref}
      style={left != null ? { left } : undefined}
    >
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

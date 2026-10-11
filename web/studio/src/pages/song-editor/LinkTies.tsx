/**
 * T196 — anchoring stage 3: the LINK of a mark, drawn — a dot on the linked text and a thin dotted line
 * to the nearest edge of the mark's box (VLL's idea, ⟨D1⟩ of T192). Shown for the one selected mark, or
 * for every linked mark on the page while "Show links" is on.
 *
 * CHROME ONLY. It is DOM inside the page's `.selection-overlay` (aria-hidden, pointer-events: none), like
 * the P206 jump segment: it is never painted into the annotation canvas, so it cannot reach a bake, a PDF
 * download, a note composite or Stage — all of which render server-side from the stored marks.
 *
 * Its own component, rendered from data handed in (the JumpFlags precedent), so a DOM test can draw it.
 */
import type { LinkAt } from "./markLink";
import { linkTie } from "./markLink";

export type LinkTieItem = {
  uuid: string;
  state: "linked" | "check";
  at: LinkAt;
  /** The mark's box in [0,1] page coordinates (the live preview while it is dragged). */
  box: { minX: number; minY: number; maxX: number; maxY: number };
};

export function LinkTies({ items, pageBoxPx }: { items: LinkTieItem[]; pageBoxPx: { w: number; h: number } | null }) {
  if (!pageBoxPx || items.length === 0) return null;
  const { w, h } = pageBoxPx;
  return (
    <svg
      className="link-ties"
      data-testid="link-ties"
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {items.map((it) => {
        const { dot, end } = linkTie(it.at, it.box, w, h);
        return (
          <g key={it.uuid} className={`link-tie link-${it.state}`} data-testid="link-tie" data-uuid={it.uuid} data-state={it.state}>
            {end && (
              <>
                <line className="link-line-halo" x1={dot.x} y1={dot.y} x2={end.x} y2={end.y} />
                <line className="link-line" data-testid="link-line" x1={dot.x} y1={dot.y} x2={end.x} y2={end.y} />
              </>
            )}
            <circle
              className="link-dot"
              data-testid="link-dot"
              data-x={dot.x.toFixed(1)}
              data-y={dot.y.toFixed(1)}
              cx={dot.x}
              cy={dot.y}
              r={3.5}
            />
          </g>
        );
      })}
    </svg>
  );
}

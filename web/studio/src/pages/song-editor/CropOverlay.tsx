import { useCallback, useEffect, useRef, useState } from "react";
import type { CropRect } from "../../api";
import { type Rect, squareAt, moveRect, clamp01 } from "./cropGeometry";

// CropOverlay (T198) draws a page's bake area: a TRANSPARENT window at the page's aspect ratio (policy A — a
// square in page fractions, a pure zoom), with the area OUTSIDE it under a light grey veil hatched at 45°.
//
// Two modes, from `interactive`:
//   - interactive (the Crop tool is active on a croppable PDF): the window can be drawn / moved / scaled with
//     corner handles, nudged by the keyboard, and reset. It owns its own pointer + keyboard gestures,
//     separate from the annotation EditCanvas.
//   - passive (the tool is off, but the page has a crop): the veil STAYS visible so the baked area is always
//     in view, and the whole overlay is pointer-events:none so it never blocks annotating.
// A page with no crop renders nothing at all (the component returns null). It never applies the bake's view
// transform on the client — "Tablet view" (U1b) is a server render. The rotate handle (T199) mounts above the
// top edge later; the window is centred and clamped so that slots in without reshaping this.

const NUDGE = 0.005; // arrow-key step (fraction of the page)
const NUDGE_BIG = 0.02; // Shift+arrow step

type HandleId = "nw" | "ne" | "se" | "sw";
const HANDLES: HandleId[] = ["nw", "ne", "se", "sw"];
function oppositeCorner(r: Rect, h: HandleId): { x: number; y: number } {
  const x = h === "nw" || h === "sw" ? r.x + r.w : r.x;
  const y = h === "nw" || h === "ne" ? r.y + r.h : r.y;
  return { x, y };
}

type Gesture =
  // `before` is the rect as the gesture STARTED, so Escape restores it (or clears, when the page had no crop
  // and this is a from-scratch draw). A redraw over an existing crop records the existing rect, so Escape
  // never wipes a saved crop.
  | { kind: "draw"; before: Rect | null; ax: number; ay: number }
  | { kind: "move"; before: Rect; orig: Rect; startX: number; startY: number }
  | { kind: "resize"; before: Rect; handle: HandleId; ax: number; ay: number };

export type CropOverlayProps = {
  /** The rect to show (draft while editing, saved otherwise), or null when the page is uncropped. */
  rect: CropRect | null;
  /** True when the Crop tool is active on this croppable PDF: the window is editable. False → passive veil. */
  interactive: boolean;
  /** New rect as the user draws / moves / resizes / nudges. */
  onChange: (rect: Rect) => void;
  /** Clear THIS page's crop (the per-page Reset, and the Escape of a from-scratch draw). */
  onReset: () => void;
  page: number;
};

export default function CropOverlay({ rect, interactive, onChange, onReset, page }: CropOverlayProps) {
  const ref = useRef<HTMLDivElement>(null);
  const gestureRef = useRef<Gesture | null>(null);
  const [dragging, setDragging] = useState(false);

  const toFrac = useCallback((clientX: number, clientY: number) => {
    const el = ref.current;
    if (!el) return { x: 0, y: 0 };
    const b = el.getBoundingClientRect();
    return { x: clamp01((clientX - b.left) / b.width), y: clamp01((clientY - b.top) / b.height) };
  }, []);

  const current: Rect | null = rect ? { x: rect.x, y: rect.y, w: rect.w, h: rect.h } : null;

  const endGesture = useCallback(() => {
    gestureRef.current = null;
    setDragging(false);
  }, []);
  useEffect(() => endGesture, [endGesture]);

  const onPointerDown = useCallback(
    (e: React.PointerEvent) => {
      if (!interactive || e.button !== 0) return;
      e.preventDefault();
      (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
      ref.current?.focus();
      const pt = toFrac(e.clientX, e.clientY);
      const handle = (e.target as HTMLElement).dataset?.handle as HandleId | undefined;
      if (handle && current) {
        const opp = oppositeCorner(current, handle);
        gestureRef.current = { kind: "resize", before: current, handle, ax: opp.x, ay: opp.y };
      } else if (current && pt.x >= current.x && pt.x <= current.x + current.w && pt.y >= current.y && pt.y <= current.y + current.h) {
        gestureRef.current = { kind: "move", before: current, orig: current, startX: pt.x, startY: pt.y };
      } else {
        // A draw from empty space. `before: current` so Escape restores a pre-existing crop rather than
        // wiping it; it is null only on a genuinely uncropped page (then Escape clears).
        gestureRef.current = { kind: "draw", before: current, ax: pt.x, ay: pt.y };
      }
      setDragging(true);
    },
    [interactive, current, toFrac],
  );

  const onPointerMove = useCallback(
    (e: React.PointerEvent) => {
      const g = gestureRef.current;
      if (!g) return;
      const pt = toFrac(e.clientX, e.clientY);
      if (g.kind === "move") onChange(moveRect(g.orig, pt.x - g.startX, pt.y - g.startY));
      else onChange(squareAt(g.ax, g.ay, pt.x, pt.y));
    },
    [onChange],
  );

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (!interactive) return;
      if (e.key === "Escape") {
        // Restore the rect to the START of the in-progress gesture (and a from-scratch draw clears the page).
        const g = gestureRef.current;
        if (g) {
          if (g.before) onChange(g.before);
          else onReset();
          endGesture();
          e.preventDefault();
        }
        return;
      }
      if (!current) return;
      const step = e.shiftKey ? NUDGE_BIG : NUDGE;
      let dx = 0;
      let dy = 0;
      if (e.key === "ArrowLeft") dx = -step;
      else if (e.key === "ArrowRight") dx = step;
      else if (e.key === "ArrowUp") dy = -step;
      else if (e.key === "ArrowDown") dy = step;
      else return;
      e.preventDefault();
      onChange(moveRect(current, dx, dy));
    },
    [interactive, current, onChange, onReset, endGesture],
  );

  // A page with no crop and no active tool shows nothing. But when the tool IS active we still mount the
  // overlay on an uncropped page, so a first drag can draw the window — it just shows no veil yet.
  if (!current && !interactive) return null;

  const pct = (r: Rect) => ({
    left: `${r.x * 100}%`,
    top: `${r.y * 100}%`,
    width: `${r.w * 100}%`,
    height: `${r.h * 100}%`,
  });

  return (
    <div
      ref={ref}
      className={`crop-overlay${interactive ? " interactive" : " passive"}${dragging ? " dragging" : ""}`}
      data-testid={`crop-overlay-${page}`}
      tabIndex={interactive ? 0 : -1}
      role={interactive ? "application" : undefined}
      aria-label={interactive ? "Crop area" : undefined}
      aria-hidden={interactive ? undefined : true}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={endGesture}
      onPointerCancel={endGesture}
      onKeyDown={onKeyDown}
    >
      {current && (
        <>
          {/* The hatched grey veil over the four bands OUTSIDE the window. */}
          <div className="crop-dim" style={{ left: 0, top: 0, width: "100%", height: `${current.y * 100}%` }} />
          <div className="crop-dim" style={{ left: 0, top: `${(current.y + current.h) * 100}%`, width: "100%", bottom: 0 }} />
          <div className="crop-dim" style={{ left: 0, top: `${current.y * 100}%`, width: `${current.x * 100}%`, height: `${current.h * 100}%` }} />
          <div className="crop-dim" style={{ left: `${(current.x + current.w) * 100}%`, top: `${current.y * 100}%`, right: 0, height: `${current.h * 100}%` }} />

          {/* The transparent window. Handles + affordances + reset only when interactive. */}
          <div className="crop-rect" data-testid={`crop-rect-${page}`} style={pct(current)}>
            {interactive && (
          <>
            {/* faint move cross (centre) — the move affordance; never blocks the move drag */}
            <span className="crop-move-cross" aria-hidden="true" />
            {HANDLES.map((h) => (
              <span
                key={h}
                className={`crop-handle crop-handle-${h}`}
                data-handle={h}
                data-testid={`crop-handle-${page}-${h}`}
              />
            ))}
            {/* T199 rotate handle mounts here, above the top edge; the window is centred so rotation about its
                centre slots in without reshaping this. */}
            <button
              type="button"
              className="crop-reset-btn"
              data-testid={`crop-reset-${page}`}
              title="Reset this page (bake the whole page)"
              aria-label="Reset this page"
              onPointerDown={(e) => e.stopPropagation()}
              onClick={(e) => {
                e.stopPropagation();
                onReset();
              }}
            >
              ×
            </button>
              </>
            )}
          </div>
        </>
      )}
    </div>
  );
}

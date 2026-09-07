// T78 — the shared drag-to-reorder primitive, extracted from the setlist so the Files list can
// reuse the exact same behaviour (VLL: "homogeneity first … think components" — a future touch/
// pointer fix then lands ONCE, here). This is deliberately headless: it supplies the grip drag
// source, the per-row drop handlers + hover highlight, the keyboard/menu move helpers and the FLIP
// motion, but each call site keeps its OWN row markup, testids and classes. That is what lets the
// setlist's DOM (and therefore its e2e) stay byte-identical across the extraction.
//
// Scope is a SINGLE ordered group (the spec's steer: extract the row/drag primitive, not the
// setlist's main/bench grouping). The setlist composes two groups by using two useSortable() over a
// shared useFlipRows(), so cross-group ★ moves still animate list-wide.
import {
  useCallback,
  useLayoutEffect,
  useRef,
  useState,
  type CSSProperties,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from "react";

const FLIP_MS = 200;

// useFlipRows — FLIP reorder motion (T52, lifted verbatim from SetlistDetail). Rows register their
// element by id into ONE map, so on each commit (dep change) every tracked row that moved plays an
// inverse-translate → zero transition — drag, move up/down and cross-group moves animate uniformly,
// dependency-free, on every browser. prefers-reduced-motion skips the transforms (instant).
export function useFlipRows(dep: unknown): (id: string, el: HTMLElement | null) => void {
  const els = useRef(new Map<string, HTMLElement>());
  const prev = useRef(new Map<string, DOMRect>());
  const register = useCallback((id: string, el: HTMLElement | null) => {
    if (el) els.current.set(id, el);
    else els.current.delete(id);
  }, []);
  useLayoutEffect(() => {
    const reduce = window.matchMedia?.("(prefers-reduced-motion: reduce)")?.matches ?? false;
    const next = new Map<string, DOMRect>();
    els.current.forEach((el, id) => next.set(id, el.getBoundingClientRect()));
    if (!reduce) {
      next.forEach((r, id) => {
        const p = prev.current.get(id);
        if (!p) return; // newly mounted row — nothing to animate from
        const dx = p.left - r.left;
        const dy = p.top - r.top;
        if (Math.abs(dx) < 1 && Math.abs(dy) < 1) return;
        const el = els.current.get(id);
        if (!el) return;
        // Invert: jump back to the old position with no transition…
        el.style.transition = "none";
        el.style.transform = `translate(${dx}px, ${dy}px)`;
        el.getBoundingClientRect(); // force reflow so the jump is applied before playing
        // …then play forward to the natural position.
        requestAnimationFrame(() => {
          el.style.transition = `transform ${FLIP_MS}ms ease`;
          el.style.transform = "";
        });
      });
    }
    prev.current = next;
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [dep]);
  return register;
}

// reorder returns a new id list with the item at `from` moved to land above the row at `to` — the
// same "drop hint is the row's top border" semantics the setlist uses, so a downward drop lands
// where the hint shows rather than one slot too low.
export function reorder(ids: string[], from: number, to: number): string[] {
  const arr = ids.slice();
  const [moved] = arr.splice(from, 1);
  const insertAt = from < to ? to - 1 : to;
  arr.splice(insertAt, 0, moved);
  return arr;
}

// reorderTo moves the item at `from` into GAP `position` — the N+1-gap model (T142 stage 1). For N items
// there are N+1 insertion gaps: 0 = before the first row, k = between rows k-1 and k, N = AFTER the last
// row. The end gap (position === ids.length) is the one the old top-edge `reorder`/HTML5-drop model could
// not express ("on ne peut pas deplacer un morceau en dernier") — there is no row after the last to hint
// against. Removing the moved item first shifts every later gap down by one. The pointer-drag rewrite
// (T142 stage 2) computes a gap from the pointer position and commits through here.
export function reorderTo(ids: string[], from: number, position: number): string[] {
  const arr = ids.slice();
  const [moved] = arr.splice(from, 1);
  const insertAt = position > from ? position - 1 : position;
  arr.splice(Math.max(0, Math.min(insertAt, arr.length)), 0, moved);
  return arr;
}

// dropGapFor returns the insertion GAP (0..N) a pointer is over, given each row's vertical midpoint in
// order. A pointer above row i's midpoint ⇒ gap i (insert before row i); past the last midpoint ⇒ gap N —
// the END gap the old top-edge model could not reach ("on ne peut pas deplacer un morceau en dernier").
// Pure + geometry-mocked, so the gap math is unit-tested (T142 stage 2).
export function dropGapFor(midpointsY: number[], pointerY: number): number {
  for (let i = 0; i < midpointsY.length; i++) {
    if (pointerY < midpointsY[i]) return i;
  }
  return midpointsY.length;
}

// scrollParent walks up to the nearest vertically-scrollable ancestor — the container edge auto-scroll
// acts on — falling back to the document scroller.
function scrollParent(el: HTMLElement | null): HTMLElement {
  for (let n = el?.parentElement ?? null; n; n = n.parentElement) {
    const oy = getComputedStyle(n).overflowY;
    if ((oy === "auto" || oy === "scroll") && n.scrollHeight > n.clientHeight) return n;
  }
  return (document.scrollingElement as HTMLElement | null) ?? document.body;
}

const EDGE_PX = 48; // distance from a container edge where auto-scroll engages
const EDGE_SPEED = 12; // px per animation frame while held at the edge

// SortableRowProps spread onto the call site's row element; GripProps onto its grip. Pointer Events replace
// HTML5 drag-and-drop (T142 stage 2): one code path for mouse/touch/pen, an END drop position, edge
// auto-scroll, focus-preserving keyboard reorder, and no accidental text selection on a touch grip.
export interface SortableRowProps {
  ref: (el: HTMLElement | null) => void;
}
export interface GripProps {
  ref: (el: HTMLElement | null) => void;
  onPointerDown: (e: ReactPointerEvent) => void;
  onKeyDown: (e: ReactKeyboardEvent) => void;
  tabIndex: 0;
  role: "button";
  "aria-label": string;
  style: CSSProperties;
}

export interface Sortable {
  rowProps: (index: number) => SortableRowProps; // spread onto each row element
  gripProps: (index: number) => GripProps; // spread onto each row's grip/handle
  isDragOver: (index: number) => boolean; // an insertion line goes ABOVE this row (drop gap === index)
  isDropAtEnd: () => boolean; // the drop gap is AFTER the last row — render a line below it (the END gap)
  dragging: boolean; // a pointer drag is in progress (for a source-dim / cursor cue)
  canMoveUp: (index: number) => boolean;
  canMoveDown: (index: number) => boolean;
  move: (index: number, dir: -1 | 1) => void; // arrow / …-menu reorder — restores focus to the moved row
  liveMessage: string; // ARIA live-region text announcing the latest keyboard reorder
}

// useSortable wires a Pointer-Events drag + keyboard reorder over `ids`, calling onReorder(newOrderedIds)
// after any successful reorder (the caller persists — reorderSetlist for the setlist, displayOrder PATCHes
// for Files). registerRef comes from a useFlipRows() so the caller controls the FLIP scope.
export function useSortable(
  ids: string[],
  onReorder: (orderedIds: string[]) => void | Promise<void>,
  registerRef: (id: string, el: HTMLElement | null) => void,
): Sortable {
  const rowEls = useRef(new Map<string, HTMLElement>());
  const gripEls = useRef(new Map<string, HTMLElement>());
  const [dropGap, setDropGap] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);
  const [live, setLive] = useState("");

  // Fresh ids/onReorder for the imperative document listeners (added at drag start), so they never act on
  // a stale order.
  const cur = useRef({ ids, onReorder });
  cur.current = { ids, onReorder };

  // The active pointer drag. Held in a ref (not state) so the listeners mutate it without re-rendering.
  const drag = useRef<{
    from: number;
    pointerId: number;
    lastY: number;
    gap: number;
    raf: number;
    container: HTMLElement;
  } | null>(null);

  // After a reorder the list re-renders; focus the moved row's grip so an arrow move never drops focus to
  // <body> and jumps the page ("les fleches repositionne ou on se trouve dans la page").
  const focusAfter = useRef<string | null>(null);
  useLayoutEffect(() => {
    const id = focusAfter.current;
    focusAfter.current = null;
    if (id) gripEls.current.get(id)?.focus({ preventScroll: true });
  }, [ids]);

  const midpoints = useCallback(
    () =>
      cur.current.ids.map((id) => {
        const el = rowEls.current.get(id);
        if (!el) return Number.POSITIVE_INFINITY;
        const r = el.getBoundingClientRect();
        return r.top + r.height / 2;
      }),
    [],
  );

  const recomputeGap = useCallback(
    (clientY: number) => {
      const g = dropGapFor(midpoints(), clientY);
      if (drag.current) drag.current.gap = g;
      setDropGap(g);
    },
    [midpoints],
  );

  const onMove = useRef<(e: PointerEvent) => void>(() => {});
  const onUp = useRef<(e: PointerEvent) => void>(() => {});
  const onCancel = useRef<() => void>(() => {});
  // STABLE listener identities (created once) that delegate to the latest .current — so add/remove
  // EventListener always match the same reference and a drag's listeners actually detach on drop (a fresh
  // closure each render would leak the old ones and re-fire on the next drag).
  const moveWrap = useRef((e: PointerEvent) => onMove.current(e)).current;
  const upWrap = useRef((e: PointerEvent) => onUp.current(e)).current;
  const cancelWrap = useRef(() => onCancel.current()).current;

  const endDrag = useCallback(
    (commit: boolean) => {
      const d = drag.current;
      if (!d) return;
      cancelAnimationFrame(d.raf);
      document.removeEventListener("pointermove", moveWrap);
      document.removeEventListener("pointerup", upWrap);
      document.removeEventListener("pointercancel", cancelWrap);
      drag.current = null;
      setDragging(false);
      setDropGap(null);
      // Gaps that leave the item in place (its own slot, before or after) are no-ops — don't persist them.
      if (commit && d.gap !== d.from && d.gap !== d.from + 1) {
        focusAfter.current = cur.current.ids[d.from];
        void cur.current.onReorder(reorderTo(cur.current.ids, d.from, d.gap));
      }
    },
    [],
  );

  const autoScroll = useCallback(() => {
    const d = drag.current;
    if (!d) return;
    const c = d.container;
    const isDoc = c === document.scrollingElement || c === document.body;
    const top = isDoc ? 0 : c.getBoundingClientRect().top;
    const bottom = isDoc ? window.innerHeight : c.getBoundingClientRect().bottom;
    let dy = 0;
    if (d.lastY < top + EDGE_PX) dy = -EDGE_SPEED;
    else if (d.lastY > bottom - EDGE_PX) dy = EDGE_SPEED;
    if (dy !== 0) {
      c.scrollBy(0, dy);
      recomputeGap(d.lastY); // rows moved under a stationary finger — keep the indicator honest
    }
    d.raf = requestAnimationFrame(autoScroll);
  }, [recomputeGap]);

  onMove.current = (e: PointerEvent) => {
    const d = drag.current;
    if (!d || e.pointerId !== d.pointerId) return;
    d.lastY = e.clientY;
    recomputeGap(e.clientY);
  };
  onUp.current = (e: PointerEvent) => {
    if (drag.current && e.pointerId === drag.current.pointerId) endDrag(true);
  };
  onCancel.current = () => endDrag(false);

  const move = useCallback((index: number, dir: -1 | 1) => {
    const ids0 = cur.current.ids;
    const to = index + dir;
    if (to < 0 || to >= ids0.length) return;
    const arr = ids0.slice();
    [arr[index], arr[to]] = [arr[to], arr[index]]; // adjacent swap = one step up/down
    focusAfter.current = ids0[index];
    setLive(`Moved to position ${to + 1} of ${ids0.length}`);
    void cur.current.onReorder(arr);
  }, []);

  return {
    rowProps: (index: number): SortableRowProps => ({
      ref: (el) => {
        const id = ids[index];
        registerRef(id, el); // FLIP scope (caller-controlled)
        if (el) rowEls.current.set(id, el);
        else rowEls.current.delete(id);
      },
    }),
    gripProps: (index: number): GripProps => ({
      ref: (el) => {
        const id = ids[index];
        if (el) gripEls.current.set(id, el);
        else gripEls.current.delete(id);
      },
      onPointerDown: (e) => {
        if (e.pointerType === "mouse" && e.button !== 0) return; // left button only for a mouse
        e.preventDefault(); // a touch that isn't yet a drag must not select the title text (defect 4)
        (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
        const container = scrollParent(rowEls.current.get(cur.current.ids[0]) ?? null);
        drag.current = { from: index, pointerId: e.pointerId, lastY: e.clientY, gap: index, raf: 0, container };
        setDragging(true);
        setDropGap(index);
        document.addEventListener("pointermove", moveWrap);
        document.addEventListener("pointerup", upWrap);
        document.addEventListener("pointercancel", cancelWrap);
        drag.current.raf = requestAnimationFrame(autoScroll);
      },
      onKeyDown: (e) => {
        if (e.key === "ArrowUp") {
          e.preventDefault();
          move(index, -1);
        } else if (e.key === "ArrowDown") {
          e.preventDefault();
          move(index, 1);
        }
      },
      tabIndex: 0,
      role: "button",
      "aria-label": `Reorder: drag, or focus and use arrow keys (position ${index + 1} of ${ids.length})`,
      style: { touchAction: "none", userSelect: "none", cursor: "grab" },
    }),
    isDragOver: (index: number) => dropGap === index,
    isDropAtEnd: () => dropGap === ids.length,
    dragging,
    canMoveUp: (index: number) => index > 0,
    canMoveDown: (index: number) => index < ids.length - 1,
    move,
    liveMessage: live,
  };
}

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
import { useCallback, useLayoutEffect, useRef, useState, type DragEvent } from "react";

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

// SortableRowProps spread onto the call site's row element; GripProps onto its grip drag source.
export interface SortableRowProps {
  ref: (el: HTMLElement | null) => void;
  onDragOver: (e: DragEvent) => void;
  onDragLeave: (e: DragEvent) => void;
  onDrop: (e: DragEvent) => void;
}
export interface GripProps {
  draggable: true;
  onDragStart: (e: DragEvent) => void;
}

export interface Sortable {
  rowProps: (index: number) => SortableRowProps; // spread onto each row element
  gripProps: (index: number) => GripProps; // spread onto each row's grip/handle
  isDragOver: (index: number) => boolean; // true for the row currently hinted as the drop target
  canMoveUp: (index: number) => boolean;
  canMoveDown: (index: number) => boolean;
  move: (index: number, dir: -1 | 1) => void; // keyboard / …-menu reorder — same persisted result
}

// useSortable wires drag + move over `ids`, calling onReorder(newOrderedIds) after any successful
// reorder (the caller persists — reorderSetlist for the setlist, displayOrder PATCHes for Files).
// registerRef comes from a useFlipRows() so the caller controls the FLIP scope (list-wide for the
// setlist's two groups; per-list for Files).
export function useSortable(
  ids: string[],
  onReorder: (orderedIds: string[]) => void | Promise<void>,
  registerRef: (id: string, el: HTMLElement | null) => void,
): Sortable {
  const dragFrom = useRef<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  const commit = useCallback(
    (from: number, to: number) => {
      if (from === to) return;
      void onReorder(reorder(ids, from, to));
    },
    [ids, onReorder],
  );

  return {
    rowProps: (index: number): SortableRowProps => ({
      ref: (el) => registerRef(ids[index], el),
      onDragOver: (e) => {
        if (dragFrom.current === null) return; // not our drag
        e.preventDefault();
        e.dataTransfer.dropEffect = "move";
        setOverIndex(index);
      },
      onDragLeave: (e) => {
        // only clear when the pointer truly leaves the row — dragleave also fires crossing a CHILD
        // (grip, buttons) still inside the row, which made the hint flicker (T52).
        if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
          setOverIndex((cur) => (cur === index ? null : cur));
        }
      },
      onDrop: (e) => {
        e.preventDefault();
        const from = dragFrom.current;
        dragFrom.current = null;
        setOverIndex(null);
        if (from !== null) commit(from, index);
      },
    }),
    gripProps: (index: number): GripProps => ({
      draggable: true,
      onDragStart: (e) => {
        e.dataTransfer.effectAllowed = "move";
        dragFrom.current = index;
      },
    }),
    isDragOver: (index: number) => overIndex === index,
    canMoveUp: (index: number) => index > 0,
    canMoveDown: (index: number) => index < ids.length - 1,
    move: (index: number, dir: -1 | 1) => {
      const to = index + dir;
      if (to < 0 || to >= ids.length) return;
      // For an adjacent swap the "land above `to`" hint math differs by direction; go through the
      // same commit path with the destination expressed as a hint row so drag and move agree.
      commit(index, dir === 1 ? to + 1 : to);
    },
  };
}

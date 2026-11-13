/**
 * T210 — the one scroll box every line-up grid lives in: the Line-ups grid, its by-part view and the Sound sheet.
 *
 * T203-FIX3 stuck the header and the song column against the PAGE's scroll. On a touch engine a page wider than
 * the screen widens the layout viewport to the table, so a finger drag only pans the visual viewport across a page
 * that never scrolls, and both the header and the song column ride away. Inside a box with a bounded height
 * (`.lineup-scroll` in styles.css) the page keeps the screen's width and the box is what scrolls, so the header
 * sticks to the box's top and the song column to its left, on a tablet and on a desktop alike.
 */
import type { ReactNode } from "react";

export function LineupScroll({ children }: { children: ReactNode }) {
  return (
    <div className="lineup-scroll" data-testid="lineup-scroll">
      {children}
    </div>
  );
}

/**
 * M4: focus() scrolls the box only when the cell is outside it, and then just to its edge. A cell that's inside
 * the box but under the stuck header rows or the stuck song column stays hidden there (measured: going up, row 9
 * under the header; going left, a column under the song cell). Move the box on until the cell clears both.
 */
export function revealInLineupScroll(cell: HTMLElement) {
  const box = cell.closest<HTMLElement>(".lineup-scroll");
  if (!box) return;
  const b = box.getBoundingClientRect();
  const headH = box.querySelector("thead")?.getBoundingClientRect().height ?? 0;
  const songW = box.querySelector(".lineup-corner")?.getBoundingClientRect().width ?? 0;
  const r = (cell.closest("td") ?? cell).getBoundingClientRect();
  const top = b.top + box.clientTop + headH;
  const bottom = b.top + box.clientTop + box.clientHeight;
  const left = b.left + box.clientLeft + songW;
  const right = b.left + box.clientLeft + box.clientWidth;
  if (r.top < top) box.scrollTop -= top - r.top;
  else if (r.bottom > bottom) box.scrollTop += r.bottom - bottom;
  if (r.left < left) box.scrollLeft -= left - r.left;
  else if (r.right > right) box.scrollLeft += r.right - right;
}

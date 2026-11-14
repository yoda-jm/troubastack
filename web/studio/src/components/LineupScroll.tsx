/**
 * T210 — the one scroll box every line-up grid lives in: the Line-ups grid, its by-part view and the Sound sheet.
 *
 * T203-FIX3 stuck the header and the song column against the PAGE's scroll. On a touch engine a page wider than
 * the screen widens the layout viewport to the table, so a finger drag only pans the visual viewport across a page
 * that never scrolls, and both the header and the song column ride away. Inside a box with a bounded height
 * (`.lineup-scroll` in styles.css) the page keeps the screen's width and the box is what scrolls, so the header
 * sticks to the box's top and the song column to its left, on a tablet and on a desktop alike.
 *
 * T211 — the Line-ups page gives the box a FILL mode: the box takes all the height its flex column has left
 * (`.lineup-scroll-fill`, no max-height), so it ends at the window's bottom edge and the page itself never
 * scrolls. The wrapper then also owns the three edge shadows and measures, after every layout, the frozen
 * header's height and the song column's width — set as CSS variables (`--head-px`, `--song-px`, plus the live
 * scrollbar thickness `--sbw`/`--sbh`) so the scrollbar tracks start beside what actually scrolls. The Sound
 * sheet keeps the bounded (non-fill) box; nothing here changes it.
 */
import { useCallback, useEffect, useLayoutEffect, useRef, type ReactNode, type RefObject } from "react";

export function LineupScroll({
  children,
  fill = false,
  scrollRef,
  onOverflowX,
}: {
  children: ReactNode;
  /** T211: fill the remaining height of the flex column (Line-ups page) instead of a bounded box (the sheet). */
  fill?: boolean;
  /** The scroll element, exposed so the page can scroll it (Jump to) and read it. */
  scrollRef?: RefObject<HTMLDivElement>;
  /** Called after each measure with whether the grid is wider than its box (drives "Jump to"). */
  onOverflowX?: (overflow: boolean) => void;
}) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const ownRef = useRef<HTMLDivElement>(null);
  const boxRef = scrollRef ?? ownRef;
  const edgeLRef = useRef<HTMLDivElement>(null);
  const edgeRRef = useRef<HTMLDivElement>(null);
  const edgeTRef = useRef<HTMLDivElement>(null);

  // The edge shadows: left once scrolled right of the song column, right while columns remain, top once scrolled
  // down. pointer-events:none; they sit clear of the scrollbars. Cheap, so they run on every scroll.
  const edges = useCallback(() => {
    const box = boxRef.current;
    if (!box) return;
    edgeLRef.current?.classList.toggle("on", box.scrollLeft > 0);
    edgeRRef.current?.classList.toggle("on", box.scrollLeft + box.clientWidth < box.scrollWidth - 1);
    edgeTRef.current?.classList.toggle("on", box.scrollTop > 0);
  }, [boxRef]);

  // Measure the frozen corner (song column width) and the thead (header height), and the live scrollbar
  // thickness, onto the wrap as CSS variables — the ::-webkit-scrollbar track margins and the edge shadows read
  // them. Also report whether the grid overflows horizontally (so "Jump to" shows only then). The whole table is
  // one block, so reading the corner/thead once is enough.
  const measure = useCallback(() => {
    const box = boxRef.current;
    const wrap = wrapRef.current;
    if (!box || !wrap) return;
    const corner = box.querySelector<HTMLElement>(".lineup-corner");
    const thead = box.querySelector<HTMLElement>("thead");
    wrap.style.setProperty("--song-px", `${corner ? corner.offsetWidth : 0}px`);
    wrap.style.setProperty("--head-px", `${thead ? thead.offsetHeight : 0}px`);
    wrap.style.setProperty("--sbw", `${box.offsetWidth - box.clientWidth}px`);
    wrap.style.setProperty("--sbh", `${box.offsetHeight - box.clientHeight}px`);
    edges();
    onOverflowX?.(box.scrollWidth > box.clientWidth + 1);
  }, [boxRef, edges, onOverflowX]);

  // Subscribe once: the box's own scroll (edges), and a ResizeObserver + window resize (re-measure on a viewport
  // or content size change). The measure-on-render effect below covers a content change that doesn't move the box.
  useEffect(() => {
    if (!fill) return;
    const box = boxRef.current;
    const wrap = wrapRef.current;
    if (!box || !wrap) return;
    box.addEventListener("scroll", edges, { passive: true });
    const ro = new ResizeObserver(() => measure());
    ro.observe(wrap);
    ro.observe(box);
    window.addEventListener("resize", measure);
    return () => {
      box.removeEventListener("scroll", edges);
      ro.disconnect();
      window.removeEventListener("resize", measure);
    };
  }, [fill, boxRef, edges, measure]);

  // After every render (the grid re-projects on a filter/edit, changing the table's size without resizing the
  // box), re-measure so the track margins, the edge shadows and "Jump to" follow.
  useLayoutEffect(() => {
    if (fill) measure();
  });

  if (!fill) {
    return (
      <div className="lineup-scroll" data-testid="lineup-scroll" ref={boxRef}>
        {children}
      </div>
    );
  }
  return (
    <div className="lineup-grid-wrap" ref={wrapRef}>
      <div className="lineup-scroll lineup-scroll-fill" data-testid="lineup-scroll" ref={boxRef}>
        {children}
      </div>
      <div className="lineup-edge lineup-edge-l" data-testid="lineup-edge-l" ref={edgeLRef} aria-hidden="true" />
      <div className="lineup-edge lineup-edge-r" data-testid="lineup-edge-r" ref={edgeRRef} aria-hidden="true" />
      <div className="lineup-edge lineup-edge-t" data-testid="lineup-edge-t" ref={edgeTRef} aria-hidden="true" />
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

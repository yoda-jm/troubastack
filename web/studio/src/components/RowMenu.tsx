// T78 — a compact "…" overflow menu for a list row's per-item actions (rename / delete / move /
// view source). `children` is a render-prop given `close`, so an action dismisses the menu after firing.
//
// T87 — the panel is rendered in a PORTAL on document.body at position:fixed, positioned from the
// trigger's rect. It used to be `position:absolute` inside the row, so an ancestor with
// `overflow:hidden` (the Details card) clipped it away on the lower rows — a dead control. A fixed
// child of <body> has no clipping ancestor. It flips above the trigger near the viewport bottom,
// clamps horizontally, and re-tracks on scroll/resize.
import {
  useCallback,
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import { createPortal } from "react-dom";

const MARGIN = 8;

export function RowMenu({
  testId,
  label = "Actions",
  children,
}: {
  testId: string;
  label?: string;
  children: (close: () => void) => ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const [pos, setPos] = useState<{ left: number; top: number } | null>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const panelId = useId(); // links the portalled panel to the trigger for a11y (aria-controls)

  const reposition = useCallback(() => {
    const t = triggerRef.current;
    if (!t) return;
    const r = t.getBoundingClientRect();
    const panel = panelRef.current;
    const pw = panel?.offsetWidth ?? 0;
    const ph = panel?.offsetHeight ?? 0;
    // Right-aligned to the trigger (unchanged); clamped horizontally below.
    let left = r.right - pw;
    // Below the trigger by default; flip above if it would overflow the viewport bottom.
    let top = r.bottom + 4;
    if (ph > 0 && top + ph + MARGIN > window.innerHeight) {
      top = r.top - ph - 4;
    }
    top = Math.max(top, MARGIN);
    // Apply the MARGIN floor LAST so a panel wider than the viewport still clamps to MARGIN,
    // never a negative left.
    left = Math.max(MARGIN, Math.min(left, window.innerWidth - pw - MARGIN));
    setPos({ left, top });
  }, []);

  // Measure + place before paint (the panel is mounted in this commit, so offsetHeight is known),
  // so the flip decision uses the real height and there is no visible jump.
  useLayoutEffect(() => {
    if (open) reposition();
  }, [open, reposition]);

  useEffect(() => {
    if (!open) return;
    // Re-track the trigger on scroll/resize (capture:true so a nested scroll container counts).
    // NB: reposition, don't close — Playwright (and a user) may scroll the item into view as part
    // of clicking it, and closing on that scroll detaches the panel mid-click. The panel following
    // its trigger is the lesser evil (Fable's judgement-call flag; the spec allows either).
    const onScroll = () => reposition();
    const onResize = () => reposition();
    const onDown = (e: MouseEvent) => {
      const tgt = e.target as Node;
      // Trap 1: the portalled panel is NOT inside the trigger's wrapper, so treat EITHER the
      // trigger or the panel as "inside" — otherwise clicking an item reads as outside, closes
      // the menu on mousedown, and the item's click never lands (the action silently no-ops).
      if (triggerRef.current?.contains(tgt) || panelRef.current?.contains(tgt)) return;
      setOpen(false);
    };
    // Trap 2: a portalled panel's keydown does not bubble to this component, so Escape lives on
    // document while the menu is open.
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    window.addEventListener("scroll", onScroll, true); // capture: nested scroll containers count
    window.addEventListener("resize", onResize);
    document.addEventListener("mousedown", onDown);
    document.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("scroll", onScroll, true);
      window.removeEventListener("resize", onResize);
      document.removeEventListener("mousedown", onDown);
      document.removeEventListener("keydown", onKey);
    };
  }, [open, reposition]);

  return (
    <div className="row-menu">
      <button
        type="button"
        ref={triggerRef}
        className="icon-btn row-menu-trigger"
        data-testid={testId}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? panelId : undefined}
        aria-label={label}
        title={label}
        onClick={() => setOpen((o) => !o)}
      >
        ⋯
      </button>
      {open &&
        createPortal(
          <div
            className="row-menu-panel"
            role="menu"
            id={panelId}
            data-portal="row-menu"
            ref={panelRef}
            style={{ left: pos?.left ?? -9999, top: pos?.top ?? -9999 }}
          >
            {children(() => setOpen(false))}
          </div>,
          document.body,
        )}
    </div>
  );
}

// RowMenuItem — one action button inside a RowMenu. `danger` styles a destructive action.
export function RowMenuItem({
  testId,
  onClick,
  disabled,
  danger,
  children,
}: {
  testId: string;
  onClick: () => void;
  disabled?: boolean;
  danger?: boolean;
  children: ReactNode;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      className={`row-menu-item${danger ? " danger" : ""}`}
      data-testid={testId}
      disabled={disabled}
      onClick={onClick}
    >
      {children}
    </button>
  );
}

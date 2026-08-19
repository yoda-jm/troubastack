// T78 — a compact "…" overflow menu for a list row's per-item actions (rename / delete / move /
// view source). Click-outside and Escape close it; the trigger and each item carry caller-supplied
// testids. `children` is a render-prop given `close`, so an action dismisses the menu after firing.
import { useEffect, useRef, useState, type ReactNode } from "react";

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
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function onDoc(e: MouseEvent) {
      if (!wrapRef.current?.contains(e.target as Node)) setOpen(false);
    }
    document.addEventListener("mousedown", onDoc);
    return () => document.removeEventListener("mousedown", onDoc);
  }, [open]);

  return (
    <div
      className="row-menu"
      ref={wrapRef}
      onKeyDown={(e) => {
        if (e.key === "Escape") setOpen(false);
      }}
    >
      <button
        type="button"
        className="icon-btn row-menu-trigger"
        data-testid={testId}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={label}
        title={label}
        onClick={() => setOpen((o) => !o)}
      >
        ⋯
      </button>
      {open && (
        <div className="row-menu-panel" role="menu">
          {children(() => setOpen(false))}
        </div>
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

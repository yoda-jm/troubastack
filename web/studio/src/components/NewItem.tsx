import { useEffect, useRef, useState, type ReactNode } from "react";

/**
 * Progressive-disclosure creation control (T04). Content-first: renders a single
 * "+ <label>" button; on click it swaps to the inline creation form (autofocused,
 * Escape or Cancel collapses it). The revealed form keeps its own data-testids and
 * submit flow — this only gates when it is shown.
 *
 * `children` is a render-prop receiving `close`, so the form can offer a Cancel
 * button. The form is NOT auto-closed on submit (the caller clears its own inputs),
 * so several items can be added in one sitting; the user collapses explicitly.
 */
export function NewItem({
  label,
  testId,
  children,
}: {
  label: string;
  testId: string;
  children: (close: () => void) => ReactNode;
}) {
  const [open, setOpen] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    // Focus the first field of the revealed form.
    wrapRef.current?.querySelector<HTMLElement>("input, select, textarea")?.focus();
  }, [open]);

  if (!open) {
    return (
      <button type="button" className="new-item-btn" data-testid={testId} onClick={() => setOpen(true)}>
        + {label}
      </button>
    );
  }

  return (
    <div
      ref={wrapRef}
      className="new-item-open"
      onKeyDown={(e) => {
        if (e.key === "Escape") setOpen(false);
      }}
    >
      {children(() => setOpen(false))}
    </div>
  );
}

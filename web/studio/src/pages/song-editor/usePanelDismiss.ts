import { useEffect, useRef, type RefObject } from "react";

/**
 * T94 — the ONE dismissal contract for a panel that floats over the score (the Details panel AND the
 * file rail). Extracted from T89's Details-only handler so the two surfaces cannot drift: a
 * second, hand-written copy for the rail is exactly how the T87/T91 portal regression comes back.
 *
 * A panel closes on Escape, an outside click, or an outside tap. Both pointer paths and Escape cede
 * to any open `[data-portal]` overlay:
 *   - a click/tap inside a `[data-portal]` node counts as INSIDE the panel — T87 portals a file row's
 *     ⋯ menu out of the panel to <body>, and T91's dialogs live there too;
 *   - Escape is ceded to an open `[data-portal]` (it closes itself on the same keydown), so pressing
 *     Escape on a delete-layer confirmation (T83) opened from the rail closes the dialog and leaves
 *     the rail open, not the rail underneath it.
 *
 * `touchstart` is handled as well as `mousedown` because Escape does not exist on a phone (§4): there
 * the ✕ and the outside tap are the only exits. `onClose` is read through a ref so a caller passing an
 * inline closure does not re-subscribe the listeners every render.
 *
 * `outsideClick` (default true) turns the pointer arm off. The file RAIL sets it false: it is a
 * working inspector you drive WHILE editing the score — you select a canvas object to see it in the
 * rail's annotation list, and delete it via the rail's own control. The editor work surface (canvas +
 * toolbar) fills the screen, so an outside-click arm would fire on every draw/select/tool click and
 * dismiss the rail mid-task (it broke 11 inspect-while-edit specs). So the rail's exits are ✕ +
 * Escape; Details — a takeover panel you do NOT edit the canvas with — keeps all three (T89). Escape
 * is shared and safe for both. [T94 gate flag: a documented deviation from §3.3's "outside-click on
 * every surface".]
 */
export function usePanelDismiss(
  open: boolean,
  onClose: () => void,
  panelRef: RefObject<HTMLElement | null>,
  toggleTestId: string,
  outsideClick = true,
) {
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  useEffect(() => {
    if (!open) return;
    const close = () => onCloseRef.current();
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== "Escape") return;
      if (document.querySelector("[data-portal]")) return; // a portalled overlay owns Escape
      close();
    };
    const onOutside = (e: Event) => {
      const t = e.target as HTMLElement | null;
      if (panelRef.current?.contains(t)) return;
      if (t?.closest?.(`[data-testid="${toggleTestId}"]`)) return; // the pill handles its own click
      if (t?.closest?.("[data-portal]")) return; // a portalled child counts as inside
      close();
    };
    document.addEventListener("keydown", onKey);
    if (outsideClick) {
      document.addEventListener("mousedown", onOutside);
      document.addEventListener("touchstart", onOutside);
    }
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onOutside);
      document.removeEventListener("touchstart", onOutside);
    };
  }, [open, panelRef, toggleTestId, outsideClick]);
}

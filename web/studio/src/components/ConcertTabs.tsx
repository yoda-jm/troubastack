import { NavLink } from "react-router-dom";

/**
 * T206 — the concert's tab strip: "Running order" | "Sound sheet". Underline tabs (the .drawer-tab family,
 * not the .details-tab pills). NavLinks, so each tab is a real route: /sheet stays deep-linkable, the
 * browser Back button works, and NavLink marks the active one (the `active` class + aria-current="page")
 * on its own. `end` on the running-order tab keeps it from reading active while the /sheet child is open.
 */
export function ConcertTabs({ bandId, setlistId }: { bandId: string; setlistId: string }) {
  const base = `/bands/${bandId}/setlists/${setlistId}`;
  const cls = ({ isActive }: { isActive: boolean }) => "concert-tab" + (isActive ? " active" : "");
  return (
    <nav className="concert-tabs" aria-label="Concert views">
      <NavLink to={base} end className={cls} data-testid="concert-tab-order">
        Running order
      </NavLink>
      <NavLink to={`${base}/sheet`} className={cls} data-testid="concert-tab-sheet">
        Sound sheet
      </NavLink>
    </nav>
  );
}

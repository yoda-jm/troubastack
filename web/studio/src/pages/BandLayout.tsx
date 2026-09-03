import { useCallback, useEffect, useState } from "react";
import { Link, Outlet, useLocation, useOutletContext, useParams } from "react-router-dom";
import { ApiError, api, type Band, type Role } from "../api";
import { ErrorBanner } from "../components/ErrorBanner";
import { SectionTabs, type BandSection } from "../components/SectionTabs";

/**
 * T130 — Overview, Setlists and Settings are TABS OF ONE BAND, so they share one layout route:
 * this element owns the crumb, the tab strip and a SINGLE band fetch, and renders the active section
 * through <Outlet/>. Switching tabs no longer unmounts the page (the crumb + strip persist, the band
 * is fetched once, and myRole stops flickering), and the crumb is defined here ONCE so it can never
 * diverge across the three again.
 */
export type BandContext = {
  band: Band;
  myRole: Role;
  reload: () => Promise<void>;
  setBand: (b: Band) => void;
};

/** Section pages read the shared band + role from here instead of fetching their own. */
export function useBand(): BandContext {
  return useOutletContext<BandContext>();
}

function activeSection(pathname: string): BandSection {
  if (pathname.endsWith("/settings")) return "settings";
  if (pathname.endsWith("/setlists")) return "setlists";
  return "overview";
}

export function BandLayout() {
  const { bandId } = useParams<{ bandId: string }>();
  const { pathname } = useLocation();
  const [band, setBand] = useState<Band | null>(null);
  const [myRole, setMyRole] = useState<Role | null>(null);
  const [error, setError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    if (!bandId) return;
    try {
      const res = await api.getBand(bandId);
      setBand(res.band);
      setMyRole(res.myRole);
      setError(null);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Failed to load band");
    }
  }, [bandId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  // The crumb is defined ONCE, here. Since the three sections are tabs of one band, "up" from any of
  // them is the bands list — one label, one destination, and no per-page copy to diverge (T130).
  return (
    <div className="page">
      <Link className="crumb" to="/bands">
        &larr; Bands
      </Link>
      {!band || !bandId || myRole === null ? (
        // Loading/error live BELOW the crumb — the crumb never blanks. The tab strip only appears once
        // the band (and myRole, which gates Settings) is known, so it never flickers on first load.
        error ? (
          <ErrorBanner message={error} />
        ) : (
          <div className="muted" data-testid="band-loading">
            Loading…
          </div>
        )
      ) : (
        <>
          <SectionTabs bandId={bandId} active={activeSection(pathname)} showSettings={myRole === "admin"} />
          <Outlet context={{ band, myRole, reload, setBand } satisfies BandContext} />
        </>
      )}
    </div>
  );
}

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
  if (pathname.endsWith("/lineups")) return "lineups";
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
  const section = activeSection(pathname);
  // T211: the Line-ups route fills the window — the page becomes a full-height flex column (.page-fill) and
  // the crumb shares ONE compact row with the section tabs (the visible "Line-ups" h1 goes; Lineups keeps a
  // screen-reader one). Every other tab keeps today's layout: crumb on its own line, then the tab strip.
  const isLineups = section === "lineups";
  const loaded = band && bandId && myRole !== null;
  const crumb = (
    <Link className="crumb" to="/bands">
      &larr; Bands
    </Link>
  );
  return (
    <div className={"page" + (isLineups ? " page-fill" : "")}>
      {isLineups ? (
        <div className="band-toprow">
          {crumb}
          {loaded && <SectionTabs bandId={bandId} active="lineups" showSettings={myRole === "admin"} />}
        </div>
      ) : (
        crumb
      )}
      {!loaded ? (
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
          {!isLineups && <SectionTabs bandId={bandId} active={section} showSettings={myRole === "admin"} />}
          <Outlet context={{ band, myRole, reload, setBand } satisfies BandContext} />
        </>
      )}
    </div>
  );
}

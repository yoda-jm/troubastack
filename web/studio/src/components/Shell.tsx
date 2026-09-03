/**
 * App shell for authenticated pages: shows the current user, a link to pending
 * invites, a logout button, and renders the active route via <Outlet>. Also acts
 * as the auth guard — if there is no user once loading finishes, redirect to
 * /login (the GET /api/me 401 path).
 */
import { useEffect, useState } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import { api } from "../api";
import { useAuth } from "../auth";
import { AccountMenu } from "./AccountMenu";
import { ErrorBoundary } from "./ErrorBoundary";
import { GlobalError } from "./GlobalError";

// T46: Studio embedded mode. The Android app hosts Studio in a WebView (I10, A06) and
// opts in via `?embedded=1` on the entry URL — a param, NOT the JS bridge, so the nav
// never flashes (the bridge handshake lands after first paint) and it's testable in
// plain Playwright. Persisted to sessionStorage so it survives SPA navigation (the param
// is only on the first load). When set we suppress the app topbar (its Bands/Invites/
// profile/Log out duplicate the app's own chrome and read as an embedded browser); and
// the Log out + account affordances go with it — the app owns the session it cookie-
// seeds, so an in-WebView logout would silently break it.
const EMBEDDED_KEY = "trouba_embedded";
function studioEmbedded(): boolean {
  try {
    if (new URLSearchParams(window.location.search).get("embedded") === "1") {
      sessionStorage.setItem(EMBEDDED_KEY, "1");
    }
    return sessionStorage.getItem(EMBEDDED_KEY) === "1";
  } catch {
    return new URLSearchParams(window.location.search).get("embedded") === "1";
  }
}

export function Shell() {
  const { user, loading, logout } = useAuth();
  const navigate = useNavigate();
  const location = useLocation();
  const [pendingCount, setPendingCount] = useState<number | null>(null);

  useEffect(() => {
    if (!loading && !user) {
      // Preserve the destination so the user returns here after authenticating.
      const next = location.pathname + location.search;
      const suffix = next && next !== "/bands" ? `?next=${encodeURIComponent(next)}` : "";
      navigate(`/login${suffix}`, { replace: true });
    }
  }, [loading, user, navigate, location]);

  useEffect(() => {
    if (!user) return;
    void api
      .listInvites()
      .then((invites) => setPendingCount(invites.length))
      .catch(() => setPendingCount(null));
  }, [user]);

  // A11y-scoped viewport (arch audit 2026-07-10, note #3 → WCAG 1.4.4): the editor
  // needs `user-scalable=no` so its in-app pinch (T27 stage 4) owns the gesture, but
  // the management pages have no in-app zoom, so disabling browser pinch there hurts
  // low-vision users. Scope the restriction to the editor route; everywhere else is
  // zoomable. (index.html ships the zoomable default for the first paint.)
  useEffect(() => {
    const editor = /\/bands\/[^/]+\/songs\/[^/]+/.test(location.pathname);
    const meta = document.querySelector('meta[name="viewport"]');
    if (!meta) return;
    meta.setAttribute(
      "content",
      editor
        ? "width=device-width, initial-scale=1.0, user-scalable=no"
        : "width=device-width, initial-scale=1.0",
    );
  }, [location.pathname]);

  if (loading) {
    return <div className="center">Loading…</div>;
  }
  if (!user) {
    // Redirect effect will fire; render nothing meanwhile.
    return null;
  }

  // The song editor is a full-bleed, canvas-first surface (T27 stage 3): hide the app
  // top bar so the score owns the whole viewport (also the mobile win). Back-nav lives
  // in the editor's own floating chrome.
  const fullbleed = /\/bands\/[^/]+\/songs\/[^/]+/.test(location.pathname);
  // Embedded (in the app's WebView): drop the app-duplicating chrome entirely. (T46)
  const embedded = studioEmbedded();

  return (
    <div className={`shell${fullbleed ? " shell-fullbleed" : ""}${embedded ? " shell-embedded" : ""}`}>
      {/* Global backstop: any uncaught error / rejection anywhere becomes visible (T32). */}
      <GlobalError />
      {!fullbleed && !embedded && (
      <header className="topbar">
        <Link to="/bands" className="brand">
          {/* BRAND08: the compact mark BESIDE the name (not instead of it) — the text stays the
              accessible, selectable, translatable name; the mark is decorative here. Ground-independent
              (one asset), served from docs/brand/dist by the brandAssets Vite plugin. */}
          <img className="brand-mark" src="/troubastudio-compact.svg" alt="" aria-hidden="true" width="24" height="24" />
          <span className="brand-name">TroubaStudio</span>
        </Link>
        <nav className="nav">
          {/* BRAND08: the masthead (mark + name) already links to /bands, so a separate "Bands" nav
              item duplicated that route — removed now that the mark makes the masthead the clear home
              affordance (VLL's ruling). Invites stays: it's a distinct route with its own badge. */}
          <Link to="/invites" data-testid="nav-invites">
            Invites
            {pendingCount ? <span className="badge" data-testid="invite-badge">{pendingCount}</span> : null}
          </Link>
        </nav>
        <div className="user">
          <AccountMenu
            user={user}
            onLogout={async () => {
              await logout();
              navigate("/login", { replace: true });
            }}
          />
        </div>
      </header>
      )}
      <main className="content">
        {/* Render-crash boundary around the routed page (T32). */}
        <ErrorBoundary>
          <Outlet />
        </ErrorBoundary>
      </main>
    </div>
  );
}

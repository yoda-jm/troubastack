/**
 * App shell for authenticated pages: shows the current user, a link to pending
 * invites, a logout button, and renders the active route via <Outlet>. Also acts
 * as the auth guard — if there is no user once loading finishes, redirect to
 * /login (the GET /api/me 401 path).
 */
import { useEffect, useRef, useState } from "react";
import { Link, Outlet, useLocation, useNavigate } from "react-router-dom";
import QRCode from "qrcode";
import { api, type AppBinary } from "../api";
import { useAuth } from "../auth";
import { Avatar } from "./Avatar";
import { ErrorBoundary } from "./ErrorBoundary";
import { GlobalError } from "./GlobalError";

type ServerVersion = { version: string; builtAt: string; spaEmbedded: boolean };

/**
 * T29 — build-identity chip: shows the SPA bundle's baked git version; clicking it
 * fetches GET /api/version and shows the server's version/build time alongside.
 * If the two DIFFER, a warning line flags it — the stale-browser-cache / stale-build
 * detector (two field incidents on 2026-07-10 motivated this). Display only; no
 * compatibility enforcement.
 */
function VersionChip() {
  const [open, setOpen] = useState(false);
  const [server, setServer] = useState<ServerVersion | null>(null);
  const [error, setError] = useState(false);

  async function toggle() {
    const next = !open;
    setOpen(next);
    if (next && !server) {
      try {
        const res = await fetch("/api/version");
        setServer((await res.json()) as ServerVersion);
      } catch {
        setError(true);
      }
    }
  }

  const mismatch = server != null && server.version !== __APP_VERSION__;
  return (
    <span className="version-chip-wrap">
      <button type="button" className="version-chip" data-testid="version-chip" onClick={() => void toggle()}>
        {__APP_VERSION__}
      </button>
      {open && (
        <div className="version-popover" data-testid="version-popover">
          <div className="mono">Studio&nbsp;&nbsp;{__APP_VERSION__}</div>
          {server ? (
            <>
              <div className="mono" data-testid="version-server">
                Server&nbsp;&nbsp;{server.version} · {server.builtAt}
                {!server.spaEmbedded && " · no SPA embedded"}
              </div>
              {mismatch && (
                <div className="version-mismatch" data-testid="version-mismatch" role="alert">
                  ⚠ Studio and server versions differ — reload (Ctrl+Shift+R) or rebuild the server.
                </div>
              )}
            </>
          ) : (
            <div className="mono muted">{error ? "server version unavailable" : "…"}</div>
          )}
        </div>
      )}
    </span>
  );
}

// GetAppChip (OPS02 — VLL placement re-scope): a topbar "Get the app" affordance
// (top-right, next to the version chip) so a band member installs the native app
// straight from the server on ANY page — a QR (bandleader-screen → member-camera)
// + a tap-to-download button in a popover (mirrors the version-chip pattern). It is
// hidden entirely when /api/apps is empty (dev / no-embed image), and the topbar
// itself is suppressed in the fullscreen editor + embedded WebView, so the
// visibility set is exactly "every normal page, not the editor" for free.
//
// The iOS row reads "Coming soon" (greyed, inert) until an `ios` entry rides the
// manifest, at which point the SAME row flips to a live download (VLL amendment).
function GetAppChip() {
  const [apps, setApps] = useState<AppBinary[]>([]);
  const [open, setOpen] = useState(false);
  const qrRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    let cancelled = false;
    api
      .listApps()
      .then((a) => {
        if (!cancelled) setApps(a);
      })
      .catch(() => {
        /* no apps endpoint / none embedded — the chip stays hidden */
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const android = apps.find((a) => a.platform === "android");
  const ios = apps.find((a) => a.platform === "ios");
  const primary = android ?? ios; // the QR points at the primary download
  const url = primary ? window.location.origin + primary.path : "";

  useEffect(() => {
    if (!open || !url) return;
    let cancelled = false;
    QRCode.toString(url, { type: "svg", margin: 1, width: 132 })
      .then((svg) => {
        if (!cancelled && qrRef.current) qrRef.current.innerHTML = svg;
      })
      .catch(() => {
        if (!cancelled && qrRef.current) qrRef.current.textContent = url;
      });
    return () => {
      cancelled = true;
    };
  }, [open, url]);

  if (apps.length === 0) return null;

  const meta = (a: AppBinary) => `${a.version} · ${(a.size / 1e6).toFixed(1)} MB`;

  return (
    <span className="getapp-wrap">
      <button
        type="button"
        className="getapp-chip"
        data-testid="get-app-btn"
        aria-expanded={open}
        onClick={() => setOpen((o) => !o)}
      >
        <span aria-hidden="true">📱</span> Get the app
      </button>
      {open && (
        <div className="getapp-popover" data-testid="get-app-popover">
          <div className="getapp-title">Get TroubaStage</div>
          <p className="getapp-sub">
            Perform offline on your phone or tablet. Scan the code with your phone camera, or
            tap to download.
          </p>
          {url && <div className="qr" data-testid="get-app-qr" ref={qrRef} />}
          <ul className="getapp-platforms">
            {android && (
              <li className="getapp-platform" data-testid="get-app-android">
                <a
                  className="primary button"
                  data-testid="get-app-download"
                  href={android.path}
                  download={android.filename}
                >
                  Download for Android
                </a>
                <span className="getapp-meta" data-testid="get-app-version">
                  {meta(android)} · Android
                </span>
              </li>
            )}
            <li className="getapp-platform" data-testid="get-app-ios">
              {ios ? (
                <>
                  <a
                    className="primary button"
                    data-testid="get-app-ios-download"
                    href={ios.path}
                    download={ios.filename}
                  >
                    Download for iOS
                  </a>
                  <span className="getapp-meta">{meta(ios)} · iOS</span>
                </>
              ) : (
                <>
                  <span
                    className="button is-disabled"
                    data-testid="get-app-ios-soon"
                    aria-disabled="true"
                  >
                    iOS <span className="coming-soon">Coming soon</span>
                  </span>
                  <span className="getapp-meta">Available in a future release</span>
                </>
              )}
            </li>
          </ul>
        </div>
      )}
    </span>
  );
}

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
          TroubaStudio
        </Link>
        <nav className="nav">
          <Link to="/bands">Bands</Link>
          <Link to="/invites" data-testid="nav-invites">
            Invites
            {pendingCount ? <span className="badge" data-testid="invite-badge">{pendingCount}</span> : null}
          </Link>
        </nav>
        <div className="user">
          <GetAppChip />
          <VersionChip />
          <Link to="/me" className="profile-link" data-testid="nav-profile">
            <Avatar user={user} size={26} />
            <span data-testid="current-user">{user.displayName}</span>
          </Link>
          <button
            type="button"
            data-testid="logout"
            onClick={async () => {
              await logout();
              navigate("/login", { replace: true });
            }}
          >
            Log out
          </button>
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

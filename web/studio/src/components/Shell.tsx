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
import { Avatar } from "./Avatar";

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

  return (
    <div className={`shell${fullbleed ? " shell-fullbleed" : ""}`}>
      {!fullbleed && (
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
        <Outlet />
      </main>
    </div>
  );
}

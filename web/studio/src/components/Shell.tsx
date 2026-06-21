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

  return (
    <div className="shell">
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
      <main className="content">
        <Outlet />
      </main>
    </div>
  );
}

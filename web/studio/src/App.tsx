/**
 * Route table for the non-canvas Studio pages. The canvas/annotation editor at
 * /bands/:bandId/songs/:songId is a deferred placeholder (see SongEditor).
 */
import { Suspense } from "react";
import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { Login } from "./pages/Login";
import { Register } from "./pages/Register";
import { ResetPassword } from "./pages/ResetPassword";
import { Bands } from "./pages/Bands";
import { BandLayout } from "./pages/BandLayout";
import { BandDetail } from "./pages/BandDetail";
import { BandSettings } from "./pages/BandSettings";
import { Setlists } from "./pages/Setlists";
import { SetlistDetail } from "./pages/SetlistDetail";
import { Invites } from "./pages/Invites";
import { Profile } from "./pages/Profile";
import { Join } from "./pages/Join";
import { RouteFallback, RouteErrorBoundary } from "./components/RouteBoundary";

// The editor routes (lazy-loaded pages + their paths) live in one table so the router and Shell cannot
// disagree about which routes are full-bleed (T184). App mounts them; Shell reads the same entries.
import { EDITOR_ROUTES } from "./routes";

export function App() {
  return (
    <RouteErrorBoundary>
      <Suspense fallback={<RouteFallback />}>
        <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
      {/* Public: a one-time reset link lands here (the token is the credential). */}
      <Route path="/reset-password/:token" element={<ResetPassword />} />

      {/* Authenticated area — Shell enforces the auth guard. */}
      <Route element={<Shell />}>
        <Route path="/bands" element={<Bands />} />
        {/* T130: Overview / Setlists / Settings are tabs of ONE band — a shared layout owns the
            crumb, the tab strip and a single band fetch; the sections render through its Outlet. */}
        <Route path="/bands/:bandId" element={<BandLayout />}>
          <Route index element={<BandDetail />} />
          <Route path="setlists" element={<Setlists />} />
          <Route path="settings" element={<BandSettings />} />
        </Route>
        <Route path="/bands/:bandId/setlists/:setlistId" element={<SetlistDetail />} />
        {/* The editor routes come from ONE table (routes.tsx) that Shell also derives full-bleed from, so a
            route cannot be mounted without being full-bleed (T184). Flat song address, the T175 ⟨D5⟩ setlist
            address, and the T105 chart editor — all documented beside the table. */}
        {EDITOR_ROUTES.map((r) => (
          <Route key={r.path} path={r.path} element={r.element} />
        ))}
        <Route path="/invites" element={<Invites />} />
        <Route path="/me" element={<Profile />} />
        <Route path="/join/:token" element={<Join />} />
      </Route>

          <Route path="/" element={<Navigate to="/bands" replace />} />
          <Route path="*" element={<Navigate to="/bands" replace />} />
        </Routes>
      </Suspense>
    </RouteErrorBoundary>
  );
}

/**
 * Route table for the non-canvas Studio pages. The canvas/annotation editor at
 * /bands/:bandId/songs/:songId is a deferred placeholder (see SongEditor).
 */
import { lazy, Suspense } from "react";
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

// T112: the annotation editor + its chart-editor route pull in pdf.js and the whole drawing canvas —
// ~half the bundle, and code that nobody reaching /login needs. Load them only when an editor route is
// actually visited, behind the Suspense boundary below.
const SongEditor = lazy(() => import("./pages/SongEditor").then((m) => ({ default: m.SongEditor })));
const ChartEditorPage = lazy(() =>
  import("./pages/ChartEditorPage").then((m) => ({ default: m.ChartEditorPage })),
);

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
        <Route path="/bands/:bandId/songs/:songId" element={<SongEditor />} />
        {/* T105: the dedicated full-page chart editor, reachable from the viewer and linkable. */}
        <Route path="/bands/:bandId/songs/:songId/chart/:fileId" element={<ChartEditorPage />} />
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

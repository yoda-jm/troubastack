/**
 * Route table for the non-canvas Studio pages. The canvas/annotation editor at
 * /bands/:bandId/songs/:songId is a deferred placeholder (see SongEditor).
 */
import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { Login } from "./pages/Login";
import { Register } from "./pages/Register";
import { Bands } from "./pages/Bands";
import { BandDetail } from "./pages/BandDetail";
import { SongEditor } from "./pages/SongEditor";
import { Invites } from "./pages/Invites";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />

      {/* Authenticated area — Shell enforces the auth guard. */}
      <Route element={<Shell />}>
        <Route path="/bands" element={<Bands />} />
        <Route path="/bands/:bandId" element={<BandDetail />} />
        <Route path="/bands/:bandId/songs/:songId" element={<SongEditor />} />
        <Route path="/invites" element={<Invites />} />
      </Route>

      <Route path="/" element={<Navigate to="/bands" replace />} />
      <Route path="*" element={<Navigate to="/bands" replace />} />
    </Routes>
  );
}

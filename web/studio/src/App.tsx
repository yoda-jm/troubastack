/**
 * Route table for the non-canvas Studio pages. The canvas/annotation editor at
 * /bands/:bandId/songs/:songId is a deferred placeholder (see SongEditor).
 */
import { Navigate, Route, Routes } from "react-router-dom";
import { Shell } from "./components/Shell";
import { Login } from "./pages/Login";
import { Register } from "./pages/Register";
import { ResetPassword } from "./pages/ResetPassword";
import { Bands } from "./pages/Bands";
import { BandDetail } from "./pages/BandDetail";
import { BandSettings } from "./pages/BandSettings";
import { SongEditor } from "./pages/SongEditor";
import { Setlists } from "./pages/Setlists";
import { SetlistDetail } from "./pages/SetlistDetail";
import { Invites } from "./pages/Invites";
import { Profile } from "./pages/Profile";
import { Join } from "./pages/Join";

export function App() {
  return (
    <Routes>
      <Route path="/login" element={<Login />} />
      <Route path="/register" element={<Register />} />
      {/* Public: a one-time reset link lands here (the token is the credential). */}
      <Route path="/reset-password/:token" element={<ResetPassword />} />

      {/* Authenticated area — Shell enforces the auth guard. */}
      <Route element={<Shell />}>
        <Route path="/bands" element={<Bands />} />
        <Route path="/bands/:bandId" element={<BandDetail />} />
        <Route path="/bands/:bandId/settings" element={<BandSettings />} />
        <Route path="/bands/:bandId/setlists" element={<Setlists />} />
        <Route path="/bands/:bandId/setlists/:setlistId" element={<SetlistDetail />} />
        <Route path="/bands/:bandId/songs/:songId" element={<SongEditor />} />
        <Route path="/invites" element={<Invites />} />
        <Route path="/me" element={<Profile />} />
        <Route path="/join/:token" element={<Join />} />
      </Route>

      <Route path="/" element={<Navigate to="/bands" replace />} />
      <Route path="*" element={<Navigate to="/bands" replace />} />
    </Routes>
  );
}

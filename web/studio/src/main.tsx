/**
 * TroubaStudio SPA entry. Client-rendered (no SSR) so core can serve the built
 * assets directly from embed.FS (I10, I14). Mounts the router + auth provider
 * into #app. The canvas editor (deferred) plugs into the SongEditor route.
 */
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter } from "react-router-dom";
import { App } from "./App";
import { AuthProvider } from "./auth";
import "./styles.css";

const el = document.getElementById("app");
if (!el) throw new Error("missing #app mount point");

createRoot(el).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <App />
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);

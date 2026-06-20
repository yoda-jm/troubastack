/**
 * Vite config for TroubaStudio.
 *
 * Builds a client-rendered static SPA into dist/, which `core` embeds
 * (embed.FS) and serves directly — no Node runtime in production (I10, I14).
 *
 * Dev server proxies /api → the Go core on :8080 so the SPA and API are
 * same-origin to the browser; the HttpOnly trouba_session cookie then "just
 * works" with fetch credentials:'include'.
 */
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const API_TARGET = process.env.TROUBA_API_TARGET ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: API_TARGET,
        changeOrigin: true,
      },
    },
  },
  preview: {
    port: 5173,
    strictPort: true,
    proxy: {
      "/api": {
        target: API_TARGET,
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: "dist",
  },
});

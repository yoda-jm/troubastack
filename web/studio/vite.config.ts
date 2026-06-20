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
import { fileURLToPath } from "node:url";

const API_TARGET = process.env.TROUBA_API_TARGET ?? "http://localhost:8080";

// studio installs with --no-workspaces, so @troubastack/ink (the one renderer,
// I8) is not in node_modules. Alias it straight to the sibling package source;
// Vite bundles it (and its perfect-freehand dep, installed locally here too).
const inkSrc = fileURLToPath(new URL("../ink/src/index.ts", import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      "@troubastack/ink": inkSrc,
    },
  },
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

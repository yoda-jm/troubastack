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
import { execSync } from "node:child_process";

const API_TARGET = process.env.TROUBA_API_TARGET ?? "http://localhost:8080";

// T29: bake the git version into the bundle so the UI can show its own build and
// flag a mismatch against the server's /api/version (the stale-cache detector).
// Dev servers / builds outside a git checkout report "dev".
function gitVersion(): string {
  try {
    return execSync("git describe --always --dirty", { stdio: ["ignore", "pipe", "ignore"] })
      .toString()
      .trim();
  } catch {
    return "dev";
  }
}

// studio installs with --no-workspaces, so @troubastack/ink (the one renderer,
// I8) is not in node_modules. Alias it straight to the sibling package source;
// Vite bundles it (and its perfect-freehand dep, installed locally here too).
const inkSrc = fileURLToPath(new URL("../ink/src/index.ts", import.meta.url));

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(gitVersion()),
  },
  plugins: [react()],
  resolve: {
    alias: {
      "@troubastack/ink": inkSrc,
    },
  },
  server: {
    port: 5173,
    strictPort: true,
    // TROUBA_NO_HMR disables hot-module reload — used by the DEMO-VID walkthrough recorder so
    // HMR websocket churn can't schedule spurious navigations mid-capture.
    hmr: process.env.TROUBA_NO_HMR ? false : undefined,
    proxy: {
      "/api": {
        target: API_TARGET,
        changeOrigin: true,
        // The realtime editor opens a WebSocket at …/songs/:id/ws; let the
        // HTTP→WS upgrade pass through to the Go core in dev/e2e.
        ws: true,
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
        ws: true,
      },
    },
  },
  build: {
    outDir: "dist",
  },
});

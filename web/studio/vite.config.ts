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
import { defineConfig, type Plugin } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";
import { execSync } from "node:child_process";
import { readFileSync } from "node:fs";

const API_TARGET = process.env.TROUBA_API_TARGET ?? "http://localhost:8080";

// BRAND03: the tab favicon is the studio mark, and the mark has exactly one source of
// truth — docs/brand/dist/troubastudio-minimal.svg. Rather than commit a second copy
// (which drifts from the bricks the moment the brand changes, the way web/site's
// build.sh warns against), this plugin serves it in dev and COPIES it into the build.
// The PNG raster fallback stays in public/ (SVG favicons don't cover every browser).
function brandFavicon(): Plugin {
  const FAVICON = "troubastudio-minimal.svg";
  const src = fileURLToPath(new URL(`../../docs/brand/dist/${FAVICON}`, import.meta.url));
  return {
    name: "trouba-brand-favicon",
    configureServer(server) {
      server.middlewares.use(`/${FAVICON}`, (_req, res) => {
        res.setHeader("Content-Type", "image/svg+xml");
        res.end(readFileSync(src));
      });
    },
    generateBundle() {
      this.emitFile({ type: "asset", fileName: FAVICON, source: readFileSync(src) });
    },
  };
}

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
  plugins: [react(), brandFavicon()],
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
    // T112: split so app code and heavy deps cache independently across deploys, and so pdf.js is its
    // own chunk pulled in only with the (lazy) editor route rather than riding the entry bundle.
    rollupOptions: {
      output: {
        manualChunks: {
          vendor: ["react", "react-dom", "react-router-dom"],
          pdfjs: ["pdfjs-dist"],
        },
      },
    },
    // Set to a number the build actually MEETS (T112 §2d) — the pdf.js chunk is legitimately large and
    // now off the initial path; this is not a limit chosen to silence a warning on shipped-to-/login code.
    chunkSizeWarningLimit: 450,
  },
});

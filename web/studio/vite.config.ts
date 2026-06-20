/**
 * Vite config for TroubaStudio.
 *
 * Builds a client-rendered static SPA into dist/, which `core` embeds
 * (embed.FS) and serves directly — no Node runtime in production (I10, I14,
 * 06-tech-stack.md). Avoid SSR (no Next.js-style server rendering).
 *
 * Vite is not installed yet (scaffold, no network), so this stub avoids
 * importing 'vite' types and just exports a plain config object.
 */

// TODO: `import { defineConfig } from "vite"` once deps are installed.
export default {
  // base must match where core mounts the SPA so asset URLs resolve when embedded.
  // resolve.alias maps @troubastack/ink to the workspace source during dev.
  // TODO: configure build.outDir, base, and the @troubastack/ink alias.
};

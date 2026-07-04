// Build step for @troubastack/bake.
//
// Bundling (not tsc project references) is how we honor the I8 "one renderer"
// rule at build time: esbuild inlines @troubastack/ink's SOURCE into each output,
// so there is never a second *compiled copy* of ink on disk (the trap the old
// --noEmit stub called out). The alias below is the single place the ink source
// path is wired for esbuild; tsc gets the same mapping from tsconfig `paths`.
//
// Two node outputs — dist/index.js (the library: renderOverlays + types) and
// dist/cli.js (the process core spawns). @napi-rs/canvas is a native prebuilt
// addon, so it stays EXTERNAL and is required from node_modules at runtime.
//
// The browser parity bundle is NOT built here; test/parity.test.mjs builds it
// in-process (browserBuildOptions) so the test is self-contained.

import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { build } from "esbuild";

const here = dirname(fileURLToPath(import.meta.url));

/** The one wiring of the ink source path for esbuild (mirrors tsconfig paths). */
export const inkAlias = { "@troubastack/ink": resolve(here, "../ink/src/index.ts") };

/** esbuild options for the browser parity bundle: inline EVERYTHING (ink +
 *  perfect-freehand), no node/native deps, so it loads in bare Chromium. */
export const browserBuildOptions = {
  entryPoints: [resolve(here, "test/browser-entry.ts")],
  bundle: true,
  format: "iife",
  platform: "browser",
  target: "es2022",
  alias: inkAlias,
  write: false,
};

const nodeCommon = {
  bundle: true,
  format: "esm",
  platform: "node",
  target: "node24",
  // Native addon — never bundle it; resolve from node_modules at runtime.
  external: ["@napi-rs/canvas"],
  alias: inkAlias,
  logLevel: "info",
};

if (import.meta.url === `file://${process.argv[1]}`) {
  await build({
    ...nodeCommon,
    entryPoints: [resolve(here, "src/index.ts")],
    outfile: resolve(here, "dist/index.js"),
  });
  await build({
    ...nodeCommon,
    entryPoints: [resolve(here, "src/cli.ts")],
    outfile: resolve(here, "dist/cli.js"),
    banner: { js: "#!/usr/bin/env node" },
  });
}

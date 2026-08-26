/// <reference types="vitest/config" />
import { mergeConfig, defineConfig } from "vitest/config";
import viteConfig from "./vite.config";

// T110: reuse vite's resolve/alias (so tests can import @troubastack/ink from source) and add the test
// block. The DEFAULT environment stays `node` — the pure-function suites (*.test.ts) need no DOM and
// prove it by staying fast there. T119 adds component tests (*.test.tsx) that opt INTO jsdom per file via
// a `// @vitest-environment jsdom` docblock — so both environments run in one `vitest run`, node stays
// the default, and a DOM is spun up only for the handful of tests that actually assert rendered output.
export default mergeConfig(viteConfig, defineConfig({
  test: {
    environment: "node",
    include: ["test/**/*.test.ts", "test/**/*.test.tsx"],
  },
}));

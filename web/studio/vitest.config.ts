/// <reference types="vitest/config" />
import { mergeConfig, defineConfig } from "vitest/config";
import viteConfig from "./vite.config";

// T110: reuse vite's resolve/alias (so tests can import @troubastack/ink from source) and add the test
// block. A node environment is enough — the first suite is pure functions, no DOM.
export default mergeConfig(viteConfig, defineConfig({
  test: {
    environment: "node",
    include: ["test/**/*.test.ts"],
  },
}));

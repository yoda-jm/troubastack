/**
 * FLOW CHECK — asserts the full build-a-concert flow end to end and screenshots every step.
 * Isolated backend :8091 + vite :5274 + its own data dir. NEVER touches the :8080 demo.
 * Run: npx playwright test -c playwright.flowcheck.config.ts
 */
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./flowcheck",
  testMatch: /flowcheck\.spec\.ts/,
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  timeout: 600_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:5274",
    viewport: { width: 1280, height: 800 },
    trace: "retain-on-failure",
    actionTimeout: 12_000,
    navigationTimeout: 20_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1280, height: 800 } } }],
  webServer: [
    {
      command:
        "cd ../../core && rm -rf ./troubadata-flowcheck && " +
        "TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_NO_MDNS=1 " +
        "TROUBA_DATA_DIR=./troubadata-flowcheck TROUBACORE_ADDR=:8091 go run ./cmd/troubacore",
      url: "http://localhost:8091/healthz",
      reuseExistingServer: false,
      timeout: 180_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      command:
        "TROUBA_API_TARGET=http://localhost:8091 TROUBA_NO_HMR=1 npm run dev -- --port 5274 --strictPort",
      url: "http://localhost:5274",
      reuseExistingServer: false,
      timeout: 90_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});

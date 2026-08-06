/**
 * DEMO-VID Part B — web walkthrough RECORDING config (separate from the e2e config).
 *
 * Drives the SEEDED demo (the shipped seed data — real songs, charts, annotations, orchestra)
 * through the storyboard and records 1920×1080 video per scene. Unlike the e2e config, the
 * backend is SEEDED (via globalSetup) so scenes 6–14 can show the real Open Road annotations,
 * the setlist/bake, and the orchestra parts.
 *
 * Run:  npx playwright test -c playwright.walkthrough.config.ts
 * Out:  walkthrough/output/<scene>/*.webm  (one video per scene test)
 */
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./walkthrough",
  testMatch: /walkthrough\.spec\.ts/,
  globalSetup: "./walkthrough/global-setup.ts",
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [["list"]],
  timeout: 180_000,
  expect: { timeout: 15_000 },
  use: {
    baseURL: "http://localhost:5173",
    viewport: { width: 1920, height: 1080 },
    video: { mode: "on", size: { width: 1920, height: 1080 } },
    trace: "retain-on-failure",
    // Fail individual actions fast so a missed selector inside soft() throws (and is skipped)
    // rather than hanging until the test timeout.
    actionTimeout: 12_000,
    navigationTimeout: 20_000,
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"], viewport: { width: 1920, height: 1080 } } }],
  webServer: [
    {
      // Seeded backend: file store + a temp data dir so the seed's uploaded PDFs persist for
      // the bake scene. globalSetup runs the seed once the server is up.
      command:
        "cd ../../core && TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_NO_MDNS=1 " +
        "TROUBA_DATA_DIR=./troubadata-walkthrough TROUBACORE_ADDR=:8080 go run ./cmd/troubacore",
      url: "http://localhost:8080/healthz",
      reuseExistingServer: true,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      command: "npm run dev",
      url: "http://localhost:5173",
      reuseExistingServer: true,
      timeout: 60_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});

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
    baseURL: "http://localhost:5273",
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
      // ISOLATED empty backend on :8090 (NOT the persistent :8080 demo). A fresh data dir is
      // wiped on every run so the tour starts from a truly empty server and builds The
      // Troubadours live; globalSetup then seeds ONLY the orchestra. reuseExistingServer is
      // OFF so we never accidentally record against a stale/populated instance.
      command:
        "cd ../../core && rm -rf ./troubadata-walkthrough && " +
        "TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_NO_MDNS=1 " +
        "TROUBA_DATA_DIR=./troubadata-walkthrough TROUBACORE_ADDR=:8090 go run ./cmd/troubacore",
      url: "http://localhost:8090/healthz",
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      // Its own Vite on :5273, proxying /api → the isolated :8090 backend. HMR off so websocket
      // churn can't schedule spurious navigations mid-capture.
      command:
        "TROUBA_API_TARGET=http://localhost:8090 TROUBA_NO_HMR=1 npm run dev -- --port 5273 --strictPort",
      url: "http://localhost:5273",
      reuseExistingServer: false,
      timeout: 60_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});

/**
 * Playwright e2e config. Runs the SPA against a LIVE stack:
 *   1. TroubaCore (Go) on :8080 with the in-memory app store (fresh per run →
 *      deterministic), and the annotation store also in-memory.
 *   2. Vite dev server on :5173 proxying /api → :8080 (same-origin cookies).
 *
 * baseURL is the Vite origin, so the browser hits the SPA and the session cookie
 * just works. Playwright waits for each server's URL before starting tests.
 */
import { defineConfig, devices } from "@playwright/test";

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  retries: 0,
  reporter: [["list"]],
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: "http://localhost:5173",
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // Fresh in-memory backend per run → deterministic tests.
      command:
        "cd ../../core && TROUBA_APP_STORE=mem TROUBA_STORE=mem TROUBACORE_ADDR=:8080 go run ./cmd/troubacore",
      url: "http://localhost:8080/healthz",
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      command: "npm run dev",
      url: "http://localhost:5173",
      reuseExistingServer: !process.env.CI,
      timeout: 60_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});

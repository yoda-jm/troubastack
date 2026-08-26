/**
 * Playwright e2e config. Runs the SPA against a LIVE, ISOLATED stack:
 *   1. TroubaCore (Go) on :8091 with the in-memory app store (fresh per run →
 *      deterministic), and the annotation store also in-memory.
 *   2. Vite dev server on :5174 proxying /api → the core, so the SPA and API are
 *      same-origin to the browser (the HttpOnly trouba_session cookie just works).
 *
 * T81 — dedicated default ports (:8091 / :5174), NOT :8080 / :5173. A local preview
 * (`make demo`, `make band=...`) holds :8080/:5173, and with `reuseExistingServer:false`
 * a hardcoded :8080 made `make e2e` simply unable to run alongside it — the friction that
 * let a red `main` sit unnoticed for the whole T72→T79 window. The default is now
 * collision-free; override with E2E_CORE_PORT / E2E_VITE_PORT if you ever need to.
 * Both servers use reuseExistingServer:false so a lingering vite from an earlier run
 * (pointed at a different TROUBA_API_TARGET) can never make the suite test the wrong backend.
 */
import { defineConfig, devices } from "@playwright/test";

const CORE_PORT = process.env.E2E_CORE_PORT ?? "8091";
const VITE_PORT = process.env.E2E_VITE_PORT ?? "5174";
const BASE_URL = `http://localhost:${VITE_PORT}`;

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: false,
  workers: 1,
  forbidOnly: !!process.env.CI,
  // T117: retries are for an INFRASTRUCTURE blip — the Go webServer cold-compile, the teardown
  // ECONNRESET, runner load — a single-shot event one retry clears. NOT to quiet a real flake.
  // CI only (locally a flake must surface immediately). ONE, not two: at retries=2 a 10% flake goes
  // green ~999 runs in 1000 (1 - p^(n+1)) and stops telling you — the concealment we don't want.
  retries: process.env.CI ? 1 : 0,
  // On CI: `github` annotates flaky/failed inline on the PR, and the `json` report feeds the
  // flaky-warning step. A retried pass scores `flaky` — a distinct outcome from `passed`, never a
  // silent green.
  reporter: process.env.CI
    ? [["list"], ["github"], ["json", { outputFile: "playwright-report.json" }]]
    : [["list"]],
  timeout: 30_000,
  expect: { timeout: 10_000 },
  use: {
    baseURL: BASE_URL,
    trace: "retain-on-failure",
  },
  projects: [{ name: "chromium", use: { ...devices["Desktop Chrome"] } }],
  webServer: [
    {
      // Fresh in-memory backend per run → deterministic tests. NO_MDNS so it can't clash with a
      // preview's mDNS advertisement while both are up.
      command: `cd ../../core && TROUBA_APP_STORE=mem TROUBA_STORE=mem TROUBA_NO_MDNS=1 TROUBACORE_ADDR=:${CORE_PORT} go run ./cmd/troubacore`,
      url: `http://localhost:${CORE_PORT}/healthz`,
      reuseExistingServer: false,
      timeout: 120_000,
      stdout: "pipe",
      stderr: "pipe",
    },
    {
      // Its own Vite, proxying /api → this run's isolated core (not the :8080 preview).
      command: `TROUBA_API_TARGET=http://localhost:${CORE_PORT} npm run dev -- --port ${VITE_PORT} --strictPort`,
      url: BASE_URL,
      reuseExistingServer: false,
      timeout: 60_000,
      stdout: "pipe",
      stderr: "pipe",
    },
  ],
});

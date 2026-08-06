/**
 * Seeds the walkthrough backend with the shipped demo data (real songs, charts, annotations,
 * the orchestra with parts) once TroubaCore is up. Idempotent — safe to re-run against an
 * already-seeded instance (the seed keys everything by stable ids).
 */
import { execFileSync } from "node:child_process";

async function waitFor(url: string, ms = 60_000) {
  const deadline = Date.now() + ms;
  while (Date.now() < deadline) {
    try {
      const r = await fetch(url);
      if (r.ok) return;
    } catch {
      /* not up yet */
    }
    await new Promise((r) => setTimeout(r, 300));
  }
  throw new Error(`timed out waiting for ${url}`);
}

export default async function globalSetup() {
  await waitFor("http://localhost:8080/healthz");
  // eslint-disable-next-line no-console
  console.log("[walkthrough] seeding demo data…");
  execFileSync("go", ["run", "./cmd/seed", "-addr", "http://localhost:8080", "-password", "demo"], {
    cwd: "../../core",
    stdio: "inherit",
    env: { ...process.env, GOFLAGS: "-mod=mod" },
  });
  console.log("[walkthrough] seed complete.");
}

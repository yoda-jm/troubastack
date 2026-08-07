/**
 * DEMO-VID Part B — walkthrough backend prep.
 *
 * The recording BUILDS The Troubadours live, on camera, from an empty server (register the
 * members, create the band, add the song, annotate it, set the setlist, bake). So this setup
 * does NOT seed that band — it only seeds the City Chamber ORCHESTRA (via `seed -only
 * orchestra`), which the tour reveals at the end as "the same app, at orchestra scale". After
 * a run the server holds the live-built Troubadours + the seeded orchestra = the full demo.
 *
 * Isolated on its own port (:8090) + data dir so it never touches the persistent :8080 demo.
 */
import { execFileSync } from "node:child_process";

const API = "http://localhost:8090";

async function waitFor(url: string, ms = 90_000) {
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
  await waitFor(`${API}/healthz`);
  // eslint-disable-next-line no-console
  console.log("[walkthrough] seeding ONLY the orchestra (Troubadours is built live)…");
  execFileSync(
    "go",
    ["run", "./cmd/seed", "-addr", API, "-password", "demo", "-only", "orchestra"],
    { cwd: "../../core", stdio: "inherit", env: { ...process.env, GOFLAGS: "-mod=mod" } },
  );
  console.log("[walkthrough] orchestra seeded; empty stage set for the live build.");
}

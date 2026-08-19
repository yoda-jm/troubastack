# T81 — e2e must not fight the local demo for :8080

**Priority:** high for its size (S) — this is the friction that let two stale assertions live in
`text-chart.spec` through the whole T72→T79 window · **Area:** `web/studio/playwright.config.ts`,
CI, docs. Raised independently by the lane and by me at the T79 gate (2026-08-20).

## Why

`playwright.config.ts` boots the Go core with `TROUBACORE_ADDR=:8080` and
`reuseExistingServer: false`. So if **anything** already holds :8080 — most obviously the local
demo (`make demo`) or a real band preview (`make band=<shortname>`, B14) — the suite cannot start.

That is not a theoretical annoyance. It is the documented reason the full suite got downgraded to
"ran the affected subset" at the T78/T79 gate, which is how `text-chart.spec` sat red on `main`
since T72 without anyone noticing. **A gate people cannot run is a gate that stops being run.**

Two further facts make this cheap:

- **`vite.config.ts` already supports an override**: `TROUBA_API_TARGET ?? "http://localhost:8080"`.
  The proxy half is done; only the Playwright half hardcodes.
- **The pattern already exists in this repo.** `playwright.walkthrough.config.ts` +
  `walkthrough/global-setup.ts` run an isolated backend on **:8090** with its own data dir,
  explicitly *"so it never touches the persistent :8080 demo"*. Copy that; do not invent a mechanism.

## Design (decided)

1. **Dedicated default ports, not just overridable ones.** Make the e2e stack default to its own
   ports (core **:8091**, adjacent to the walkthrough's :8090; vite **:5174**), overridable via
   `TROUBA_E2E_CORE_PORT` / `TROUBA_E2E_WEB_PORT`. Configurable-but-still-8080 would leave the
   collision in place for anyone who doesn't know the flag — and not knowing is the whole failure
   mode here. The default must be collision-free.
2. **The vite the suite uses must point at the core the suite booted.** Start it with
   `TROUBA_API_TARGET=http://localhost:<core port>`; derive `baseURL` and the `url` health checks
   from the same variables so there is one source of truth for each port.
3. **`reuseExistingServer: false` for both.** With dedicated ports, reuse buys little, and a lingering
   vite from an earlier run pointed at a *different* `TROUBA_API_TARGET` would silently route tests
   at the wrong backend — precisely the class of failure this task exists to remove. Pay the few
   seconds of cold start.
4. **CI keeps working**, on the new defaults or with explicit env — whichever is clearer in
   `ci.yml`. No `continue-on-error`, no weakening of the hard gate.

## Acceptance criteria

- **The reproduction is the test:** with a server occupying **:8080** (e.g. `make demo` running),
  `make e2e` starts and runs the full suite. This is the criterion — demonstrate it in the handoff
  with the occupying server actually up, not merely with the ports changed.
- **Isolation is proven, not assumed.** A test (or a documented check in the handoff) shows the suite
  talks to its own fresh in-memory backend and not a demo/preview one — e.g. the seeded demo band is
  **absent** from the e2e backend. Without this, a mis-pointed proxy would look green while testing
  the wrong server.
- Ports come from single variables: no literal `8080`/`5173`/`8091`/`5174` left scattered across
  `playwright.config.ts`; `TROUBA_E2E_CORE_PORT` / `TROUBA_E2E_WEB_PORT` override cleanly.
- The **walkthrough** config keeps working unchanged on :8090 (it already solved this; don't
  regress it).
- CI's e2e job green on the new configuration; the job still hard-gates.
- Docs updated where a human would look: the e2e section of `docs/tasks/README.md` ground rules
  (rule 4 names `make e2e`) and any README that tells someone to run the suite — state that e2e is
  port-isolated and how to override.
- `tsc -b studio` clean.

## Out of scope

- **Why CI's e2e job never surfaced a red `main`** across T72→T79. That is a companion question and
  a genuine one — a hard gate nobody reads is worse than no gate — but it is answered by looking at
  the Actions run history and the branch/PR flow, not by editing a config. Track it separately;
  this task must not be closed by claiming it.
- Speeding up the suite, sharding it, or changing what it covers.
- The `:8080` mention in `editor-insecure-context.spec.ts` (a comment about a real deployment box,
  not a dependency).

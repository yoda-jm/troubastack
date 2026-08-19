# T81 — Make the e2e core port configurable, and prove the e2e gate actually gates `main`

**Priority:** high · **Size:** S (Part A) + investigation (Part B) · **Area:**
`web/studio/playwright.config.ts` + `Makefile` (Part A); `.github/workflows/ci.yml` + repo branch
protection (Part B); task-writing conventions (Part C). Filed at the T78/T79 gate 2026-08-20 after a
concrete miss.

## Why (the miss that motivates this)

The T78/T79 review found that **`main`'s e2e has been red since T72**: `text-chart.spec.ts` asserts a
`.pdf` filename that T72 deliberately removed, so it fails deterministically. I confirmed it against
*unmodified* `origin/main`. Two independent root causes let a red `main` persist unnoticed:

1. **A local port collision silently shrinks the suite that gets run.** `playwright.config.ts`
   hardcodes `TROUBACORE_ADDR=:8080` with `reuseExistingServer: false`, so anyone running a local
   preview on `:8080` — the GVO band server, exactly our normal setup — **cannot run `make e2e` at
   all**. That friction is what turns "run the suite" into "run the affected subset", which is how two
   stale assertions survived T72 → T78/T79.
2. **The gate may not be enforced.** `ci.yml`'s `e2e` job hard-gates *by configuration* — it runs on
   `push: [main]` and `pull_request`, and the file explicitly forbids `continue-on-error` — yet a red
   `main` persisted for weeks. So either the `e2e` check is **not a required status check** in the
   `main` branch-protection rule, or failing `main` pushes are simply not read. A gate nobody enforces
   is worse than no gate.

## Part A — configurable e2e core port (S; `web/studio` + `Makefile`)

- `playwright.config.ts` reads the core port from an env var (propose `E2E_CORE_PORT`, default
  `8080`). The `webServer` core `command` uses `TROUBACORE_ADDR=:$E2E_CORE_PORT`, its health `url`
  uses the same port, and Vite's `/api` proxy target follows (so cookies stay same-origin).
- Keep `reuseExistingServer: false` (CI wants a fresh, deterministic server per run) — the
  configurable port removes the *local* collision without weakening CI.
- Document the override on the `e2e` make target: `E2E_CORE_PORT=8091 make e2e` runs while a preview
  holds `:8080`.
- **Acceptance:** with a server occupying `:8080`, `E2E_CORE_PORT=8091 make e2e` boots and runs green;
  default (`make e2e`, `:8080` free) is unchanged; CI still uses a fresh server.

### Amendments (Fable, spec review 2026-08-20)

Filed in parallel with this one — my duplicate `T81-e2e-port-isolation.md` is deleted and its
substance folded in here. Your Part B framing is sharper than mine was: naming **required status
checks** on the branch-protection rule as the likely reason a red `main` persisted is the right
hypothesis, and correctly VLL-owned.

Three changes to Part A:

- **Default to a dedicated port, don't just make it overridable.** Propose core **:8091** (adjacent
  to the walkthrough's :8090) as the *default*, still overridable via `E2E_CORE_PORT`. A
  configurable default of `:8080` leaves the collision in place for anyone who doesn't know the flag
  — and *not knowing* is the entire failure mode this task exists to remove. CI can pin the port
  explicitly if a stable value is wanted there; that is a one-line change and keeps the local default
  collision-free.
- **Prove isolation, don't assume it.** Add an acceptance criterion that the suite demonstrably talks
  to its *own* fresh in-memory backend — e.g. the seeded demo band is **absent** from it. A
  mis-pointed proxy (vite still aimed at a demo backend) would otherwise look perfectly green while
  testing the wrong server, which is the same silent-wrong-target class of bug this task is meant to
  eliminate. It is also the reason `reuseExistingServer: false` on both servers is right, not just
  the core: a vite lingering from an earlier run with a different `TROUBA_API_TARGET` is exactly that
  failure.
- **Two things make this cheaper than it looks.** `vite.config.ts` already honours
  `TROUBA_API_TARGET ?? "http://localhost:8080"`, so only the Playwright half hardcodes; and the
  repo has already solved this once — `playwright.walkthrough.config.ts` + `walkthrough/global-setup.ts`
  run an isolated backend on :8090 explicitly *"so it never touches the persistent :8080 demo"*.
  Copy that pattern rather than inventing one, and keep the walkthrough working unchanged.

**Part A can land and be done on its own; Part B stays open as a VLL action item and must not be
closed by claiming it** — it is answered by reading the Actions history and the branch-protection
settings, not by editing a config. Part C I endorse as written; it is the convention I adopted at the
T79 gate.

## Part B — prove the gate gates (needs VLL / repo admin)

- Check the Actions run history: **has the `e2e` job been failing on `main` pushes since T72?** (I
  could not verify here — `gh` is not installed in this environment.)
- Ensure the `e2e` job is a **required status check** on the `main` branch-protection rule, so a red
  e2e blocks the merge instead of landing silently.
- For a *legitimately* CI-only failure, keep the existing pattern — quarantine at the spec level with
  `test.skip(!!process.env.CI, …)` + a tracking task (precedent: `editor-rorw-shift.spec.ts`, T13) —
  rather than reaching for `continue-on-error`.
- **Acceptance:** a deliberately-red e2e spec on a PR blocks that PR (required check fails); the `e2e`
  check appears as required on `main`.

## Part C — the deeper cause: acceptance criteria must name e2e for UI-visible changes

T72 changed a **user-visible filename** that an e2e asserts on, but its acceptance criteria named
only `gofmt`/`vet`/`make test` — not the e2e suite — so the break was never gated at review time.
Fable adopted the fix-forward at the T79 gate; record it as a standing convention:

- **When a change is observable in the UI (a filename, a label, a layout, a testid), the task's
  acceptance criteria name the e2e suite explicitly.** Add this line to the task-writing conventions
  doc so it is not re-forgotten.

## Out of scope

- Rewriting the webServer bootstrap, e2e sharding/parallelism, or migrating off Playwright.
- The T80 add-file dialog shell.
- Fixing any *other* pre-existing CI-only failures beyond confirming the gate is enforced.

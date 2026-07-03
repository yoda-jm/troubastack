# T02 — GitHub Actions CI

**Priority:** 2 (do after T01) · **Size:** S · **Area:** `.github/workflows`, `Makefile`

## Context

The repository has **no CI at all**. `docs/ARCHITECTURE.md` claims several invariants are
enforced by "test … in CI", but nothing gates a push: `make test` runs Go tests only, the
web workspace isn't typechecked anywhere, and a broken import sat in `web/bake` unnoticed
(fixed in T01). A remote GitHub repo now exists, so GitHub Actions is the natural home.

## Changes

Create `.github/workflows/ci.yml` with jobs that run on `push` to `main` and on
`pull_request`:

1. **go** — `actions/setup-go` (read the Go version from `core/go.mod`), then
   `cd core && go vet ./... && go test ./...` and a gofmt check that **fails** on diff:
   `test -z "$(gofmt -l .)"` (the current `make check` only prints).
2. **web** — `actions/setup-node` (Node 24). IMPORTANT: the `web/` npm workspace is
   nominal — there is no root `package-lock.json`, and studio is designed to install
   standalone (`make setup` runs `npm install --no-workspaces` in `web/studio`; ink is
   resolved from source via a Vite alias + tsconfig `paths`, NOT from the registry).
   So install per-package where a lockfile exists, and run tsc from studio's install:
   ```
   cd web/studio && npm ci --no-workspaces
   cd web/ink && npm ci --no-workspaces
   cd web && TSC=studio/node_modules/.bin/tsc; \
     $TSC --noEmit -p ink && $TSC -b studio && $TSC --noEmit -p bake
   cd web/studio && npm run build
   ```
   (studio's tsconfig.json is a solution file — `-p studio` compiles nothing;
   `-b studio` follows the app + node project references so the gate is real.)
   Do NOT `npm ci` at the `web/` root (no lockfile) and do NOT add registry deps between
   the sibling packages — see the `//deps` comment in `web/studio/package.json`.
3. **proto** — install `buf` (official `bufbuild/buf-action` or download a pinned
   release) and run `buf lint` from `proto/`. Do **not** run `buf generate` yet —
   codegen adoption is a separate decision (see T09's note).
4. **e2e** (separate job, allowed to be slower) — `make setup`-equivalent steps
   (`cd web && npm ci && npx playwright install --with-deps chromium`), build the
   backend, and run `make e2e`. If the Makefile target assumes a running server, check
   how `web/studio/playwright.config.*` launches it (it may have a `webServer` block) and
   adapt; if the suite proves flaky in CI, mark the job `continue-on-error: true` and
   note it in the PR description rather than silently dropping it.

Also update the root `Makefile`: make `check` fail (not just print) on unformatted files,
so local and CI behavior match.

## Acceptance criteria

- The workflow runs on GitHub and all jobs are green on the task branch.
- Introducing a deliberate type error in `web/studio` (locally, not committed) makes the
  web job's command fail — i.e. the commands actually gate.
- `make check` exits non-zero when a Go file is unformatted.

## Out of scope

- Kotlin/`app` builds (the Gradle files are intentionally commented out).
- `buf generate` / generated-code drift checks (T09 decides that policy first).
- Coverage reporting, release workflows.

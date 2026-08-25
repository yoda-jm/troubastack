# T111 — CI builds the artifact users actually run

**Priority:** normal · **Size:** S · **Area:** `.github/`, `deploy/` (Web & Core lane).
From the 2026-08-25 project audit, finding C8.

## 1. The problem

This repo gates five ways — Go tests, generated-mirror drift, glyph geometry, cross-language vectors,
pixel parity, e2e — and **never builds the Docker image**. The three-stage `Dockerfile` and the compose
stack are the least verified artifacts in an otherwise heavily verified repo, and they are the ones a
self-hoster actually runs. A broken `Dockerfile` is discovered by a user, at deploy time.

## 2. What to build

**(a) A build-only image job** in CI. Build the production image; do not push it anywhere.

**(b) Compose validation** — `docker compose config` on the deploy stack, so a malformed compose or a
dangling variable fails in CI rather than on the host.

**(c) The CI hygiene this touches while you're in the file:** `timeout-minutes` on the new job (and the
four existing jobs that lack it), and `concurrency: cancel-in-progress` at workflow level.

## 3. Rules

- **Build-only.** No registry, no credentials, no push — publishing is a separate decision (audit F5).
- Use layer caching so this doesn't add minutes to every push, but **do not** let a cache hit mask a
  broken build: the job must fail on a `Dockerfile` that cannot build from scratch.
- Teeth-check it: break the `Dockerfile` locally, confirm the job reddens, restore. Report that.

## 4. Acceptance criteria

- A CI job builds the production image and fails on a broken `Dockerfile` (teeth-checked and reported).
- `docker compose config` validates the deploy stack in CI.
- Every CI job has `timeout-minutes`; the workflow has `concurrency: cancel-in-progress`.
- Added wall-clock cost to a normal push reported.

## 5. Out of scope

Publishing images, tags, releases, signed APKs (audit F5). Multi-arch. Changing the `Dockerfile` itself
beyond what's needed to make it build.

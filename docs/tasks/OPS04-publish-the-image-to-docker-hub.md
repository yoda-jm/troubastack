# OPS04 — publish the container image to Docker Hub automatically

**Lane:** web-core (CI/ops).
**Status:** spec, not started.
**Asked by:** VLL — "j'aimerais publier mon image docker automatiquement dans Dockerhub".

## What already exists — this is a small delta, not a build

Surveyed rather than assumed:

- `.github/workflows/ci.yml` already has an **`image` job**: `docker/setup-buildx-action` +
  `docker/build-push-action@v6` with a GHA layer cache, building `troubacore:ci` with
  **`push: false`**, then validating `deploy/docker-compose.yml`.
- The `Dockerfile` already takes **`ARG VERSION=docker`** and **`ARG BUILT_AT=unknown`** and
  threads them into the binary via `-ldflags -X …buildinfo.version/builtAt`, exposed as
  `buildinfo.Version()` / `BuiltAt()`.

So the work is: log in, push on `main` only, and feed the args that are already wired.

## Three findings that shape the task

**1. The published image would currently claim its version is `docker`.** The CI job passes
**no** `build-args`, so the `ARG VERSION=docker` default wins. An image on a public registry
whose own version string is the word "docker" cannot be supported — someone reports a bug and
nobody can tell which build they ran. **Passing `VERSION`/`BUILT_AT` is part of this task, not a
nicety.**

**2. There are no git tags in this repository — none at all.** So there is no semver to publish
against, and `git describe` alone yields a bare sha. A tagging scheme is a decision, not a
detail; see below.

**3. `deploy/docker-compose.yml` builds from source** (`build: context: ..`). That is *why*
publishing is worth doing: today a self-hoster runs a multi-stage Node+Go build on their own box
before the thing starts. After this, `docker compose up -d` can pull. Switching compose to
`image:` with a `build:` override kept for developers is the actual user-visible win, and it
belongs in this task.

## Decisions — settled by VLL, 2026-09-02

**a) Repository name: `<user>/troubastack`.** ✅ It matches what the page and the README tell
people they are installing, which is what someone types into Docker Hub's search box.

*Consequence, and it is a real work item:* BRAND04 specs the image's OCI title as **TroubaCore**.
Pulling `troubastack` and having `docker inspect` answer "TroubaCore" names two products for one
artefact. Since the repo name is settled, **`org.opencontainers.image.title` becomes
`TroubaStack`**, and BRAND04's label table takes that one-line amendment. The binary stays
`troubacore`, and the OCI *description* may still say the image runs the TroubaCore server — the
title is the product being distributed.

**b) Tags: `latest` only.** ✅ Decided by VLL, against my recommendation, so the cost is recorded
rather than argued: with no immutable tag there is **nothing to pin and nothing to roll back to**,
and when someone reports a problem there is no way to establish which image they were running.
The `VERSION`/`BUILT_AT` build-args below become the *only* traceability the artefact carries, which
raises them from "part of this task" to the thing that makes the task supportable at all.

Adding `main-<short-sha>` later costs one line and breaks nothing.

**c) Architectures: `linux/amd64` only, for now.** ✅ One line, no QEMU, the job stays fast.
`linux/arm64` would need buildx + QEMU and run several times slower, because the Node SPA build
executes under emulation.

*The obligation that comes with that choice:* the README and the project page must **state the
published architecture**. The page pitches "a home server or a cheap VPS is plenty", and a large
share of home servers are arm64 — publishing amd64-only while staying quiet would make that
sentence misleading. arm64 on a native runner is a follow-up task, not a silent gap.

## Work

1. **Secrets — only VLL can do this.** Create a Docker Hub **access token** (not the account
   password), scoped to read/write on that one repository, and add
   `DOCKERHUB_USERNAME` + `DOCKERHUB_TOKEN` as GitHub Actions secrets.
2. **Push only from `main`.** The `image` job runs on pull requests too. A push step must be
   guarded — `if: github.event_name == 'push' && github.ref == 'refs/heads/main'` — so that **a
   fork's pull request never reaches the credential**. This is the security core of the task: the
   build stays on PRs, the login and push do not.
3. **Feed the version**: `build-args: VERSION=${{ github.sha }}` (or `git describe --always`) and
   `BUILT_AT` as an ISO-8601 UTC timestamp.
4. **Tag** `latest` and `main-<short-sha>`; keep the existing GHA cache.
5. **Concurrency**: a group on the publish step so two quick merges cannot race `latest` into an
   older build.
6. **Land BRAND04's OCI labels with or before this.** The first published image is what people
   judge; an unlabelled one on a public registry has no title, no source link and no licence.
7. **Compose**: switch the service to `image: <user>/troubastack:latest` with the local `build:` preserved
   behind a documented override for developers.
8. **Docker Hub description**: at minimum set it by hand once. Automating it from the README is
   optional and not worth a dependency.

## ⚠ Required sweep on landing — this task makes committed statements false

Both of these say, today, that no image exists. They are load-bearing honesty statements and I
wrote one of them:

- `web/site/index.html` — *"no published registry image, no GitHub Releases binary and no store or
  F-Droid listing"* (the "Packaging status, honestly" panel).
- `README.md:116` — the same claim.

When this lands, **both must be updated in the same commit**, along with the install instructions,
which currently tell people to build. A public page that still says "no registry image" the day
after one is published is worse than never having published.

## Done when

- A green push to `main` results in a pull-able image.
- `docker run --rm <repo>:latest` reports a **traceable** version through `buildinfo` — not
  `docker`, not `unknown`. Check this on the *pulled* image, not the locally built one.
- A pull request from a **fork** builds the image and **does not** log in — verify by reading the
  run, not by assuming the `if` is right.
- `docker compose up -d` on a clean machine pulls instead of building.
- The page and the README no longer claim there is no registry image, and they state the
  architecture that is actually published.

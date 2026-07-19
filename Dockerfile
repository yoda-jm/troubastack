# TroubaCore production image (OPS01). Multi-stage: build the SPA + bake worker with
# Node, embed the SPA into the Go binary at compile time, then a slim runtime that can
# also BAKE (poppler + node + the bake worker). One origin, one binary, one container.
#
# NOTE: authored to the spec; the live `docker build` / `docker compose up` bring-up is
# OPS01's attended acceptance step (no docker in the authoring env). Verify per
# deploy/README.md before relying on it.
#
# Build from the REPO ROOT:  docker build -t troubacore .

# ---- Stage 1: build the SPA (web/studio) + the bake worker (web/bake) -------------
FROM node:24-slim AS web
WORKDIR /src
# @troubastack/ink is resolved from SOURCE at build time via studio's Vite alias and
# bake's esbuild alias (deliberately NOT an npm dep — repo runs --no-workspaces), so
# both builds need web/ink present. Its only runtime dep, perfect-freehand, is listed in
# studio's and bake's own package.json, so `npm ci` below installs it.
COPY web/ink web/ink
# Studio SPA (no npm workspaces — each package has its own lockfile; `npm ci` per pkg).
COPY web/studio/package.json web/studio/package-lock.json web/studio/
RUN cd web/studio && npm ci
COPY web/studio web/studio
RUN cd web/studio && npm run build          # → web/studio/dist
# Bake overlay worker (spawned by core for setlist bakes).
COPY web/bake/package.json web/bake/package-lock.json web/bake/
RUN cd web/bake && npm ci
COPY web/bake web/bake
RUN cd web/bake && npm run build            # → web/bake/dist (index.js + cli.js)

# ---- Stage 2: build the Go binary with the SPA embedded ---------------------------
FROM golang:1.26 AS go
WORKDIR /src
# core imports NO codegen output: the proto types are hand-mirrored (I1/P203), so
# core/internal/gen (a buf-generate target) is unused by the build — `go build ./...`
# succeeds with it absent. No buf/codegen step is needed here.
COPY core core
COPY proto proto
# The embed is COMPILE-TIME: the built SPA must sit in core/internal/webassets/dist
# before `go build` (mirrors Makefile `embed`).
COPY --from=web /src/web/studio/dist/ core/internal/webassets/dist/
ARG VERSION=docker
ARG BUILT_AT=unknown
RUN cd core && CGO_ENABLED=0 go build \
      -ldflags "-X troubastack/core/internal/buildinfo.version=${VERSION} -X troubastack/core/internal/buildinfo.builtAt=${BUILT_AT}" \
      -o /out/troubacore ./cmd/troubacore

# ---- Stage 3: runtime (bake-capable) ----------------------------------------------
# node:24-slim (bake shells out to Node) + poppler-utils (pdftoppm for PDF rasters).
# For a MINIMAL, no-bake image, swap this base for gcr.io/distroless/static and drop
# the apt/bake-worker copies — the API + embedded SPA need only the static binary.
FROM node:24-slim AS runtime
RUN apt-get update && apt-get install -y --no-install-recommends poppler-utils \
      && rm -rf /var/lib/apt/lists/*
# The bake worker + its native @napi-rs/canvas + bundled font (linux/same-arch prebuilt
# carries from the build stage).
COPY --from=web /src/web/bake/dist /app/web/bake/dist
COPY --from=web /src/web/bake/node_modules /app/web/bake/node_modules
COPY --from=web /src/web/bake/assets /app/web/bake/assets
COPY --from=web /src/web/bake/package.json /app/web/bake/package.json
COPY --from=go /out/troubacore /usr/local/bin/troubacore
# OPS02: embed the downloadable native app binaries. `deploy/apps/` is committed with
# just a .gitkeep, so this COPY always succeeds; whoever builds a release image drops
# the (debug-signed, unknown-sources-installable) APK there first — e.g.
#   cp androidApp-debug.apk deploy/apps/troubashare.apk && docker build -t troubacore .
# An EMPTY dir ⇒ /api/apps is empty ⇒ Studio hides the "Get the app" card (see
# deploy/README.md § "Embed the app"). Non-secret; served unauthenticated.
COPY deploy/apps/ /app/apps/
# Point core at the worker's absolute path (the default is repo-relative and won't
# resolve here); keep the data dir on a mounted volume.
ENV TROUBA_BAKE_CLI=/app/web/bake/dist/cli.js \
    TROUBA_NODE=node \
    TROUBA_PDFTOPPM=pdftoppm \
    TROUBA_APPS_DIR=/app/apps \
    TROUBA_DATA_DIR=/data \
    TROUBACORE_ADDR=:8080
RUN useradd -r -u 10001 trouba && mkdir -p /data && chown trouba /data
USER trouba
VOLUME /data
EXPOSE 8080
ENTRYPOINT ["troubacore"]

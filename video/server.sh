#!/usr/bin/env bash
# The isolated filming server (docs/video/README.md §4.2, §5): Studio embedded, seeded demo band, the APK
# offered by "Get the app" — on 127.0.0.1:${PORT:-18097} with a throwaway data dir. Never :8080, never real data.
#
#   video/server.sh build            # Studio + server binary, ONE version stamp (no "versions differ" banner)
#   video/server.sh start [APK]      # fresh data dir, start, seed; prints BASE= and the data dir
#   video/server.sh stop
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE="${TROUBA_VIDEO_STATE:-$ROOT/video/out/server}"
PORT="${PORT:-18097}"
BIN="$STATE/troubacore"
mkdir -p "$STATE"

build() {
  local v; v="$(git -C "$ROOT" describe --always --dirty)"   # computed BEFORE the embed dirties the tree
  local built; built="$(date -u +%Y-%m-%dT%H:%MZ)"
  ( cd "$ROOT/web/studio" && [ -e node_modules ] && ./node_modules/.bin/vite build --logLevel warn )
  rm -rf "$ROOT/core/internal/webassets/dist" && mkdir -p "$ROOT/core/internal/webassets/dist"
  cp -r "$ROOT/web/studio/dist/"* "$ROOT/core/internal/webassets/dist/"
  ( cd "$ROOT/core" && go build -ldflags "-X troubastack/core/internal/buildinfo.version=$v -X troubastack/core/internal/buildinfo.builtAt=$built" -o "$BIN" ./cmd/troubacore )
  git -C "$ROOT" checkout -q -- core/internal/webassets/dist   # restore the tracked placeholder
  echo "built $BIN (version $v)"
}

stop() {
  [ -s "$STATE/pid" ] || return 0
  local pid; pid="$(cat "$STATE/pid")"
  if [ "$(readlink "/proc/$pid/exe" 2>/dev/null)" = "$BIN" ]; then kill "$pid"; fi   # only OUR binary
  rm -f "$STATE/pid"
}

start() {
  local apk="${1:-$ROOT/app/androidApp/build/outputs/apk/debug/androidApp-debug.apk}"
  [ -x "$BIN" ] || build
  stop
  rm -rf "$STATE/data" "$STATE/apps" && mkdir -p "$STATE/data" "$STATE/apps"
  [ -s "$apk" ] && cp "$apk" "$STATE/apps/troubastage.apk"
  TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR="$STATE/data" TROUBA_NO_MDNS=1 \
  TROUBACORE_ADDR="127.0.0.1:$PORT" TROUBA_APPS_DIR="$STATE/apps" \
  TROUBA_BAKE_CLI="${TROUBA_BAKE_CLI:-$ROOT/web/bake/dist/cli.js}" \
    setsid "$BIN" >"$STATE/server.log" 2>&1 </dev/null &
  echo $! > "$STATE/pid"
  for _ in $(seq 60); do curl -sf "http://127.0.0.1:$PORT/healthz" >/dev/null && break; sleep 0.5; done
  curl -sf "http://127.0.0.1:$PORT/healthz" >/dev/null || { echo "server did not start; see $STATE/server.log" >&2; exit 1; }
  ( cd "$ROOT/core" && go run ./cmd/seed -addr "http://127.0.0.1:$PORT" -password demo >/dev/null )
  echo "BASE=http://127.0.0.1:$PORT  (data: $STATE/data, users marie/leo/sasha, password demo)"
}

case "${1:-}" in build) build ;; start) shift; start "$@" ;; stop) stop ;; *) sed -n 2,8p "$0"; exit 2 ;; esac

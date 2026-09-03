# TroubaStack top-level dev tasks. See docs/ARCHITECTURE.md for the rules these serve.
.PHONY: help setup dev run run-api core test test-web studio embed dist e2e e2e-smoke check proto app fixtures seed demo band

# `make band=<shortname>` sets the `band` variable and (with no explicit target) runs the default goal —
# so when `band` is set, make the `band` target the default. Plain `make` still shows help.
ifneq ($(band),)
.DEFAULT_GOAL := band
endif

help:
	@echo "TroubaStack — targets:"
	@echo "  setup    install web deps + the Playwright browser (run once)"
	@echo "  dev      run the full app for development: core API + Vite hot-reload"
	@echo "           -> open http://localhost:5173 (Vite proxies /api -> :8080)"
	@echo "  run      single binary: real SPA + API on :8080 (empty, in-memory)"
	@echo "  run-api  API only from source (serves the SPA placeholder) — backend dev"
	@echo "  test     run Go tests (engine + stores + http API)"
	@echo "  test-web browser-free vitest units for studio + ink pure functions"
	@echo "  e2e      Playwright end-to-end (boots core + vite + chromium)"
	@echo "  dist     build SPA -> embed -> core/bin/troubacore"
	@echo "  check    go vet + gofmt"
	@echo "  seed     populate a RUNNING server with the demo dataset (cd core && go run ./cmd/seed)"
	@echo "  demo     single binary: real SPA + API on :8080 with SEEDED data (file-backed)"
	@echo "           -> login marie/demo or maestro/demo; reset: rm -rf $(TROUBA_HOME)/troubadata"
	@echo "  app      build the KMP mobile app: shared checks + debug APK (needs a JDK + Android SDK)"
	@echo "  fixtures regenerate the committed TroubaStage bundle fixtures (dev tool cmd/mkbundle)"
	@echo "  proto    deferred (buf codegen)"

setup:
	cd web/studio && npm install --no-workspaces && npx playwright install chromium

# Full app, development: core in the background (API on :8080), Vite in the
# foreground (SPA on :5173 with hot reload, proxying /api -> :8080). Vite serves
# the real SPA, so you do NOT need to embed anything. Ctrl-C stops both.
dev:
	@( cd core && exec env TROUBA_APP_STORE=mem TROUBA_DIE_WITH_PARENT=1 go run ./cmd/troubacore ) & \
	CORE_PID=$$!; \
	trap 'kill $$CORE_PID 2>/dev/null' EXIT INT TERM; \
	echo ">>> core (API) on :8080, pid $$CORE_PID — open http://localhost:5173" ; \
	cd web/studio && npm run dev

# Run the production single binary: builds + embeds the SPA, then serves SPA + API
# together on :8080 (one origin). This is what gets deployed.
run: dist
	cd core && exec env TROUBA_APP_STORE=mem TROUBA_DIE_WITH_PARENT=1 ./bin/troubacore

# Backend-only: fast `go run` with NO SPA build, so you get the placeholder page at
# :8080. Use this when iterating on the API and driving the SPA via `make dev`/Vite.
run-api:
	cd core && TROUBA_APP_STORE=mem go run ./cmd/troubacore

core:
	cd core && go build ./...

test:
	cd core && go test -race -timeout=30m ./...

# T110: browser-free unit tests for the pure functions in studio + ink (vitest). Fast — no vite server,
# no chromium. Complements `e2e` (which stays for what only a browser can reach).
test-web:
	cd web/studio && npm run test:unit

studio:
	cd web/studio && npm run build

# Copies the built SPA over the committed placeholder (a build artifact step —
# don't commit core/internal/webassets/dist/*). `make studio` must have its deps
# installed first (`make setup`).
embed: studio
	rm -rf core/internal/webassets/dist
	mkdir -p core/internal/webassets/dist
	cp -r web/studio/dist/* core/internal/webassets/dist/

# T29: stamp the git version + build time into the binary (buildinfo pkg; surfaced
# by GET /api/version and the Studio version chip). Unstamped builds report "dev".
VERSION_LDFLAGS = -X troubastack/core/internal/buildinfo.version=$(shell git describe --always --dirty) \
                  -X troubastack/core/internal/buildinfo.builtAt=$(shell date -u +%Y-%m-%dT%H:%MZ)

dist: embed
	cd core && go build -ldflags "$(VERSION_LDFLAGS)" -o bin/troubacore ./cmd/troubacore
	@echo "built core/bin/troubacore — serves /api + the embedded SPA on one origin"

# T81: e2e boots its OWN isolated stack on :8091 (core) + :5174 (vite), so it runs even while a
# local preview (`make demo` / `make band=...`) holds :8080/:5173. Override with
# E2E_CORE_PORT=<n> / E2E_VITE_PORT=<n> if those defaults ever clash.
e2e:
	cd web/studio && npx playwright test

# T117: the fast SMOKE subset (~11 critical-path/cross-cutting tests, tagged @smoke) — what runs on
# every branch push to catch CI-environment divergence early. The full `e2e` stays the landing gate.
e2e-smoke:
	cd web/studio && npx playwright test --grep @smoke

check:
	cd core && go vet ./...
	@unformatted=$$(cd core && gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
	  echo "gofmt needs these files (run: cd core && gofmt -w .):"; \
	  echo "$$unformatted"; \
	  exit 1; \
	fi

# Populate an ALREADY-RUNNING server (e.g. `make run-api` in another shell) with
# the demo dataset over HTTP, then print the browse guide.
seed:
	cd core && go run ./cmd/seed

# T129: runtime data (seeded servers) lives OUTSIDE the source tree — a `git clean -xdf` in this
# worktree would otherwise erase it (bands/ has no backup). Same formula as troubaHome() in
# core/cmd/seed. Override with TROUBA_HOME, or point TROUBA_DATA_DIR at an absolute path yourself.
TROUBA_HOME ?= $(HOME)/troubastack-data

# One-shot demo: builds + EMBEDS the SPA (via dist), then runs the single binary
# with the FILE backends (data persists under $(TROUBA_HOME)/troubadata/) in the BACKGROUND,
# seeds it, and hands the server to the FOREGROUND so you can browse the REAL SPA +
# seeded data on http://localhost:8080. Ctrl-C stops it. Reset: rm -rf $(TROUBA_HOME)/troubadata
demo: dist
	@cd core; \
	echo ">>> seeding demo data …"; \
	TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=$(TROUBA_HOME)/troubadata ./bin/troubacore & \
	SEED_CORE=$$!; \
	trap 'kill $$SEED_CORE 2>/dev/null' EXIT INT TERM; \
	for i in $$(seq 1 50); do \
		curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && break; \
		sleep 0.2; \
	done; \
	go run ./cmd/seed -addr http://localhost:8080 -password demo || true; \
	kill $$SEED_CORE 2>/dev/null; wait $$SEED_CORE 2>/dev/null; \
	trap - EXIT INT TERM; \
	echo ">>> READY: open http://localhost:8080 (real SPA + seeded data). Ctrl-C to stop; reset: rm -rf $(TROUBA_HOME)/troubadata"; \
	exec env TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=$(TROUBA_HOME)/troubadata TROUBA_DIE_WITH_PARENT=1 ./bin/troubacore

# Seed + run one or more real, local bands by their band.json "shortname":
#   make band=<shortname>              one band
#   make band=<shortname>,<shortname>  several onto one server (comma-separated list)
# (each band lives in a gitignored bands/<folder>/band.json; NOT demo content). Same one-shot
# flow as `make demo` but seeds only those band(s) into their OWN data dir, so recreating your
# server rebuilds it cleanly. Reset: rm -rf $(TROUBA_HOME)/troubadata-<shortname[-shortname...]>
#
# The comma list is a make-level convenience: it expands to repeated `-band` flags (the seed CLI
# takes one shortname per -band, so `-band a,b` is NOT a thing — it splits here, not there).
comma := ,
empty :=
space := $(empty) $(empty)
band_flags := $(foreach b,$(subst $(comma),$(space),$(band)),-band $(b))
band_dir := troubadata-$(subst $(comma),-,$(band))
band: dist
	@test -n "$(band)" || { echo "usage: make band=<shortname>[,<shortname>...]  (see bands/*/band.json 'shortname')"; exit 2; }
	@cd core; \
	echo ">>> seeding band(s) '$(band)' …"; \
	TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=$(TROUBA_HOME)/$(band_dir) ./bin/troubacore & \
	SEED_CORE=$$!; \
	trap 'kill $$SEED_CORE 2>/dev/null' EXIT INT TERM; \
	for i in $$(seq 1 50); do \
		curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && break; \
		sleep 0.2; \
	done; \
	go run ./cmd/seed -addr http://localhost:8080 -password demo $(band_flags) || true; \
	kill $$SEED_CORE 2>/dev/null; wait $$SEED_CORE 2>/dev/null; \
	trap - EXIT INT TERM; \
	echo ">>> READY: open http://localhost:8080 (band(s) '$(band)'). Ctrl-C to stop; reset: rm -rf $(TROUBA_HOME)/$(band_dir)"; \
	exec env TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=$(TROUBA_HOME)/$(band_dir) TROUBA_DIE_WITH_PARENT=1 ./bin/troubacore

# Deferred until the contract is codegen'd.
proto:
	cd proto && buf lint && buf generate

# T09: regenerate the proto-derived Go mirrors (cmd/gen-mirrors, pure-Go protocompile —
# no buf/protoc binary). Deterministic; CI drift-guards it. Kotlin/TS outputs join in
# later T09 stages.
gen:
	cd core && go run ./cmd/gen-mirrors

# The KMP/CMP mobile app (A01): compile + check the shared "mobile library" and assemble the
# Android debug APK. Uses the committed Gradle wrapper, so only a JDK (+ Android SDK) is assumed.
app:
	cd app && ./gradlew :shared:check :androidApp:assembleDebug

# Regenerate the committed TroubaStage test fixtures with the dev bundle generator (A03). Output is
# deterministic, so this should produce no diff unless the format or generator changed. This covers
# demo/ and torture/ ONLY — fixtures/baked/ is a frozen real web/bake snapshot, deliberately not an
# mkbundle output (see fixtures/README.md); do not regenerate it here.
fixtures:
	cd core && go run ./cmd/mkbundle \
	  -out ../app/shared/src/commonTest/resources/fixtures/demo \
	  -torture ../app/shared/src/commonTest/resources/fixtures/torture

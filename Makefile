# TroubaStack top-level dev tasks. See docs/ARCHITECTURE.md for the rules these serve.
.PHONY: help setup dev run run-api core test studio embed dist e2e check proto app fixtures seed demo

help:
	@echo "TroubaStack — targets:"
	@echo "  setup    install web deps + the Playwright browser (run once)"
	@echo "  dev      run the full app for development: core API + Vite hot-reload"
	@echo "           -> open http://localhost:5173 (Vite proxies /api -> :8080)"
	@echo "  run      single binary: real SPA + API on :8080 (empty, in-memory)"
	@echo "  run-api  API only from source (serves the SPA placeholder) — backend dev"
	@echo "  test     run Go tests (engine + stores + http API)"
	@echo "  e2e      Playwright end-to-end (boots core + vite + chromium)"
	@echo "  dist     build SPA -> embed -> core/bin/troubacore"
	@echo "  check    go vet + gofmt"
	@echo "  seed     populate a RUNNING server with the demo dataset (cd core && go run ./cmd/seed)"
	@echo "  demo     single binary: real SPA + API on :8080 with SEEDED data (file-backed)"
	@echo "           -> login marie/demo or maestro/demo; reset: rm -rf core/troubadata"
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
	cd core && go test ./...

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

e2e:
	cd web/studio && npx playwright test

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

# One-shot demo: builds + EMBEDS the SPA (via dist), then runs the single binary
# with the FILE backends (data persists under core/troubadata/) in the BACKGROUND,
# seeds it, and hands the server to the FOREGROUND so you can browse the REAL SPA +
# seeded data on http://localhost:8080. Ctrl-C stops it. Reset: rm -rf core/troubadata
demo: dist
	@cd core; \
	echo ">>> seeding demo data …"; \
	TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=./troubadata ./bin/troubacore & \
	SEED_CORE=$$!; \
	trap 'kill $$SEED_CORE 2>/dev/null' EXIT INT TERM; \
	for i in $$(seq 1 50); do \
		curl -sf http://localhost:8080/healthz >/dev/null 2>&1 && break; \
		sleep 0.2; \
	done; \
	go run ./cmd/seed -addr http://localhost:8080 -password demo || true; \
	kill $$SEED_CORE 2>/dev/null; wait $$SEED_CORE 2>/dev/null; \
	trap - EXIT INT TERM; \
	echo ">>> READY: open http://localhost:8080 (real SPA + seeded data). Ctrl-C to stop; reset: rm -rf core/troubadata"; \
	exec env TROUBA_APP_STORE=file TROUBA_STORE=file TROUBA_DATA_DIR=./troubadata TROUBA_DIE_WITH_PARENT=1 ./bin/troubacore

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
# deterministic, so this should produce no diff unless the format or generator changed.
fixtures:
	cd core && go run ./cmd/mkbundle \
	  -out ../app/shared/src/commonTest/resources/fixtures/demo \
	  -torture ../app/shared/src/commonTest/resources/fixtures/torture

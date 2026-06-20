# TroubaStack top-level dev tasks. See docs/ARCHITECTURE.md for the rules these serve.
.PHONY: help setup dev run run-api core test studio embed dist e2e check proto app

help:
	@echo "TroubaStack — targets:"
	@echo "  setup    install web deps + the Playwright browser (run once)"
	@echo "  dev      run the full app for development: core API + Vite hot-reload"
	@echo "           -> open http://localhost:5173 (Vite proxies /api -> :8080)"
	@echo "  run      build everything and run the SINGLE binary (SPA + API on :8080)"
	@echo "  run-api  run core from source, API only (serves the SPA placeholder) — backend dev"
	@echo "  test     run Go tests (engine + stores + http API)"
	@echo "  e2e      Playwright end-to-end (boots core + vite + chromium)"
	@echo "  dist     build SPA -> embed -> core/bin/troubacore"
	@echo "  check    go vet + gofmt"
	@echo "  proto / app : deferred (buf codegen / KMP mobile)"

setup:
	cd web/studio && npm install --no-workspaces && npx playwright install chromium

# Full app, development: core in the background (API on :8080), Vite in the
# foreground (SPA on :5173 with hot reload, proxying /api -> :8080). Vite serves
# the real SPA, so you do NOT need to embed anything. Ctrl-C stops both.
dev:
	@cd core && TROUBA_APP_STORE=mem go run ./cmd/troubacore & \
	CORE_PID=$$!; \
	trap 'kill $$CORE_PID 2>/dev/null' EXIT INT TERM; \
	echo ">>> core (API) on :8080, pid $$CORE_PID — open http://localhost:5173" ; \
	cd web/studio && npm run dev

# Run the production single binary: builds + embeds the SPA, then serves SPA + API
# together on :8080 (one origin). This is what gets deployed.
run: dist
	cd core && TROUBA_APP_STORE=mem ./bin/troubacore

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

dist: embed
	cd core && go build -o bin/troubacore ./cmd/troubacore
	@echo "built core/bin/troubacore — serves /api + the embedded SPA on one origin"

e2e:
	cd web/studio && npx playwright test

check:
	cd core && go vet ./... && gofmt -l .

# Deferred until the contract is codegen'd / mobile resumes.
proto:
	cd proto && buf lint && buf generate
app:
	cd app && ./gradlew build

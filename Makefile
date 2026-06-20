# TroubaStack top-level dev tasks. See docs/ARCHITECTURE.md for the rules these serve.
.PHONY: help core test studio embed dist run e2e check proto app

help:
	@echo "TroubaStack — targets:"
	@echo "  core    build TroubaCore (Go server)"
	@echo "  test    run Go tests (engine + stores + http API)"
	@echo "  studio  build the TroubaStudio SPA -> web/studio/dist"
	@echo "  embed   copy the built SPA into core's go:embed dir"
	@echo "  dist    studio + embed + build the single server binary (serves /api + SPA)"
	@echo "  run     run core with the in-memory backend (dev)"
	@echo "  e2e     Playwright end-to-end (boots core + vite + chromium)"
	@echo "  check   go vet + gofmt"
	@echo "  proto / app : deferred (buf codegen / KMP mobile)"

core:
	cd core && go build ./...

test:
	cd core && go test ./...

studio:
	cd web/studio && npm install --no-workspaces && npm run build

# NOTE: copies built assets over the committed placeholder index.html (a build
# artifact step — do not commit core/internal/webassets/dist/*).
embed: studio
	rm -rf core/internal/webassets/dist
	mkdir -p core/internal/webassets/dist
	cp -r web/studio/dist/* core/internal/webassets/dist/

dist: embed
	cd core && go build -o bin/troubacore ./cmd/troubacore
	@echo "built core/bin/troubacore — serves /api + the embedded SPA on one origin"

run:
	cd core && TROUBA_APP_STORE=mem go run ./cmd/troubacore

e2e:
	cd web/studio && npx playwright test

check:
	cd core && go vet ./... && gofmt -l .

# Deferred until the contract is codegen'd / mobile resumes.
proto:
	cd proto && buf lint && buf generate
app:
	cd app && ./gradlew build

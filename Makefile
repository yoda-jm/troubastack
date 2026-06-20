# TroubaStack top-level dev tasks. Each layer also has its own tooling.
# See docs/ARCHITECTURE.md for the rules these commands serve.

.PHONY: help proto core web app fmt check

help:
	@echo "TroubaStack — targets:"
	@echo "  proto   generate contract types for all clients (I1)"
	@echo "  core    build TroubaCore (Go server)"
	@echo "  web     build the web workspace (ink + studio + bake)"
	@echo "  app     build the mobile app (KMP)"
	@echo "  check   lint + tests across layers"

proto:
	cd proto && buf lint && buf generate

core: proto
	cd core && go build ./...

web: proto
	cd web && npm install && npm run -ws build

app: proto
	cd app && ./gradlew build

check:
	cd proto && buf lint
	cd core && go vet ./...
	cd web && npm run -ws lint
	@echo "app checks: see app/README.md"

# T123 — Let a client ask "are you a TroubaStack server?" before it trusts you

**Lane:** Web/Core · **Kind:** small new endpoint · **Verified against `b6d23b7`**
**Files:** `core/internal/httpapi/webapi.go` + a test. **Required by A53, recommended before A52.**

## Why

Scanning a QR points the app at a host chosen by whoever printed it, and then asks the person to type
their password into that host. The app would like to check the host is one of ours *before* the password
field appears — and it currently cannot, because **there is no unauthenticated endpoint that identifies
the server.** The complete list of routes outside `a.auth(...)` is register, login, logout, and the two
password-reset routes (`webapi.go:38-45`). Everything else, including `/api/me`, is behind a session.

A 401 from `/api/me` is not an answer: any host can return 401.

The same gap shows up outside this feature — the app's mDNS discovery
(`ConnectScreen.kt:99-117`) lists candidate servers it cannot confirm, and a person debugging a
connection has nothing to curl.

## Deliverable

1. **`GET /api/server`, unauthenticated**, returning a small identifying document — enough for a client to
   say "yes, TroubaStack, and I speak this API version". Register it outside `a.auth(...)`.

2. **Say as little as possible.** A product marker and an API version number. **No build hash, no Go
   version, no host details, no user or band counts** — this is reachable by anything that can open a
   socket to the box, and the endpoint's job is identification, not disclosure. A human-readable instance
   name is acceptable if one already exists in config; do not invent new configuration for it.

3. **Tests** in the existing `httpapi` test style: the route answers **without** a session (the load-bearing
   property — a test that only exercises it while logged in would pass even if someone later wrapped it in
   `a.auth`), returns the marker, and is unaffected by whether any user exists.

## Teeth-check

Wrap the route in `a.auth(...)` and confirm the unauthenticated test reddens. Report the count. That
mutation is the whole point of the task: this endpoint is only useful *because* it needs no session.

## Before landing

`gofmt -l core` — the CI `go` job gates on it after vet and test, so a green local test run is not enough.

## Not in scope

Rate limiting (**audit C6**, still deferred — this adds one more unauthenticated surface and does not
close it; record that negative in the sweep) · TLS · service discovery changes · any client work.

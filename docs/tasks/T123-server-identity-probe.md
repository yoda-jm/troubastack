# T123 — Let a client ask "are you a TroubaStack server?" before it trusts you

**Lane:** Web/Core · **Kind:** small new endpoint · **Verified against `b6d23b7`**
**Files:** `core/internal/httpapi/webapi.go` + a test. **Required by A53, recommended before A52.**

## Why

Scanning a QR points the app at a host chosen by whoever printed it, and then asks the person to type
their password into that host. The app wants to check the host is one of ours *before* the password field
appears.

> **📋 Correction to this task's premise, 2026-08-28 (Fable).** The first version of this spec asserted
> that *"there is no unauthenticated endpoint that identifies the server"* and gave *"the complete list of
> routes outside `a.auth(...)`"* as register, login, logout and the two password-reset routes. **That was
> wrong.** I enumerated from `webapi.go` alone; routes are mounted from several files. The actual
> unauthenticated set also includes **`/healthz`** and **`GET /api/version`** (`doc.go:44`, `:53`, proven
> session-free by `version_test.go:18`) and **`GET /api/apps`** + **`GET /apps/{file}`**
> (`appsapi.go:44-45`). Asserting a complete enumeration from one file is the exact mistake this project's
> review standard warns about, and I made it in a spec.

The gap is therefore **narrower and differently shaped** than stated. `/api/version` already answers
unauthenticated — but with `{version, builtAt, spaEmbedded}`, which is *build diagnostics*: it carries **no
product marker**, so matching it is a shape-heuristic rather than an identity claim, and its build version
moves with every rebuild while an API *contract* version must not. Its own comment (`doc.go:49-52`) already
names it **"the future app↔server compatibility hook"** — so the intended home for this exists.

The same need shows up outside this feature — the app's mDNS discovery (`ConnectScreen.kt:99-117`) lists
candidate servers it cannot confirm.

## Deliverable

1. **Extend the existing `GET /api/version`** with a product marker and an API *contract* version,
   alongside the build fields it already returns. **Do not add a new route.** A second near-identical
   unauthenticated endpoint would duplicate a job the codebase already assigned to this one, and every new
   unauthenticated surface is one more unrate-limited endpoint while **audit C6** stays open.

2. **Say as little as possible.** A product marker and an API version number. **No build hash, no Go
   version, no host details, no user or band counts** — this is reachable by anything that can open a
   socket to the box, and the endpoint's job is identification, not disclosure. A human-readable instance
   name is acceptable if one already exists in config; do not invent new configuration for it.

3. **Tests** in the existing `httpapi` test style: the route answers **without** a session (the load-bearing
   property — a test that only exercises it while logged in would pass even if someone later wrapped it in
   `a.auth`), returns the marker, and is unaffected by whether any user exists.

## Teeth-check

Wrap `/api/version` in `a.auth(...)` and confirm the unauthenticated identity test reddens. Report the
count. That mutation is the whole point of the task: this answer is only useful *because* it needs no
session. Note that `version_test.go` already covers the route session-free, so state clearly which
assertions are **new** — a mutation that only reddens the pre-existing test would prove nothing about
the identity fields.

## Before landing

`gofmt -l core` — the CI `go` job gates on it after vet and test, so a green local test run is not enough.

## Not in scope

Rate limiting (**audit C6**, still deferred — extending an existing route adds no new unauthenticated
surface, but it does not close C6 either; record that negative in the sweep) · TLS · service discovery
changes · any client work.

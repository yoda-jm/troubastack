# T109 — Test the WebSocket hub

**Priority:** normal, after T106 · **Size:** S · **Area:** `core/internal/sync` (Web & Core lane).
From the 2026-08-25 project audit, §4.3. `internal/sync` has **zero in-package tests** — this is the
realtime path every collaborator's ink travels through, and the only package of its size with none.

## 1. What to build

**(a) A table-driven test for `authorizeWrite`.** It is roughly a hundred lines of policy — role
(`admin`/`conductor`/`member`) against zone, layer access, and object kind. A policy matrix is the
canonical thing to drive from a table, and the table doubles as the readable statement of the rules.

**(b) The teardown-ordering test that T106 identified**, if T106 has not already added it: a test that
reddens if the `unregister`-before-read ordering in `readPump`'s defer is broken.

**(c) Whatever else is cheap once the package has a test file at all** — room lifecycle (lazily created,
garbage-collected when empty), slow-consumer drop.

## 2. Rules

- **Cover the denials, not just the grants.** A permission test that only asserts what is allowed is
  the classic hollow guard: it stays green when the policy is replaced with `return true`. Teeth-check
  by doing exactly that locally and confirming the table reddens.
- Test the real function; don't restate the policy in the test and assert the restatement.
- In-package (`package sync`), so unexported policy is reachable without widening the API.
- `gofmt -l core` clean.

## 3. Acceptance criteria

- `authorizeWrite` covered by a table including deny cases for every role.
- Replacing the function body with an unconditional allow reddens the table — verified and reported.
- The package has in-package tests where it previously had none.
- `go test -race ./core/...` green (T106 lands first).

## 4. Out of scope

Rewriting the authorization policy. WebSocket integration/e2e testing. `apply.go` engine semantics.

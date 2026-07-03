# T11 — Remove dead wiring in core

**Priority:** 11 · **Size:** S · **Area:** `core/cmd/troubacore`, `core/internal/{httpapi,session,bake,sync}`

## Context

The composition root constructs three dependencies that are **discarded** by the router:

- `cmd/troubacore/main.go` (~lines 50–52) builds `syncpkg.New()` (a hub with nil
  dependencies), a `session.Manager`, and a `bake.Baker`, and passes all three to
  `httpapi.Router` — whose signature ignores them (`_` parameters, see
  `httpapi/doc.go` ~line 38). The real hub is built inside `mountWS` via
  `NewHub(eng, wsAuth{})` (`httpapi/ws.go` ~lines 50–51).
- `core/internal/session` is a pure stub (`session/doc.go`) and is vestigial: real auth
  lives in `app.Service` (sessions/bcrypt) adapted through `wsAuth` in `httpapi/ws.go`.
- `core/internal/bake` is a stub too, but bake *is* a planned subsystem (invariants
  I11/I12) — its package stays; only the fake injection goes.

Misleading wiring like this makes the composition root lie about the object graph.

## Changes

1. Change `httpapi.Router`'s signature to accept only what it uses. Delete the ignored
   parameters and the corresponding construction in `main.go`.
2. Delete `core/internal/session` entirely (package + references). Add one line to its
   former responsibility in `httpapi/doc.go` or `app`'s doc: sessions are owned by
   `app.Service`. If `docs/ARCHITECTURE.md`'s invariant map (bottom table) references
   `core/internal/session`, update that row (it currently maps I6 to
   `core/internal/{sync,session}` — make it `sync` + `app`).
3. Keep `core/internal/bake` (it documents the planned bake), but stop constructing a
   `Baker` in `main.go` until it has a real API; the router mounts nothing for it today.
4. While in `main.go`: confirm nothing else is constructed-and-dropped; the composition
   root should read as the true object graph (store → engine → app service → router).

## Acceptance criteria

- `make test` green; `make demo` works end-to-end (login, editor, realtime echo).
- `git grep -rn "internal/session" core/ docs/` returns nothing (or only a changelog/ADR
  mention).
- `httpapi.Router` has no unused parameters; `go vet ./...` clean.

## Out of scope

- Implementing bake or sessions-as-a-package; changing auth behavior in any way.

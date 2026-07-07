# TroubaCore

The authoritative TroubaStack server, in Go. TroubaCore holds the single source
of truth (**I6**): a realtime WebSocket sync hub, a REST surface, persistence
with a retention/GC policy, bake orchestration, and it serves the built
**TroubaStudio** SPA from an embedded filesystem (**I10, I14**).

**TroubaCore has no UI.** It depends only on the contract in
[`../proto/`](../proto) (whose generated Go lands under `internal/gen/`) and on
the Node `web/bake` worker it shells out to. It never imports a sibling client.

See the constitution: [`../docs/ARCHITECTURE.md`](../docs/ARCHITECTURE.md).

## Internal package map

| Package | Responsibility | Invariants |
|---|---|---|
| `internal/domain` | Pure model + logic: objects (UUID), linear revisions, pins, LWW, tombstones. No I/O. | I2 I4 I5 I7 |
| `internal/store` | Persistence + the retention/GC policy ladder. GC never breaks a reference. | I4 I5 I7 |
| `internal/sync` | WebSocket hub: rooms per song, optimistic echo, idempotent apply by UUID. | I6 I2 |
| `internal/app` | Relational domain + auth: users/sessions (bcrypt), bands, members, invites, roles (admin/conductor/member), songs/files. | I6 |
| `internal/bake` | Bake orchestration: resolve role/scope, invoke `web/bake`, mint ConcertBundle revisions. | I11 I12 |
| `internal/httpapi` | REST handlers + serving the embedded Studio SPA. | I10 I14 |
| `internal/webassets` | `//go:embed` of the built Studio `dist/`. | I10 I14 |
| `internal/gen` | Generated proto types (git-ignored; produced by buf — **I1**). | I1 |
| `cmd/troubacore` | Entry point: starts the `http.Server`. | I6 I10 |

## Dependency rule (I14)

Dependencies point **only toward the contract**. `core` imports `proto`
(via `internal/gen`) and nothing cross-layer — no `web`, no `app`, no UI. Within
`core`, the lower packages (`domain`, `store`) know nothing of transport
(`sync`, `httpapi`) or `bake`.

## Baking delegates to web/bake for stroke parity (I8)

Stroke geometry exists **once**, in `web/ink`. TroubaCore must **not** render
strokes in Go — a second renderer would drift from Studio. Instead
`internal/bake` **shells out to the Node `web/bake` worker** (the only sanctioned
server-side JS runtime), which reuses `web/ink` and so produces pixel-identical
overlays. Core's job is orchestration: authorize the admin (I11), resolve scope,
run the worker, and mint the flattened-image ConcertBundle as appended revisions
(I12).

## Build

Stdlib-only; builds offline:

```sh
go build ./...
```

Generated proto code (`internal/gen/`) and the embedded Studio `dist/` are
produced by their own build steps and are git-ignored; the scaffold compiles
without them.

## Configuration (CFG01, ADR 0004)

Settings resolve with layered precedence — **most specific wins**:

```
built-in defaults  <  INI file  <  TROUBA_* env vars  <  CLI flags
```

- **File:** `./troubacore.ini` by default (working directory — *not* under the data
  dir). Point elsewhere with `--config <path>` or `TROUBA_CONFIG`. A missing default
  file is fine; a missing *explicitly named* file is a startup error. With no file at
  all, behavior is exactly the previous env-only behavior — every `TROUBA_*` var
  still works as an override.
- **See every knob:** `troubacore --print-default-config` prints the fully-commented
  example — the committed [`troubacore.example.ini`](troubacore.example.ini) is that
  output verbatim (a test keeps them in sync). Sections: `[server] [storage] [mdns]
  [bake] [dev]`, plus a reserved, not-yet-read `[smtp]` hook.
- **Secrets:** `database_url` may carry credentials; prefer injecting it via the
  environment (env wins over the file), and `chmod 600` the file if it holds secrets.

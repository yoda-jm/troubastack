# 0002 — Swappable persistence Store; file/git for local dev, Postgres for scale

- **Status:** Accepted (2026-06-20)
- **Relates to:** I4 (append-only linear history), I7 (GC never breaks a reference),
  I14 (boundaries).

## Context
Production wants a database, but local development should need **zero infrastructure** — no
server-side database to stand up. Persistence was already behind the `store.Store` interface, so the
backend is a free variable, not an architectural commitment.

## Decision
Persistence is **swappable**, chosen by `TROUBA_STORE` at startup. Concrete backends live in
subpackages (`memstore`, `filestore`, `gitstore`, `pgstore`); only the composition root
(`cmd/troubacore`) knows which is live (I14).

| `TROUBA_STORE` | Backend | Use |
|---|---|---|
| `file` *(default)* | plain append-only file tree (`./troubadata`) | simplest zero-infra local dev |
| `git` | go-git repo (pure Go) | versioned dev / small self-host |
| `mem` | in-memory | tests / throwaway runs |
| `pg` | Postgres | production scale, relational queries |

### Why a **git** backend is a strong fit
The domain *is* git's object model:
- linear, append-only revisions → **commits** on one branch (I4; we forbid branching anyway);
- "revert to revision N" → a **revert-commit** equal to N (`git revert`, never `reset`, I4);
- content-addressed PDF/image assets → **blobs**, deduped for free;
- setlist pins / song head → **tags / branch tip**;
- GC = reachability mark-sweep → **`git gc`** prunes unreachable objects (I7 *for free*);
- the annotation timeline is inspectable with `git log`.

`go-git` is **pure Go**, so it preserves core's single static binary (unlike the cgo PDF
rasterizers used by baking). For small self-hosted bands, `git` can even be production — "your
library is a git repo you can clone and back up."

## Consequences
- **Commit model** (per-action granularity, in-memory HEAD, conflicts) is a separate topic — see
  `0003-per-action-commits-in-memory-head.md`.
- Backends are stubs in the scaffold (return `ErrTODO`) and add **no** external dependency yet, so
  `core` keeps building offline. `go-git` / `pgx` are wired when the respective backend is
  implemented.
- Choosing a backend is a one-line change in `openStore()`.

## Rejected / deferred
- *Postgres as the only/default backend* — rejected; it forced infra on local dev.

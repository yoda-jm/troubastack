# T12 — Make ARCHITECTURE.md enforcement claims honest

**Priority:** 12 · **Size:** S · **Area:** `docs/`, `README.md`

## Context

`docs/ARCHITECTURE.md` presents its invariants as "enforced, not aspirational", and each
one carries an enforcement claim ("spec / structure / review / test"). An audit found the
claims overstate reality in specific, checkable ways. A constitution that overclaims
teaches readers to distrust it — the fix is honest per-invariant status, not weaker
invariants.

Audit results to encode (re-verify each against the tree as you edit, since T01–T11 may
have landed in between):

- **Genuinely tested today:** I2, I4, I5 (engine tests + the backend-parametrized
  `storetest` suite), I6 server-side (`httpapi/ws_test.go`).
- **Structural but unguarded:** I3 (types encode it; nothing tests it), I10, I14, I15
  (true by layout/convention; no lint/arch check prevents violation).
- **Half-real:** I7 (only the no-op keep-all GC tier is tested; the cross-layer
  reachability claim is unexercised), I8 (studio uses the single ink renderer, but bake
  is a stub and no pixel-parity/golden test exists anywhere — including the one that
  `app`'s `InkOverlay.kt` comment claims exists).
- **Aspirational:** I1 (no codegen has ever run; Go/TS/Kotlin each hand-write wire
  types — the exact thing I1 forbids), I9, I11, I12, I13 (app-side scaffolds only).
- **Meta:** the doc says "codegen runs in CI" — until T02 landed there was no CI at all,
  and CI still does not run codegen.

Also: the top-level `README.md` says "Pre-implementation scaffold", which is stale —
core, studio, and ink are substantially implemented.

## Changes

1. In `docs/ARCHITECTURE.md`, extend the "How to read enforcement" note with two status
   markers, e.g. **✅ enforced today** / **🎯 target**, and annotate each invariant's
   *Enforced* line accordingly (one short parenthetical each, e.g. for I1:
   "*(target — codegen not yet wired; hand-written mirrors in core/web/app until then,
   see docs/tasks/T09)*"). Keep the rules themselves unchanged — this task edits claims,
   not rules.
2. Fix the invariant→home table at the bottom if T11 removed `core/internal/session`.
3. Remove or correct the false specifics: "codegen runs in CI" (I1), and find the
   golden-parity-test claim in `app/shared/.../seams/InkOverlay.kt`'s comment — soften it
   to "must be guarded by a golden-image parity test (not yet written)".
4. Update `README.md`'s Status section: implemented (core engine/stores/sync, REST API,
   Studio SPA with realtime annotation editing, seeded demo), scaffold-only (bake, mobile
   app, codegen), and point to `docs/tasks/` as the live work queue.
5. Cross-link: add one line to `docs/tasks/README.md`'s intro noting ARCHITECTURE.md now
   carries per-invariant status.

## Acceptance criteria

- Every invariant in ARCHITECTURE.md carries an explicit ✅/🎯 status that matches the
  tree at merge time (spot-check I1, I7, I8 claims against reality).
- `git grep -n "codegen runs in CI" docs/` returns nothing.
- README status section no longer says pre-implementation.
- Docs-only change: `make test` trivially green.

## Out of scope

- Changing any invariant's actual rule; writing the missing tests (each belongs to its
  own future task).

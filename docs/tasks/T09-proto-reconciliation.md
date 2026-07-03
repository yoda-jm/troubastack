# T09 — Reconcile proto with the runtime type set

**Priority:** 9 · **Size:** S · **Area:** `proto/`, `core/internal/{domain,httpapi,sync}`, `web/studio/src/api.ts`

## Context

`proto/` is supposed to be the single source of truth for domain types (invariant I1),
but it has already drifted from what the running system does:

1. **`highlight` is missing from proto.** The Go domain enum
   (`core/internal/domain/domain.go`, ~line 29 `TypeHighlight`), the TS union
   (`web/studio/src/api.ts` ~line 164), and the ink renderer all know a `highlight`
   object type (legacy data renders it), but `object.proto`'s `ObjectType` enum stops at
   `TEXT`. Any persisted `highlight` object is unrepresentable in the "canonical"
   contract.
2. **Dead field:** `Object.scope` (`object.proto` ~line 67) is documented as subsumed by
   the newer `Layer.role_tag` model (the comment at ~line 55 says so explicitly), yet
   both still exist. Since nothing has ever been generated from these files (no `gen/`
   dirs exist anywhere), this is the cheapest moment there will ever be to clean it up.
3. `buf lint` isn't run anywhere (`make proto` is marked deferred).

This task makes the contract *truthful* and *linted*. It deliberately does **not**
switch the clients to generated types — that adoption is a larger, separate decision;
until then the ARCHITECTURE doc must stop claiming codegen is enforced (T12 handles the
wording).

## Changes

1. In `proto/troubastack/v1/object.proto`:
   - Add `OBJECT_TYPE_HIGHLIGHT = 6;` to the `ObjectType` enum, with a comment marking it
     legacy ("demoted to a rect/ellipse style preset; kept for persisted data").
   - Remove the `scope` field from `Object` — **reserve** its tag number and name
     (`reserved 7; reserved "scope";` with the actual number used) so it can never be
     reused. If `Scope` in `common.proto` then has no remaining references, remove that
     enum too (same reservation caution if it's inside a message; a top-level enum can
     just be deleted while nothing generates from it).
2. Run `cd proto && buf lint` and fix every finding. If `buf breaking` is configured via
   `buf.yaml`, note: there is no published baseline yet, so breaking-change checks
   against `main` start applying from this commit forward.
3. Sweep the hand-written mirrors for the same story: Go (`domain.go` enum +
   `httpapi/annotations.go` + `sync/mapping.go` string maps) and TS (`api.ts` union)
   should each carry the exact same set: `freehand, line, rect, ellipse, text,
   highlight`. Fix any mismatch found. Add a comment at each mirror pointing at
   `object.proto` as the authority.
4. Add the `proto` lint step to CI (extend the workflow from T02 if it didn't already
   include it).

## Acceptance criteria

- `buf lint` exits 0 in CI and locally.
- `git grep -n "scope" proto/` shows only the `reserved` markers (and unrelated words).
- The three mirrors (proto enum, Go maps/enum, TS union) list the identical type set —
  quote all three in the PR description.
- `make test` and `make e2e` green (no runtime behavior change expected; `highlight`
  handling already exists everywhere else).

## Out of scope

- Running `buf generate` and adopting generated types in any client (record in the PR
  that this remains open — it is the real I1 debt).
- New annotation types (see T07's dev-flagged arrow; promoting it to proto happens when
  the product wants it).

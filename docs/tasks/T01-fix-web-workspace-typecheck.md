# T01 — Fix web workspace typecheck breakage

**Priority:** 1 (blocks CI, T02) · **Size:** S · **Area:** `web/bake`, `web/studio`

## Context

The npm workspace under `web/` has two hygiene bugs that make a workspace-wide
`tsc --noEmit` impossible and hide a real dependency:

1. `web/bake/src/index.ts` imports `buildStrokeGeometry` and `renderStroke` from
   `@troubastack/ink` — **exports that do not exist**. The ink package
   (`web/ink/src/index.ts`) actually exports `renderObject`, `renderObjects`, and its
   types (`InkObject`, `InkStyle`, `InkPoint`, `PageRect`, `InkObjectType`). The bake
   package is a stub (`bake()` throws `"TODO"`), so nothing runs this code — but it must
   at least typecheck.
2. `web/studio/src/pages/SongEditor.tsx` imports `renderObjects` from `@troubastack/ink`,
   but `web/studio/package.json` does **not** declare `@troubastack/ink` as a dependency
   (it resolves only via npm-workspace hoisting) and its comment still describes ink as
   "deferred". Same check for `@troubastack/proto-gen` style placeholders: only declare
   what is actually imported.

## Changes

1. In `web/bake/src/index.ts`, replace the dangling import with the real API:
   import `renderObjects` (and whatever types the stub's doc comments reference) from
   `@troubastack/ink`. Keep the `bake()` stub throwing `TODO` — do **not** implement the
   bake (invariant I8 work is out of scope here). Update the stub's `void` references and
   comments so they name the real exports.
2. In `web/studio/package.json`, add `"@troubastack/ink": "*"` (workspace-resolved) to
   `dependencies` and fix the stale comment. Check `web/bake/package.json` declares its
   ink dependency too.
3. If `tsc --noEmit` for any of the three packages surfaces further errors, fix them —
   the goal of this task is: **the whole web workspace typechecks**.

## Acceptance criteria

- From `web/`: `npm install` succeeds, then
  `npx tsc --noEmit -p ink && npx tsc --noEmit -p studio && npx tsc --noEmit -p bake`
  all exit 0.
- `make e2e` still green (no behavior change expected).
- `git grep -n "buildStrokeGeometry\|renderStroke" web/` returns nothing.

## Out of scope

- Implementing `bake()` (that is a later, larger task).
- Any rendering behavior change.

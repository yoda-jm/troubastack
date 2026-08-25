# T108 — One e2e setup, not eighty

**Priority:** normal; **before T112** · **Size:** M · **Area:** `web/studio/e2e` (Web & Core lane).
From the 2026-08-25 project audit, §4.3 testing gaps. This debt got *worse* over the last 266 commits,
so it is worth paying before more specs are written on top of it.

## 1. The problem

**77 of 81 specs define their own `register()`.** Several also hand-roll `createBandAndOpen`,
`createSongAndOpen`, and file upload. A change to the registration flow is a 77-file edit, and every
one of those flows is driven through the **UI** even in specs that only need a logged-in user with a
band — which is most of them, and it is a large share of the ~23-minute suite.

## 2. What to build

**(a) A shared helper module** under `e2e/` exporting the setup primitives the specs actually repeat:
`register`, `createBandAndOpen`, `createSongAndOpen`, and an upload helper.

**(b) API-driven setup where the UI path is not what's under test.** The specs already show the
pattern — several use `page.request.get(...)` to read ids. Fixture state that a spec merely *needs*
should be created over the API; only the flow a spec is *about* should be clicked.

**(c) Migrate the specs.** All of them, or a named subset with the rest explicitly listed — partial is
fine if stated, silent partial is not.

## 3. Rules

- **A spec that is about registration keeps driving the UI.** The helper is for specs that need a user,
  not for the ones that test getting one.
- **Do not change what any spec asserts.** This is a setup refactor; a migrated test must keep its
  intent, not merely its selector. If a spec's assertion looks wrong, report it — don't fix it here.
- **The count must reconcile.** The suite is **206** as of T105. If the number moves, say why in the
  same sentence you report it.
- Report the before/after suite runtime — that is the payoff and it should be measured, not assumed.

## 4. Acceptance criteria

- A helper module exists and is used by the migrated specs; no migrated spec defines its own `register`.
- Full `make e2e` green, count reconciled against 206.
- Before/after wall-clock runtime reported.
- Any spec deliberately left unmigrated is listed with its reason.

## 5. Out of scope

`retries`/`workers` config changes and a smoke/full split (worth doing, but after this lands and the
runtime is known). Adding new assertions.

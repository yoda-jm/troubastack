# P204 — Annotation-history compaction (P202's deferred half, I7)

**Priority:** phase 3 — DEFERRED until history disk pressure is real ·
**Size:** M/L · **Area:** `core/internal/store` (all three backends) + engine/app
enumeration surface · **Origin:** P202 re-scope, 2026-07-10 (safe slice landed as
`5ceba9f`; see the gate memo + ruling in `docs/handoff/reviews.md`).

## Why deferred

The real disk-growth source was bake outputs (whole PNGs per rev) — handled by
P202's `troubacore gc`. Annotation history is a JSONL delta chain of small text
records; compacting it is invariant-invasive and buys little until someone has
years of heavy editing. Do not start this without fresh evidence of pressure
(numbers from a real store) and an explicit GO.

## Design ruling (resolved now so the executor doesn't improvise later)

1. **Compaction technique — baseline synthesis:** `SnapshotAt` folds `log[:prefix]`,
   so dropping a prefix requires synthesizing a **baseline snapshot** at the oldest
   kept revision, then rewriting the log as `[baseline, deltas...]`.
   `revisions[i].Number == i+1` is load-bearing across the codebase, so compaction
   **renumbers** — meaning every external reference (pins, `RootSet.KeepRevisions`,
   bake `source_revision`) must be remapped in the same atomic pass, or the store
   must keep a persistent old→new number map. Design the remap FIRST; it is the hard
   part and the I7 risk.
2. **The root set comes from the app layer, not the store** (I7): heads + every pin +
   every bake's `source_revision`. This needs new enumeration surface —
   `app.Repo` is per-user/per-band today (no `AllBands()`/`AllSongs()`), and nothing
   walks all bake revs. Add explicit enumeration methods; do NOT let a store sweep
   its own layer.
3. **gitstore:** go-git has no `Repository.GC()` porcelain. Ruling: stay pure-Go —
   drive the packfile writer (repack reachable objects, drop the old pack); do NOT
   shell out to `git gc` (fights the single-binary deployment, R2). If the packfile
   path proves unreasonable, come back to the gate rather than shelling out.
4. **Safety gate already in place:** `storetest`'s **ReachabilityI7** subtest (landed
   with P202) asserts head + pins + KeepRevisions reconstruct after `Collect`. Any
   implementation must keep it green on all three backends AND extend it to assert
   actual reclamation (size shrank) at the aggressive tier — the half P202
   deliberately left safety-only.

## Acceptance criteria

- All P202 acceptance criteria that were deferred: `storetest` reachability suite
  extended with a reclamation assertion; default behavior byte-identical (keep-all);
  a seeded + heavily-edited dataset demonstrably shrinks with every pin/bake still
  openable (numbers in the PR).
- Renumber-remap correctness proven by a test where a pin and a bake
  `source_revision` both reference revisions ABOVE the compacted prefix.

## Out of scope

- Setlist revision-pins (product decision — VLL; if adopted they join the root set).
- Operator auth tier / HTTP GC endpoint (rides OPS01; the CLI subcommand stands).
- pgstore; device-side cache eviction.

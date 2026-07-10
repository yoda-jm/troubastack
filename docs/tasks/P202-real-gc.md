# P202 — Real GC: cross-layer reachability + retention tiers (I7)

**Priority:** phase 2 · **Size:** M/L · **Area:** `core/internal/store`, `core/internal/bake`

> **RE-SCOPED 2026-07-10 (arch ruling, reviews.md):** the safe slice — item 4 (bake
> output retention: `bake.PruneOutputs` + `troubacore gc` + `bake.keep_revs`, default
> keep-all) and the safety half of item 5 (the `storetest` **ReachabilityI7** contract
> suite, green on all three backends) — **landed as `5ceba9f`**. Items 1–3 (real
> reachability GC) rest on constructs that don't exist yet (no global enumeration
> surface, no setlist revision-pins, history is a delta chain needing baseline
> synthesis + renumbering to compact) and are re-filed as
> [`P204-history-compaction.md`](P204-history-compaction.md) with the design ruling
> resolved. Setlist revision-pins are a product decision (VLL); an operator auth tier
> for any HTTP GC endpoint rides OPS01. This file is kept for history; P202 is CLOSED.

## Context

Invariant I7 (tagged 🎯 in ARCHITECTURE.md): GC must be a single cross-layer
reachability pass over a global root set — and today every backend's `Collect` is a
no-op, tested only for the trivial keep-all tier. That's *allowed* (retention is
optional, never correctness), but disk grows forever: every revision, every bake.
Phase 2 = make pruning real without ever breaking a reference.

## Changes

1. **The global root set, assembled in one place** (engine/app layer, NOT per-store):
   song heads, every setlist pin, every bake's `source_revision` (B02 records them),
   plus any retained-milestone anchors. This is the I7 point — a per-layer sweep is
   forbidden because another layer may hold the reference.
2. **Retention tiers** per `store/doc.go`'s ladder: keep-all (default, unchanged),
   keep-reachable(+grace window). Configuration via env/admin endpoint; default stays
   keep-all.
3. **Implement `Collect`** for filestore (rewrite JSONL dropping unreachable revisions'
   deltas — careful: linear history means "unreachable" only ever applies to fully
   superseded prefixes that no root can reconstruct through; when in doubt, keep) and
   gitstore (translate roots to git refs, then real `git gc` — the design's "git gc IS
   the shared GC"). memstore analogous. pgstore stays a stub.
4. **Bake pruning**: old bake outputs under `bakes/` beyond the last N revs per concert
   (default keep 3), never pruning a rev any device could still be pinned to — server
   can't know device pins, so document the trade-off and keep at least anything
   `final_locked` or newer than a config window.
5. **The I7 proof test** (the audit's ask): build a state with cross-layer references
   (a pin held by a setlist to an old revision, a bake sourcing another), run Collect
   at the aggressive tier, then **reconstruct every rooted revision successfully** and
   verify unreachable ones are actually gone (size shrank). Run against all three real
   backends via the storetest contract suite.

## Acceptance criteria

- `storetest` gains the reachability suite and all backends pass it; `make test` green.
- Default behavior byte-identical to today (keep-all; a no-op Collect stays legal).
- A seeded + baked + heavily-edited dataset demonstrably shrinks under the aggressive
  tier with all pins/bakes still openable (numbers in the PR).

## Out of scope

- Device-side cache eviction (the app's bundle storage is the user's to manage for
  now); pgstore.

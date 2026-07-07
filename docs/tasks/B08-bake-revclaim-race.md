# B08 — Bake rev-claim race: publish `rename` can fail against a concurrently-published rev

**Priority:** correctness (rare, real) · **Size:** XS/S · **Area:** `core/internal/bake`

## Context

B04 made rev publication atomic (stage in `<rev>.tmp`, write the `.tstage`, then
`os.Rename` the stage dir to `<rev>`). But the rev-*claim* loop and the publish step
have a residual race that can make a concurrent same-setlist bake **fail outright**,
not merely flake a test. Verified by reading `baker.go` (2026-07-07) and reproduced
intermittently in CI as `TestBake_ConcurrentSameSetlist_distinctRevs` failing with
`rename …/<n>.tmp …/<n>: file exists` (hit on T19's and T25's first CI runs; cleared
on `go`-job re-run).

**The window** (`baker.go:114`–`174`):
1. `nextRev` (`baker.go:315`) counts only PUBLISHED numeric dirs — `.tmp` claims are
   invisible to it (ParseUint rejects `<n>.tmp`).
2. The claim loop (`baker.go:119`) bumps `rev` only on an `os.Mkdir(<rev>.tmp)`
   **IsExist** collision — i.e. it detects a concurrent *in-flight* claim, but NOT a
   rev that has already been *published*.
3. Publish (`baker.go:174`) is `os.Rename(stageDir, <rev>)` with **no** IsExist
   handling.

So: bake A claims `1.tmp`, publishes (`rename 1.tmp → 1`, freeing `1.tmp`). Bake B,
whose `nextRev` ran before A published (got `1`), now does `os.Mkdir("1.tmp")` — which
**succeeds** (A already renamed it away), so the claim loop sees no collision. B bakes,
then `rename 1.tmp → 1` fails because `1` now exists → the whole bake returns an error.
Fast fake-bakes hit the window; multi-second real bakes rarely would — but "rarely" on
a shared LAN server during a rehearsal is still a real failed bake.

## Changes

1. `baker.go` claim loop: treat an existing **published** `<rev>` dir as a collision
   too — bump `rev` if `os.Stat(<concertDir>/<rev>)` exists OR `os.Mkdir(<rev>.tmp)`
   returns IsExist. (Cheap stat before the mkdir, or check both.)
2. Publish belt-and-suspenders: if `os.Rename(stageDir, <rev>)` returns IsExist,
   re-claim a higher `rev` (re-run the claim loop from `rev+1`), rewrite the
   `.tstage` under the new name, and retry the rename — rather than failing. Keep the
   single-publication-point guarantee (readers still only ever see a complete `<rev>`).
3. The stale-`.tmp` wart B04 documented (a hard-killed process leaves a `<rev>.tmp`
   that permanently skips that rev *number*) stays acceptable — do NOT "fix" it into a
   new race; just don't regress it.

## Acceptance criteria

- `TestBake_ConcurrentSameSetlist_distinctRevs` passes **1000×** under `-race`
  (`go test ./internal/bake -run ConcurrentSame -count=1000 -race`) — currently it
  flakes within a few hundred.
- A new focused test drives the exact ordering (A publishes between B's `nextRev` and
  B's `mkdir`) — e.g. inject a hook or use a tiny fake clock/barrier — and asserts B
  gets a distinct published rev, not an error.
- Both concurrent bakes end with distinct, fully-published `<rev>` dirs + `.tstage`
  files; no `.tmp` left behind on success; `go vet` + full `go test ./...` green.

## Out of scope

- A per-setlist mutex (the lock-free claim is deliberate — B01/B04 lineage); reworking
  `nextRev` to a monotonic counter file; the stale-`.tmp` number-skip (accepted, B04).

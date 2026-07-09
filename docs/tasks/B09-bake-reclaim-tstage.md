# B09 — Bake re-claim path can orphan/mismatch a published `<rev>.tstage`

**Priority:** correctness (very narrow tail; follow-up to B08) · **Size:** XS/S ·
**Area:** `core/internal/bake`

## Context

B08 (`86f1edd`) fixed the rev-claim race so a concurrent same-setlist bake re-claims a
higher rev instead of failing. The **primary** path is safe: `<rev>.tmp` is exclusive
(`os.Mkdir`), so only the holder ever writes `<rev>.tstage`, and the claim-loop stat of
the published `<rev>` dir catches the common window. Verified: the deterministic
2-bake test + the concurrent guard pass 50× under `-race`.

**The residual tail (found in B08 review):** the publish **re-claim** inner loop
(`baker.go`, publish `for` loop) picks the next rev by scanning only for a free
*published dir* (`os.Stat(<rev>)` fails) — it does **not** take an exclusive `<rev>.tmp`
claim for the re-claimed number. And on a lost rename it does
`os.Remove(tstagePath)`. So if **two** bakes both fail their initial publish and both
re-claim the **same** higher rev N:
- both write `N.tstage` (second overwrites the first);
- one wins `rename → N/`, the other's rename fails, stats `N` (exists), and
  **`os.Remove(N.tstage)`** — which deletes the *winner's* `N.tstage`;
- result: `N/` (a real published dir) with a **removed or content-mismatched
  `N.tstage`**. `downloadBundle` serves `<rev>.tstage` via `os.Open` (the file is
  authoritative), so a download of rev N 404s or serves the wrong bake's bytes.

This is strictly narrower than the bug B08 fixed (needs ≥2 bakes racing the *same
re-claimed* rev, and multi-second real bakes make the window near-unreachable), and it
is NOT a regression worth reverting B08 for — but it shouldn't rot.

## Changes (pick one)

1. **Two-phase `.tstage` publish (preferred).** Write the tstage to a unique temp
   (e.g. `<rev>.tstage.<pid/uuid>.tmp` or inside `stageDir`), rename the **dir** first
   (the atomic arbiter), and only on a winning dir-rename `os.Rename` the temp onto
   `<rev>.tstage`. The loser never created `<rev>.tstage`, so it has nothing to remove
   and can't touch the winner's. Reconcile with B04's intent (readers must not see a
   `<rev>/` dir without its `.tstage`): if `latestRev`/`BundlePath` keys off the dir,
   publish the `.tstage` in the same breath as the dir-rename win, before returning.
2. **Exclusive re-claim.** Make the re-claim take `<rev>.tmp` via `os.Mkdir` (bump on
   IsExist), same as the initial claim loop, so two bakes can't both target the same
   re-claimed rev — restoring the `.tmp`-exclusivity that makes the primary path safe.

## Acceptance criteria

- A new test drives TWO bakes both re-claiming the same rev (extend the `afterNextRev`
  seam, or add an `afterClaim` seam) and asserts BOTH published revs have a present,
  content-matching `<rev>.tstage` (download-openable), no orphan/mismatch.
- `TestBake_PublishReclaimsOnConcurrentPublish` + the concurrent guard still pass ≥100×
  under `-race`; `go test ./...` green.

## Out of scope

- A per-setlist mutex (the lock-free lineage stays); the accepted B04 stale-`.tmp`
  number-skip.

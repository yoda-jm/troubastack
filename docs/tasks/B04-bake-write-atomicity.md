# B04 — Bake robustness: atomic rev publication + concurrency guard (B02 follow-ups)

**Priority:** B-track, before/with B03 · **Size:** XS/S · **Area:** `core/internal/bake`

## Context

Two review findings from B02 part 1 (reviews.md, 2026-07-05), deferred out of part 2:

1. **`nextRev` race.** `Baker.Bake` computes `nextRev` by scanning rev dirs, then
   `os.MkdirAll`s it — which succeeds even when the dir already exists. Two concurrent
   bakes of the same setlist mint the same rev and interleave writes into one dir → a
   torn bundle that hashes/parses wrong.
2. **Partial-bake visibility.** `latestRev`/`ListConcerts`/`BundlePath` see a rev dir the
   moment it exists, but `bundle.json` and the `.tstage` are written afterwards — a
   concurrent list/download can 404 (or read a half-written manifest) for a rev that is
   mid-write, even though an older complete rev exists.

## Changes

1. Bake into a staging dir (`<concertId>/<rev>.tmp` or an `os.MkdirTemp` sibling) and
   `os.Rename` it to `<rev>` only after `bundle.json` AND the `.tstage` are fully
   written — publication becomes atomic, closing finding 2 entirely.
2. Claim the rev with `os.Mkdir` (not `MkdirAll`) on the FINAL rev dir name (or the
   staging name embedding the rev): on `os.IsExist`, re-scan and retry with the next
   number — closing finding 1 without a mutex. (A per-setlist `sync.Mutex` in `Baker`
   is an acceptable simpler alternative if the retry loop reads worse; say which.)
3. `latestRev` must skip non-numeric/staging entries (it already skips non-dirs; make
   sure `<rev>.tmp` never parses as a rev — the `.tmp` suffix already guarantees that
   with `ParseUint`; keep it that way and note it).

## Acceptance criteria

- A test that runs two `Bake` calls concurrently for the same setlist (fakes are fine)
  and asserts they produce two DISTINCT revs, both loadable with all refs resolving.
- A test (or the same one) asserting no `<rev>.tmp`/staging dir is visible via
  `ListConcerts`/`BundlePath` after the bakes complete.
- `go build/vet/test ./...` green; no API/endpoint changes.

## Out of scope

- Bake GC/retention (P202). Distribution (B03). Exposing full bake history over the API
  (`ListConcerts` intentionally returns latest-per-concert; the Studio card's "history"
  list is therefore latest-only today — fine for v1, revisit with B03 if wanted).

# T22 — Deterministic list ordering (songs lexicographic; sweep all listings)

**Priority:** quick fix, user-reported (VLL 2026-07-07) · **Size:** S · **Area:** `core/internal/app` (repos)

## Context

The song list reorders itself on every request: `SongsOfBand` in BOTH `memrepo` and
`filerepo` iterates a Go **map** (`for _, s := range r.songs`), whose order is
deliberately randomized per iteration. Verified by code read; user-visible in Studio's
song list. The same pattern exists in the other `…OfBand`/listing methods (≥4 map
iterations in memrepo alone) — sweep them all, don't fix just songs.

**Ordering decisions (resolved):**
- **Songs: lexicographic by Title** (case-insensitive; tiebreak by ID for stability) —
  per VLL's request.
- **Setlists: lexicographic by Name** (ci, tiebreak ID).
- **Bands (a user's list): lexicographic by Name** (ci, tiebreak ID).
- Song **files** keep `DisplayOrder` (already user-controlled — just make the tiebreak
  deterministic: DisplayOrder, then ID); `my-files` keeps its explicit order.
- Invites/invite-links/members: any stable order (CreatedAt then ID) — pick one and
  test it.

Sort **in the service layer or at the repo boundary — once, consistently across both
backends** (a shared helper the repos or the service applies), so mem and file never
disagree. Add the ordering to the endpoint tests (create out of order → assert sorted)
for songs, setlists, bands at minimum.

## Acceptance criteria

- Two consecutive `GET /api/bands/{b}/songs` return identical, lexicographic order;
  same for setlists/bands. A Go test creates entries out of order and asserts the
  sorted result on BOTH backends (the `storetest`-style parametrization if convenient).
- No map-iteration-ordered list remains user-visible: `grep -n 'for .* := range r\.'`
  over the repos — every listing either sorts before returning or is documented as
  order-irrelevant internal.
- `make test` + e2e green (e2e may already implicitly depend on some order — fix specs
  honestly if they relied on luck).

## Out of scope

- User-configurable sort; pagination; UI sort toggles.

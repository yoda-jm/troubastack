# P201 attended 2-device test — RESULT: data loop works, live render is STALE (for arch)

**Status:** bug found + root-caused on-device, awaiting arch decision on the fix ·
**Raised by:** Mobile lane (2026-07-19) · **Area:** `app/shared` (Stage render:
`PageImageCache` / `decodeCached` / `PageView`), relates to P201 change 4 (R10) +
B1 (the pin/cache-budget work) · **Task:** #23.

VLL asked to run P201's remaining acceptance — the attended 2-device rehearsal test.
Done, fully driven from this host (Playwright as the editor + the QA tablet as Stage,
against the running demo). **The autobake/download/import loop works end to end. The
live in-Stage refresh does NOT — it serves a stale page.**

## What was run (all real, on-device)

1. Set "Sat @ The Anchor" **live** (`POST …/setlists/{id}/live`).
2. **Playwright** logged into the demo Studio as marie, opened Wonderwall, drew a rect
   → real WS commit → **autobaker produced rev1** (~21 s). ✅
3. Tablet (connected as marie): TroubaStudio → the **download offer** appeared →
   downloaded → the concert is now server-backed (rev1). ✅
4. Opened it in Stage → **auto-matched Marie, no picker**, and **draw1's rect renders**
   on the page. ✅ (Confirms draw→bake→download→display.)
5. Settings → **Auto-update (rehearsal) toggle ON** (present because server-backed;
   the ● Live indicator lit). ✅
6. **Playwright drew a SECOND rect** → autobaker produced **rev2** (~21 s).
7. Waited > 60 s on the open Stage. **The page never changed** (byte-identical
   screenshots at 18 s, 30 s, and after a page-nav round-trip).
8. **Data check:** the tablet's bundle on disk is now **rev2** (concertId `1314ae52…`,
   `rev: 2`, matching the server) — so the poller DID download + import.
9. **Re-opened** the concert (fresh Stage) → **both rects render.** ✅

So: **poll → download → import → disk = rev2 all fire; the OPEN Stage keeps showing
rev1** until re-open. This is the R10 acceptance ("Stage device shows it within ~15 s
without moving the page") **not met** — the position is preserved, but the content
isn't repainted.

## Root cause (confirmed in code)

Baked page blob refs are **stable, index-based filenames**, not content-addressed:
`pageRasterRef: "blobs/s0-p0-raster.png"`, overlay `imageRef:
"blobs/s0-p0-L-<layerId>.png"`. `PageImageCache` keys purely on the ref+size —
`cacheKey(ref,w,h) = "$ref@${w}x$h"` (StageScreen.kt) — and B1's `pin` makes the
displayed page's entries **eviction-proof**. `applyUpdate` swaps state to rev2 whose
pages carry the SAME ref strings, so `decodeCached` returns the **cached rev1 bitmap**.
The `rasterHash`/`contentHash` that DO change per rev are never part of the cache key.
(The StageScreen.kt comment "…stop 'same page, fewer annotations'" shows a related
symptom was hit before and mitigated by raising the budget 12→64 — which, for a
same-key content swap, actually makes the stale entry *stickier*.)

`applyUpdate`/`remapCurrent` themselves are correct (state → rev2, position remapped);
the defect is purely the ref-keyed bitmap cache surviving a content swap.

## Proposed fix (needs arch pick)

- **(a) Hash the cache key (preferred).** Include the page/overlay content hash in the
  key: `"$ref#$hash@${w}x$h"`. A content change → new key → natural miss → fresh
  decode; stale entries age out via LRU. Threads `rasterHash`/overlay `contentHash`
  (already in `PageInfo`/state) into the `decodeCached`/`pageCacheKeys` call sites.
  Self-correcting, no explicit invalidation, keeps the B1 pin semantics.
- **(b) Invalidate on apply.** Add `PageImageCache.clear()` (or evict the swapped
  page's keys) and call it from the Stage host when `applyUpdate` bumps the rev.
  Simpler, but re-decodes neighbours and needs the host to know a swap happened.

Lean (a): it fixes the whole class (any same-filename content swap), not just this path.

## Ask for Fable

- Pick (a) or (b); confirm it's a mobile-lane fix I implement + unit-test (a cache
  test: same ref, different hash ⇒ miss) + re-run this device test.
- P201 stays code-complete otherwise; this is the one acceptance gap the attended test
  surfaced. iOS host wiring remains separate (needs a Mac).

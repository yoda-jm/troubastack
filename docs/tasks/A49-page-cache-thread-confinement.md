# A49 — Page-image cache: restore thread confinement (audit C5)

**Lane:** Mobile · **Severity:** Critical (audit C5, open since 2026-08-25, previously unspecced)
**Files:** `app/shared/src/commonMain/kotlin/com/troubashare/shared/stage/StageScreen.kt`,
`.../stage/LruCache.kt`, `app/shared/src/androidUnitTest/.../stage/` (new test)

## The defect

`LruCache` states its own contract in its KDoc:

> *"Not thread-safe — [PageImageCache] uses it only from the composition (main) thread."*

`PageImageCache` repeats it, more specifically:

> *"Accessed only from the composition (main) thread; the heavy decode runs off-thread and results are
> stored back here on the main thread."*

**Current `origin/main` implements neither half.** `decodeCached` (`StageScreen.kt:1136`) does
`cache.get()` → `decoder.decode()` → `cache.put()` as one unit, and every caller invokes that whole
unit from inside `withContext(Dispatchers.Default)`. So the cache `get` *and* the `put` both run on a
worker — the KDoc's "stored back here on the main thread" is false as written.

Four concurrent mutation paths reach two plain `LinkedHashMap`s (`map` and `pins`):

| # | Site | Thread | Mutates |
|---|---|---|---|
| 1 | `:330–332` neighbour prefetch (`LaunchedEffect` → `withContext(Default)`) | worker | `map` (get/put) |
| 2 | `:925–928` `ScrollPage` decode (`produceState` → `withContext(Default)`) | worker | `map` + `pins` (`pin`) |
| 3 | `:985–988` `PageView` decode, two-up (`produceState` → `withContext(Default)`) | worker | `map` + `pins` (`pin`) |
| 4 | `:917`, `:979` `DisposableEffect { onDispose { cache.unpin(owner) } }` | **main** | `pins` (`unpin`) |

Sites 1–3 are independent coroutines, so they genuinely overlap. **Site 4 is the one the audit row
missed:** `unpin` runs on the main thread when a page leaves composition, mutating `pins` while a
worker is inside `pin`/`get`/`put`. That means **the race does not require two-up** — any page turn
that disposes a page while its neighbour's decode is still in flight is enough. The audit's
"two-up + prefetch" framing understates the exposure.

Blast radius, worst-first: corrupted `LinkedHashMap` internal state (on the JVM, a resize racing an
insert can spin); a dropped or mis-ordered entry surfacing as the **wrong page image**; or a lost pin
re-opening B1's "same page, fewer annotations" — the exact bug pinning was written to kill.

`LruCacheTest` has 8 tests. All are single-threaded eviction/pinning semantics. **The invariant the
class documents is guarded by nothing.**

## What I have NOT established

I have shown the documented invariant is violated at four sites. I have **not** shown it corrupts in
practice. Treat this the way T106 treated C4: **install the detector first and let it decide** — do
not open with the fix and assert the fix was needed.

## Deliverable

### 1. The arbiter test (do this first, on unmodified main)

A stress test that drives `PageImageCache` the way `StageScreen` does — concurrent `get`/`put` from
several workers while another thread interleaves `pin`/`unpin` — and asserts the cache survives:
no exception, `size` never exceeds `maxEntries`, and every pinned key is still resolvable.

**It must run with real parallelism.** Put it in `androidUnitTest` (runs on a real JVM), *not*
`commonTest`. `runTest`'s default scheduler is single-threaded and **will not reproduce this** — a
test that cannot fail guards nothing. Use real threads or `Dispatchers.Default` with enough
contention to be reliable, and state the iteration count you settled on.

Report honestly: **does it redden on unmodified `origin/main`?**
- If yes — quote the failure. That is the finding.
- If no, after a genuine effort — say so plainly and say what you tried. The confinement below still
  lands (the contract is still violated and the fix is small), but the report must not claim a
  reproduction it did not get.

### 2. The fix — DECISION PINNED: thread-confine, do not add a lock

Restore what both KDocs already promise: **all `PageImageCache` access happens on the main
(composition) thread; only `decoder.decode(...)` runs on `Dispatchers.Default`.**

I considered making `PageImageCache` internally thread-safe and am **ruling against it**: `commonMain`
has no `synchronized`, and `coroutines.Mutex.withLock` is `suspend`, which does not fit the plain
`fun get`/`put`/`pin` surface. It would mean either a new dependency (`atomicfu`) or `expect`/`actual`
locks — new per-platform surface, in concert week, to hold a line the code already claims to hold.

This inverts `decodeCached`: it can no longer be a plain function called from inside `withContext`.
The `withContext(Dispatchers.Default)` must wrap **only the decode**, with the cache lookup before it
and the store after it, both on the caller's (main) thread. Shape it however reads best — a suspend
helper that takes the cache and does lookup → `withContext { decode }` → store is the obvious one.

**A benign double-decode is acceptable and expected:** two coroutines can both miss and both decode
the same key, and both `put`. That wastes a decode; it cannot corrupt. **Do not add a lock, a
dedupe map, or an in-flight registry to prevent it** — that reintroduces exactly the shared mutable
state this task removes.

### 3. Teeth-check (reproducibility claim — name it precisely enough to re-run)

Revert the confinement (move `cache.get`/`put`/`pin` back inside `withContext(Dispatchers.Default)`)
and confirm the new test reddens. **Report the reddened count**, not just "it fails".

## Scope fences

- **No behaviour change to what is displayed.** Same pages, same overlays, same pin semantics.
- **`LruCacheTest`'s 8 existing tests stay green, unmodified.** B1's eviction/pinning guarantees are
  not up for renegotiation here.
- **No new dependency.** No `atomicfu`, no `expect`/`actual` lock.
- **Do not touch the N9 prefetch policy** (`PREFETCH_SETTLE_MS`, `prefetchTargets`) — this task moves
  where cache access happens, not when prefetch fires.
- **Do not change `cacheKey`/`pageCacheKeys`** (P201/R10 content-hash keying).

## Gate

`:shared:testDebugUnitTest` and `:androidApp:test` green — **read the results XML, never the exit
code**. Cite the new test by name, the redden/green contrast from §1 and §3, and the iteration count.

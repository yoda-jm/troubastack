package com.troubashare.shared.stage

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import kotlin.random.Random
import kotlin.test.Test
import kotlin.test.assertTrue

/**
 * A49 — the arbiter for the page-cache thread-confinement contract (audit C5). [LruCache] documents
 * "not thread-safe — used only from the composition (main) thread", and [cacheThrough] is the extracted
 * confinement core StageScreen's decode paths now go through: `get`/`put` on the CALLER's thread, only
 * the decode on `Dispatchers.Default`. Every production caller is a composition coroutine, which is
 * main-confined.
 *
 * This drives `cacheThrough` + `pin`/`unpin` from many coroutines on ONE confined dispatcher (models the
 * main thread), with the decode fanning out to the real multi-thread pool, over enough contention to be
 * reliable. It asserts the cache SURVIVES: no exception, size within budget, the pinned set resolvable.
 *
 * Real parallelism is essential — `runTest`'s scheduler is single-threaded and would never reproduce
 * this, which is why this is in androidUnitTest, not commonTest. String values stand in for the Compose
 * ImageBitmap, exactly as [LruCacheTest] does.
 *
 * §1 finding (unmodified main): running the same workload with cache access on the multi-thread pool
 * instead of `cacheThrough` reddened at iteration 0 with `java.util.ConcurrentModificationException`.
 * §3 teeth-check: reverting `cacheThrough` to run `get`/`put` inside the decode's `Dispatchers.Default`
 * block reddens THIS test (get/put back on the worker pool) — production and test break together.
 */
@OptIn(ExperimentalCoroutinesApi::class) // limitedParallelism (models the single composition thread)
class PageCacheConcurrencyTest {

    private val MAX = 64
    private val KEYSPACE = 400
    private val WORKERS = 8
    private val OPS_PER_WORKER = 4000
    private val PINNED = (0 until 8).map { "hot-$it" } // a small pinned set: the budget must still hold

    private fun runWorkload(): Throwable? = runBlocking {
        val cache = LruCache<String, String>(MAX)
        PINNED.forEach { cache.put(it, "v-$it") }
        cache.pin("screen", PINNED.toSet())

        // ONE confined dispatcher for ALL cache access = the composition (main) thread analogue.
        val main = Dispatchers.Default.limitedParallelism(1)
        val failures = mutableListOf<Throwable>()

        val jobs = (0 until WORKERS).map { w ->
            launch(main) {
                val rng = Random(w * 7919 + 1)
                repeat(OPS_PER_WORKER) { i ->
                    try {
                        val key = "k-${rng.nextInt(KEYSPACE)}"
                        // get/put stay on `main` (the caller); only the decode fans to the real pool.
                        cacheThrough(cache::get, cache::put, key) { withContext(Dispatchers.Default) { "v" } }
                        if (i % 16 == 0) cache.pin("owner-$w", setOf("k-${rng.nextInt(KEYSPACE)}"))
                        if (i % 16 == 8) cache.unpin("owner-$w") // site 4: a disposer, on the same confined thread
                    } catch (t: Throwable) {
                        synchronized(failures) { failures.add(t) }
                    }
                }
            }
        }
        jobs.forEach { it.join() }
        // All `main` coroutines are done (joined) → safe to inspect from here. Survival invariants:
        val lostPin = PINNED.firstOrNull { cache.get(it) == null } // B1: a pinned key must never be evicted
        synchronized(failures) { failures.firstOrNull() }
            ?: when {
                lostPin != null -> IllegalStateException("pinned key '$lostPin' was lost")
                cache.size > MAX -> IllegalStateException("size ${cache.size} exceeded budget $MAX")
                else -> null
            }
    }

    @Test
    fun cacheThrough_survivesConcurrentAccess_whenConfined() {
        repeat(20) { iter ->
            val fail = runWorkload()
            assertTrue(fail == null, "confined cacheThrough workload must survive (iter $iter): $fail")
            // the pinned set must still be resolvable — a lost pin is B1's "same page, fewer annotations".
        }
    }
}

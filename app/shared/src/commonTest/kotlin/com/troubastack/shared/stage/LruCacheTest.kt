package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/**
 * The eviction/access-order matrix for [LruCache] (the logic behind PageImageCache) — the same
 * scratch cases the reviewer verified for the A04 LRU-portability fix, now committed. String values
 * stand in for the Compose ImageBitmap that blocked testing the cache directly.
 */
class LruCacheTest {

    @Test
    fun evictsLeastRecentlyUsed_notLeastRecentlyInserted() {
        val c = LruCache<String, String>(maxEntries = 2)
        c.put("a", "1")
        c.put("b", "2")
        c.get("a")            // touch a -> b is now the LRU
        c.put("c", "3")       // over cap -> evict b (not a, though a was inserted first)
        assertEquals("1", c.get("a"))
        assertNull(c.get("b"))
        assertEquals("3", c.get("c"))
        assertEquals(2, c.size)
    }

    @Test
    fun reInsertCountsAsAccess() {
        val c = LruCache<String, String>(maxEntries = 2)
        c.put("a", "1")
        c.put("b", "2")
        c.put("a", "1b")      // re-put a -> most-recently-used, and value updated
        c.put("c", "3")       // evict b
        assertEquals("1b", c.get("a"))
        assertNull(c.get("b"))
        assertEquals("3", c.get("c"))
    }

    @Test
    fun missDoesNotPerturbOrder() {
        val c = LruCache<String, String>(maxEntries = 2)
        c.put("a", "1")
        c.put("b", "2")
        assertNull(c.get("zzz"))   // absent lookup must not touch order
        c.put("c", "3")            // still evicts the true LRU, a
        assertNull(c.get("a"))
        assertEquals("2", c.get("b"))
        assertEquals("3", c.get("c"))
    }

    @Test
    fun respectsCapacity() {
        val c = LruCache<String, Int>(maxEntries = 3)
        for (i in 1..10) c.put("k$i", i)
        assertEquals(3, c.size)
        assertEquals(10, c.get("k10"))
        assertNull(c.get("k7"))
    }

    @Test
    fun pinnedEntriesAreNeverEvicted() {
        // B1: the on-screen page's keys are pinned; churn must evict only NON-pinned entries.
        val c = LruCache<String, Int>(maxEntries = 2)
        val owner = Any()
        c.put("raster", 0)
        c.put("overlay", 0)
        c.pin(owner, setOf("raster", "overlay"))
        for (i in 1..10) c.put("other$i", i) // heavy churn past capacity
        assertEquals(0, c.get("raster"), "pinned raster must survive eviction")
        assertEquals(0, c.get("overlay"), "pinned overlay must survive eviction")
    }

    @Test
    fun unpinnedEvictNormallyAroundPinned() {
        val c = LruCache<String, Int>(maxEntries = 3)
        val owner = Any()
        c.put("keep", 1)
        c.pin(owner, setOf("keep"))
        c.put("a", 1); c.put("b", 1); c.put("c", 1) // over cap; "keep" is pinned so an unpinned goes
        assertEquals(1, c.get("keep"))
        assertNull(c.get("a"), "the LRU non-pinned entry is evicted, not the pinned one")
    }

    @Test
    fun multipleOwnersPinnedSimultaneously_noneEvicted() {
        // B1 (A19-condition): two-up / scroll compose several pages at once; each pins under its own
        // owner and eviction must skip the UNION — no owner's pin clobbers another's.
        val c = LruCache<String, Int>(maxEntries = 4)
        val left = Any(); val right = Any()
        c.put("L-raster", 0); c.put("L-overlay", 0)
        c.pin(left, setOf("L-raster", "L-overlay"))
        c.put("R-raster", 0); c.put("R-overlay", 0)
        c.pin(right, setOf("R-raster", "R-overlay")) // must NOT unpin the left page
        for (i in 1..10) c.put("other$i", i) // heavy churn past capacity
        assertEquals(0, c.get("L-raster"), "left page's pin survives after right page pins")
        assertEquals(0, c.get("L-overlay"))
        assertEquals(0, c.get("R-raster"))
        assertEquals(0, c.get("R-overlay"))
    }

    @Test
    fun unpinningOneOwnerFreesOnlyItsKeys() {
        val c = LruCache<String, Int>(maxEntries = 4)
        val left = Any(); val right = Any()
        c.put("L", 0); c.pin(left, setOf("L"))
        c.put("R", 0); c.pin(right, setOf("R"))
        c.unpin(left) // left page left composition; its key is now evictable, right's is not
        for (i in 1..10) c.put("other$i", i) // heavy churn past capacity
        assertNull(c.get("L"), "unpinned owner's key is evictable again")
        assertEquals(0, c.get("R"), "the still-pinned owner's key survives")
    }
}

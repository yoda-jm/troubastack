package com.troubashare.shared.stage

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
}

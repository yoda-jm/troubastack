package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNotEquals

/**
 * P201/R10 (task #23): the page-image cache key MUST include the content version (raster/overlay
 * hash), not just the blob ref. Baked blob filenames are STABLE across revs, so a rehearsal
 * auto-update swaps a page's CONTENT under the same ref — keying on ref alone served the stale bitmap
 * on the live Stage (the attended 2-device test caught it). These guard the invariant that a content
 * change is a cache MISS while an unchanged page still HITS.
 */
class PageCacheKeyTest {

    @Test
    fun sameRef_differentContentHash_isDifferentKey() {
        val ref = "blobs/s0-p0-L-abc.png"
        val v1 = cacheKey(ref, "hashREV1", 800, 2400)
        val v2 = cacheKey(ref, "hashREV2", 800, 2400)
        assertNotEquals(v1, v2, "a content swap under the same ref must NOT collide in the cache")
    }

    @Test
    fun sameRef_sameContentHash_sameSize_isSameKey() {
        val ref = "blobs/s0-p0-raster.png"
        // unchanged page across a re-bake ⇒ same hash ⇒ same key ⇒ cache HIT (no needless re-decode).
        assertEquals(cacheKey(ref, "h", 800, 2400), cacheKey(ref, "h", 800, 2400))
    }

    @Test
    fun differentSize_isDifferentKey() {
        // size still participates (a resize must re-decode at the new dimensions).
        assertNotEquals(cacheKey("r", "h", 800, 2400), cacheKey("r", "h", 400, 1200))
    }
}

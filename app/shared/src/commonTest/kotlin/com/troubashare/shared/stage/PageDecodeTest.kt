package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * B1 — a failed overlay decode must be SURFACED (counted), never silently dropped, so the page can
 * degrade visibly instead of showing fewer annotations. This is the regression guard Fable required.
 */
class PageDecodeTest {

    @Test
    fun allSucceed_noneMissing() {
        val r = decodeOverlays(listOf("a", "b", "c")) { it } // decode never fails
        assertEquals(listOf("a", "b", "c"), r.overlays)
        assertEquals(0, r.missing)
    }

    @Test
    fun aFailedDecode_isCountedNotSilentlyDropped() {
        // Inject a decode failure on "b": the page keeps a,c AND reports missing=1 (→ visible badge).
        val r = decodeOverlays(listOf("a", "b", "c")) { ref -> if (ref == "b") null else ref }
        assertEquals(listOf("a", "c"), r.overlays)
        assertEquals(1, r.missing, "a failed overlay must be reported, never silently absent")
    }

    @Test
    fun allFail_missingEqualsCount_noneRendered() {
        val r = decodeOverlays<String>(listOf("a", "b")) { null }
        assertEquals(emptyList(), r.overlays)
        assertEquals(2, r.missing)
    }

    @Test
    fun noOverlays_isClean() {
        val r = decodeOverlays<String>(emptyList()) { it }
        assertEquals(emptyList(), r.overlays)
        assertEquals(0, r.missing)
    }
}

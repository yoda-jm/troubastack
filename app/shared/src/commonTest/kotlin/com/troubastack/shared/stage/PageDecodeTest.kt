package com.troubastack.shared.stage

import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals

/**
 * B1 — a failed overlay decode must be SURFACED (counted), never silently dropped, so the page can
 * degrade visibly instead of showing fewer annotations. This is the regression guard Fable required.
 *
 * A49: [decodeOverlays] became `suspend` (its decode lambda now confines cache access to the caller and
 * only fans the decode to Dispatchers.Default), so these call it inside `runTest`. The accounting logic
 * is unchanged — same successes-kept / failures-counted contract.
 */
class PageDecodeTest {

    @Test
    fun allSucceed_noneMissing() = runTest {
        val r = decodeOverlays(listOf("a", "b", "c")) { it } // decode never fails
        assertEquals(listOf("a", "b", "c"), r.overlays)
        assertEquals(0, r.missing)
    }

    @Test
    fun aFailedDecode_isCountedNotSilentlyDropped() = runTest {
        // Inject a decode failure on "b": the page keeps a,c AND reports missing=1 (→ visible badge).
        val r = decodeOverlays(listOf("a", "b", "c")) { ref -> if (ref == "b") null else ref }
        assertEquals(listOf("a", "c"), r.overlays)
        assertEquals(1, r.missing, "a failed overlay must be reported, never silently absent")
    }

    @Test
    fun allFail_missingEqualsCount_noneRendered() = runTest {
        val r = decodeOverlays<String, String>(listOf("a", "b")) { null }
        assertEquals(emptyList(), r.overlays)
        assertEquals(2, r.missing)
    }

    @Test
    fun noOverlays_isClean() = runTest {
        val r = decodeOverlays<String, String>(emptyList()) { it }
        assertEquals(emptyList(), r.overlays)
        assertEquals(0, r.missing)
    }
}

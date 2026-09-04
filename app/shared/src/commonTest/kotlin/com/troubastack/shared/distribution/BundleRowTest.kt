package com.troubastack.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** T143 — the concert row must distinguish same-named bakes, and Delete must be reachable in manage but
 *  never in perform (lean). Pure seams, RED-FIRST. */
class BundleRowTest {

    @Test
    fun subtitle_distinguishes_two_same_named_bakes() {
        // Same concert NAME, two bakes: the subtitle (rev + time) must differ — the whole point of T143 §1.
        val a = concertRowSubtitle(1uL, 1_700_000_000L)
        val b = concertRowSubtitle(10uL, 1_700_086_400L)
        assertTrue(a != b, "same-named bakes must render distinguishable subtitles: '$a' vs '$b'")
        assertTrue(a.contains("rev 1"))
        assertTrue(b.contains("rev 10"))
    }

    @Test
    fun subtitle_formats_utc_minute() {
        assertEquals("rev 7 · 2023-11-14 22:13", concertRowSubtitle(7uL, 1_700_000_000L))
    }

    @Test
    fun subtitle_omits_absent_time() {
        assertEquals("rev 0", concertRowSubtitle(0uL, 0L))
    }

    @Test
    fun menu_perform_lean_offers_nothing() {
        assertTrue(bundleMenuActions(lean = true, damaged = false).isEmpty())
        assertTrue(bundleMenuActions(lean = true, damaged = true).isEmpty()) // still lean, even if damaged
    }

    @Test
    fun menu_manage_healthy_offers_delete_alongside_the_rest() {
        val actions = bundleMenuActions(lean = false, damaged = false)
        assertTrue(BundleAction.Delete in actions, "a healthy duplicate must be deletable (VLL's case)")
        assertTrue(BundleAction.Freeze in actions && BundleAction.Pin in actions)
    }

    @Test
    fun menu_damaged_is_delete_only() {
        assertEquals(listOf(BundleAction.Delete), bundleMenuActions(lean = false, damaged = true))
    }
}

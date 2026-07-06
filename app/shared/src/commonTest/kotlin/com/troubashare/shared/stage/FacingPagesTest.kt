package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/** A12 — facing-pages spread math (pure). The layout itself is code-review + screenshot. */
class FacingPagesTest {

    @Test
    fun spreadFor_leftIsEven() {
        assertEquals(0, spreadFor(0))
        assertEquals(0, spreadFor(1))
        assertEquals(2, spreadFor(2))
        assertEquals(2, spreadFor(3))
        assertEquals(20, spreadFor(20))
        assertEquals(20, spreadFor(21))
    }

    @Test
    fun spreadPages_pairsAndLoneLast() {
        // 22-page demo: first spread 1–2, last spread 21–22 (both present).
        assertEquals(listOf(0, 1), spreadPages(0, 22))
        assertEquals(listOf(0, 1), spreadPages(1, 22))
        assertEquals(listOf(20, 21), spreadPages(21, 22))
        // Odd total ⇒ the last page (even index) shows alone.
        assertEquals(listOf(22), spreadPages(22, 23))
        assertEquals(listOf(0), spreadPages(0, 1))
    }

    @Test
    fun spreadPages_songJumpLandsOnPair() {
        // A song whose first page is odd (idx 7) lands on the pair 7–8 (idx 6,7).
        assertEquals(listOf(6, 7), spreadPages(7, 22))
    }

    @Test
    fun spreadPages_emptyBundle() {
        assertEquals(emptyList(), spreadPages(0, 0))
    }

    @Test
    fun turnByTwo_clamped() {
        assertEquals(2, nextSpreadPage(0, 22))
        assertEquals(2, nextSpreadPage(1, 22))
        assertEquals(4, nextSpreadPage(2, 22))
        // Already on the last spread ⇒ next stays inside it (no run-off).
        assertEquals(21, nextSpreadPage(20, 22))
        assertEquals(21, nextSpreadPage(21, 22))

        assertEquals(0, prevSpreadPage(0))
        assertEquals(0, prevSpreadPage(1))
        assertEquals(0, prevSpreadPage(3))
        assertEquals(18, prevSpreadPage(21))
    }

    @Test
    fun label_oneUpTwoUpAndLone() {
        assertEquals("6/22", pagerLabel(5, 22, twoUp = false))
        assertEquals("1–2/22", pagerLabel(0, 22, twoUp = true))
        assertEquals("21–22/22", pagerLabel(21, 22, twoUp = true))
        assertEquals("23/23", pagerLabel(22, 23, twoUp = true)) // lone last page
        assertEquals("0/0", pagerLabel(0, 0, twoUp = true))
    }
}

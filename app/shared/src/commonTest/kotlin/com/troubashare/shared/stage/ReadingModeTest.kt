package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals

/** A14 — the pure parts of continuous-scroll reading mode: the mode cycle, persistence, page turns. */
class ReadingModeTest {

    @Test
    fun nextFitMode_cyclesPageWidthScroll() {
        assertEquals(FitMode.FIT_WIDTH, nextFitMode(FitMode.FIT_PAGE))
        assertEquals(FitMode.SCROLL, nextFitMode(FitMode.FIT_WIDTH))
        assertEquals(FitMode.FIT_PAGE, nextFitMode(FitMode.SCROLL))
    }

    @Test
    fun fitMode_persistenceRoundTrips() {
        // .name out, parse() back in — for every mode (what the entrypoint Storage KV does).
        for (m in FitMode.entries) assertEquals(m, FitMode.parse(m.name))
    }

    @Test
    fun parseFitMode_toleratesNullAndGarbage() {
        assertEquals(FitMode.FIT_PAGE, FitMode.parse(null))
        assertEquals(FitMode.FIT_PAGE, FitMode.parse(""))
        assertEquals(FitMode.FIT_PAGE, FitMode.parse("WHATEVER"))
    }

    @Test
    fun scrollTurns_moveOnePage_clamped() {
        // next from page N lands on N+1 top; clamped at the last page.
        assertEquals(1, scrollNextPage(0, 12))
        assertEquals(5, scrollNextPage(4, 12))
        assertEquals(11, scrollNextPage(11, 12)) // already last → stays
        // prev moves back one; clamped at the first.
        assertEquals(3, scrollPrevPage(4))
        assertEquals(0, scrollPrevPage(0)) // already first → stays
    }

    @Test
    fun scrollNext_onEmptyOrSingle_staysInBounds() {
        assertEquals(0, scrollNextPage(0, 0)) // no pages → clamp to 0, never -1
        assertEquals(0, scrollNextPage(0, 1)) // single page → stays
    }
}

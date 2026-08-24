package com.troubashare.shared.distribution

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A42 ② — the pure per-poll decision (T103: the poll is the single source of truth). Terminal is decided
 * by `state` ALONE. The load-bearing case is `finishingTail_isNotTerminal`: a `done == total` snapshot
 * that is still "running" must keep polling — deciding done by counts would stop one flatten/zip step
 * early and could miss a late failure, and it re-creates the "frozen N of N" T99 exists to prevent.
 */
class BakePollStepTest {

    @Test
    fun succeeded_isTerminal_clearsRow() {
        val s = bakePollStep(BakeProgress(state = "succeeded", done = 4, total = 4))
        assertTrue(s.done, "succeeded must be terminal")
        assertEquals(BakeStatus.Hidden, s.status)
    }

    @Test
    fun failed_isTerminal_showsServerError_notATransportGuess() {
        val s = bakePollStep(BakeProgress(state = "failed", done = 2, total = 25, error = "couldn't read the sheet music: House of the Rising Sun"))
        assertTrue(s.done, "failed must be terminal")
        assertEquals(BakeStatus.Failed("couldn't read the sheet music: House of the Rising Sun"), s.status)
    }

    @Test
    fun failed_blankError_hasAHumanFallback() {
        assertEquals(BakeStatus.Failed("The bake failed — try again"), bakePollStep(BakeProgress(state = "failed")).status)
    }

    @Test
    fun running_keepsPolling_withTheLiveLine() {
        val s = bakePollStep(BakeProgress(state = "running", done = 2, total = 25, song = "House of the Rising Sun"))
        assertTrue(!s.done, "a running bake is not terminal")
        assertEquals(BakeStatus.Baking("Baking House of the Rising Sun — 2 of 25"), s.status)
    }

    @Test
    fun finishingTail_isNotTerminal() {
        // done == total but state is STILL running (the flatten/zip tail). Terminal is decided by state,
        // never by counts — this must keep polling and show "Finishing…", not stop the loop.
        val s = bakePollStep(BakeProgress(state = "running", done = 25, total = 25, song = ""))
        assertTrue(!s.done, "done==total while still running must NOT be treated as terminal")
        assertEquals(BakeStatus.Baking("Finishing…"), s.status)
    }

    @Test
    fun nullPoll_keepsPolling_baking() {
        val s = bakePollStep(null)
        assertTrue(!s.done)
        assertEquals(BakeStatus.Baking("Baking…"), s.status)
    }
}

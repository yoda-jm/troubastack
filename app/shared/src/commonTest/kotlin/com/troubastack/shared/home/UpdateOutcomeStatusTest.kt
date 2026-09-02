package com.troubastack.shared.home

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A44 — the terminal status of a finished update run, as pure logic (the transition that used to live
 * inline in MainActivity's `onUpdate`, unreachable by any test). The load-bearing case is the first one:
 * an all-succeeded run MUST be terminal and NEVER [UpdateStatus.InFlight] — reverting to
 * `InFlight("Installing…")` here is exactly the A42① deadlock, and this test reddens on it.
 */
class UpdateOutcomeStatusTest {

    @Test
    fun allSucceeded_isTerminal_neverInFlight() {
        val s = updateOutcomeStatus(emptyList())
        // The guard with teeth: the pre-fix bug returned InFlight("Installing…") here and the row hung.
        assertTrue(s !is UpdateStatus.InFlight, "an all-succeeded run must be terminal, was InFlight: $s")
        assertEquals(UpdateStatus.UpToDate, s)
    }

    @Test
    fun oneFailure_isFailed_namingIt() {
        assertEquals(
            UpdateStatus.Failed("Couldn't update Sat @ The Anchor — try again"),
            updateOutcomeStatus(listOf("Sat @ The Anchor")),
        )
    }

    @Test
    fun partialFailure_isFailed_notOptimisticUpToDate() {
        // Some concerts installed, one didn't: result-driven — the failure wins, never an optimistic
        // UpToDate (the successes are on disk; the message survives until retry).
        assertEquals(
            UpdateStatus.Failed("Couldn't update Rainy Night — try again"),
            updateOutcomeStatus(listOf("Rainy Night")),
        )
    }

    @Test
    fun multipleFailures_countTheRest() {
        assertEquals(
            UpdateStatus.Failed("Couldn't update Alpha +2 more — try again"),
            updateOutcomeStatus(listOf("Alpha", "Bravo", "Charlie")),
        )
    }
}

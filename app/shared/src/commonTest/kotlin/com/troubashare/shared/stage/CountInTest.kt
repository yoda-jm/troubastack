package com.troubashare.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

/** A11 — count-in timing (pure). The animation itself is code-review + screenshot. */
class CountInTest {

    @Test
    fun intervalMath() {
        assertEquals(600L, countInIntervalMs(100))
        assertEquals(500L, countInIntervalMs(120))
        assertEquals(612L, countInIntervalMs(98)) // 60000/98, integer division
    }

    @Test
    fun outOfRange_ignored() {
        assertNull(countInIntervalMs(0))
        assertNull(countInIntervalMs(19))
        assertNull(countInIntervalMs(301))
        assertNull(countInIntervalMs(-1))
    }

    @Test
    fun boundsInclusive() {
        assertEquals(3000L, countInIntervalMs(20))
        assertEquals(200L, countInIntervalMs(300))
    }

    @Test
    fun eightBeats_twoBarsOf4() {
        assertEquals(8, COUNT_IN_BEATS)
        // downbeats at 0 and 4 (beat 1 of each bar); others are off-beats
        assertEquals(listOf(true, false, false, false, true, false, false, false),
            (0 until COUNT_IN_BEATS).map { isDownbeat(it) })
    }
}

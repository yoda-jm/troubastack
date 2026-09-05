package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * T147 ⟨R1⟩ — the chronometer state machine, tested WITHOUT a UI and with an injectable time source (the
 * `now` argument). No sleeping: a test that sleeps proves nothing and is flaky. The suspend case is the
 * one that matters — a tick-counter implementation must fail it.
 */
class ChronoTest {
    private val MIN = 60_000L

    @Test
    fun start_pause_resume_reset_cycle() {
        var c = Chrono()
        assertEquals(0L, c.elapsedMs(0L))
        assertFalse(c.running)

        c = c.started(1_000L)              // start at t=1s
        assertTrue(c.running)
        assertEquals(4_000L, c.elapsedMs(5_000L)) // 4s later

        c = c.paused(5_000L)               // pause at t=5s → 4s banked
        assertFalse(c.running)
        assertEquals(4_000L, c.elapsedMs(9_999L)) // time marches on; paused value is frozen

        c = c.started(10_000L)             // resume at t=10s
        assertEquals(6_000L, c.elapsedMs(12_000L)) // 4 banked + 2 live

        c = c.reset()                      // reset → 00:00, paused
        assertFalse(c.running)
        assertEquals(0L, c.elapsedMs(99_999L))
    }

    @Test
    fun double_start_does_not_restart() {
        val c = Chrono().started(1_000L)
        val again = c.started(8_000L)      // a second start must be ignored while running
        assertEquals(c, again)
        assertEquals(9_000L, again.elapsedMs(10_000L)) // still measured from t=1s, not t=8s
    }

    @Test
    fun double_pause_does_not_double_count() {
        val c = Chrono().started(0L).paused(5_000L) // 5s banked
        val again = c.paused(9_000L)                // a second pause must not fold time again
        assertEquals(c, again)
        assertEquals(5_000L, again.elapsedMs(100_000L))
    }

    // --- THE suspend case (teeth-check): a tick counter fails this ---

    @Test
    fun suspend_while_paused_does_not_advance() {
        val c = Chrono().started(0L).paused(3 * MIN) // 3 min banked, then paused
        // Ten minutes of tablet sleep pass (the clock source jumps forward) while PAUSED.
        assertEquals(3 * MIN, c.elapsedMs(3 * MIN + 10 * MIN))
    }

    @Test
    fun suspend_while_running_advances_by_exactly_the_gap() {
        val c = Chrono().started(0L) // running from t=0
        // Ten minutes of sleep while RUNNING: the value advances by exactly ten, recomputed from `now`.
        assertEquals(10 * MIN, c.elapsedMs(10 * MIN))
        // and a further five while still running
        assertEquals(15 * MIN, c.elapsedMs(15 * MIN))
    }

    @Test
    fun backward_now_never_goes_negative() {
        // A monotonic clock resets on reboot; a persisted runningSince from before it could exceed `now`.
        val c = Chrono(accumulatedMs = 2 * MIN, runningSince = 1_000_000L)
        assertEquals(2 * MIN, c.elapsedMs(0L)) // degrades to accumulated, never negative
    }

    @Test
    fun format_minutes_seconds_and_hours() {
        assertEquals("0:00", formatChrono(0L))
        assertEquals("7:04", formatChrono(7 * MIN + 4_000L)) // minutes un-padded under an hour
        assertEquals("0:09", formatChrono(9_000L))
        assertEquals("1:00:00", formatChrono(60 * MIN))      // rolls to H:MM:SS at an hour
        assertEquals("1:02:03", formatChrono(60 * MIN + 2 * MIN + 3_000L))
        assertEquals("0:00", formatChrono(-5_000L))          // negative clamped
    }
}

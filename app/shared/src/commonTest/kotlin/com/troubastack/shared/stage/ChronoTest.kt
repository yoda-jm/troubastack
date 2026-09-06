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

    // --- persistence across process death and REBOOT (T147 restore bug) — encode/decode were untested ---

    @Test
    fun paused_round_trips() {
        val c = Chrono(accumulatedMs = 5 * MIN, runningSince = null)
        val s = encodeChrono(c, nowMono = 10_000L, nowWall = 1_000_000L)
        val back = decodeChrono(s, nowMono = 99_000L, nowWall = 2_000_000L)
        assertFalse(back.running)
        assertEquals(5 * MIN, back.elapsedMs(99_000L))
    }

    @Test
    fun running_round_trips_within_the_same_boot() {
        val c = Chrono(accumulatedMs = 0L, runningSince = 1_000L)     // started at mono=1s
        val persistMono = 3 * MIN + 1_000L                           // 3 min in
        val s = encodeChrono(c, nowMono = persistMono, nowWall = 1_000_000L)
        // restore 30s of wall later; same boot ⇒ monotonic advanced by the same 30s
        val back = decodeChrono(s, nowMono = persistMono + 30_000L, nowWall = 1_030_000L)
        assertTrue(back.running)
        assertEquals(3 * MIN + 30_000L, back.elapsedMs(persistMono + 30_000L))
    }

    @Test
    fun running_survives_a_reboot_as_real_elapsed_not_garbage() {
        // THE bug: persist while running, then REBOOT — monotonic resets (now small again) while wall time
        // marched on 10 min. Real elapsed must be accumulated + live-so-far + the gap, never garbage.
        val c = Chrono(accumulatedMs = 2 * MIN, runningSince = 500_000L)
        val persistMono = 500_000L + 60_000L                         // 1 min into the live segment
        val persistWall = 1_700_000_000_000L
        val s = encodeChrono(c, nowMono = persistMono, nowWall = persistWall)
        // reboot: monotonic back near 0; wall advanced 10 min past persist
        val rebootMono = 5_000L
        val rebootWall = persistWall + 10 * MIN
        val back = decodeChrono(s, nowMono = rebootMono, nowWall = rebootWall)
        assertTrue(back.running)
        // 2m accumulated + 1m live-at-persist + 10m across the reboot = 13m — the REAL elapsed.
        assertEquals(13 * MIN, back.elapsedMs(rebootMono))
        assertEquals(13 * MIN + 7_000L, back.elapsedMs(rebootMono + 7_000L)) // and keeps counting forward
    }

    @Test
    fun legacy_mono_format_migrates_to_paused_not_garbage() {
        // TEETH: a value written by the pre-fix code ("acc:mono") must NOT be read as a wall instant — that
        // is exactly what produced the ~16h garbage. It degrades to paused at its accumulated, safe.
        val legacy = "120000:60000"                                  // acc=2min, a bare monotonic since
        val back = decodeChrono(legacy, nowMono = 50_000_000L, nowWall = 1_700_000_000_000L)
        assertFalse(back.running, "legacy running value degrades to paused, not a running garbage segment")
        assertEquals(2 * MIN, back.elapsedMs(50_000_000L))
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

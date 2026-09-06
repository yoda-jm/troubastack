package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** T147 — the analog clock hand angles are pure and testable (no Canvas). */
class ClockTest {
    private fun assertAngles(h: Int, m: Int, s: Int, hour: Float, minute: Float, second: Float) {
        val (a, b, c) = clockHandAngles(h, m, s)
        assertEquals(hour, a, 0.001f, "hour @ $h:$m:$s")
        assertEquals(minute, b, 0.001f, "minute @ $h:$m:$s")
        assertEquals(second, c, 0.001f, "second @ $h:$m:$s")
    }

    @Test fun noon_is_all_zero() = assertAngles(12, 0, 0, 0f, 0f, 0f)
    @Test fun midnight_is_all_zero() = assertAngles(0, 0, 0, 0f, 0f, 0f)

    @Test fun three_oclock_hour_points_right() = assertAngles(3, 0, 0, 90f, 0f, 0f)

    @Test fun half_past_six_hour_hand_is_halfway_to_seven() =
        // 6:30 → hour = 180 + 15 = 195 (not a whole 180), minute = 180, second = 0
        assertAngles(6, 30, 0, 195f, 180f, 0f)

    @Test fun quarter_past_nine_with_seconds() =
        // 9:15:30 → hour = 270 + 7.5 + 0.25 = 277.75, minute = 90 + 3 = 93, second = 180
        assertAngles(9, 15, 30, 277.75f, 93f, 180f)

    @Test fun twenty_four_hour_input_wraps_to_twelve_hour_face() =
        // 15:00 (3pm) renders the same as 3:00 on a 12-hour face
        assertAngles(15, 0, 0, 90f, 0f, 0f)

    // --- T157: a third style (BOTH), and the two bugs the spec flags ---

    @Test fun parse_restores_both_by_name() {
        // ⟨1⟩ the old two-way `== "DIGITAL"` compare dropped a stored BOTH to ANALOG on relaunch.
        assertEquals(ClockStyle.BOTH, ClockStyle.parse("BOTH"))
        assertEquals(ClockStyle.DIGITAL, ClockStyle.parse("DIGITAL"))
        assertEquals(ClockStyle.ANALOG, ClockStyle.parse("ANALOG"))
    }

    @Test fun parse_defaults_to_analog_for_null_or_unknown() {
        assertEquals(ClockStyle.ANALOG, ClockStyle.parse(null))
        assertEquals(ClockStyle.ANALOG, ClockStyle.parse("")) // a newer build's unknown value degrades to default, not a wrong face
        assertEquals(ClockStyle.ANALOG, ClockStyle.parse("QUARTZ"))
    }

    @Test fun both_shows_the_analog_face_even_with_no_digital_text() {
        // ⟨2⟩ on a formatter-less host (iOS/tests) clockText is "" — BOTH must still show the analog face.
        assertTrue(clockShowsAnalog(ClockStyle.BOTH))
        assertFalse(clockShowsDigital(ClockStyle.BOTH, clockTextPresent = false))
        assertTrue(clockShowsDigital(ClockStyle.BOTH, clockTextPresent = true))
    }

    @Test fun each_style_shows_the_right_faces() {
        assertTrue(clockShowsAnalog(ClockStyle.ANALOG)); assertFalse(clockShowsDigital(ClockStyle.ANALOG, true))
        assertFalse(clockShowsAnalog(ClockStyle.DIGITAL)); assertTrue(clockShowsDigital(ClockStyle.DIGITAL, true))
        assertTrue(clockShowsAnalog(ClockStyle.BOTH) && clockShowsDigital(ClockStyle.BOTH, true))
    }
}

package com.troubastack.shared.buildinfo

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** A84 §5 — the formatters. No test asserts a literal build hash: the hash is whatever git said at build time. */
class AppBuildTest {
    @Test
    fun the_line_is_hash_then_utc_minute() {
        assertEquals("TroubaStage 73dee974 · built 2026-10-09 15:07 UTC", buildLine("73dee974", "2026-10-09T15:07:00Z"))
        assertEquals("TroubaStage 73dee974-dirty · built 2026-10-09 15:07 UTC", buildLine("73dee974-dirty", "2026-10-09T15:07:42.5Z"))
    }

    @Test
    fun no_hash_is_unknown_and_a_bad_time_is_left_out() {
        assertEquals("TroubaStage unknown", buildLine(null, null))
        assertEquals("TroubaStage unknown", buildLine("  ", "2026-10-09T15:07:00Z"))
        assertEquals("TroubaStage 73dee974", buildLine("73dee974", null))
        assertEquals("TroubaStage 73dee974", buildLine("73dee974", "yesterday"))
        assertEquals(null, utcMinute("2026-10-09T15:07:00+02:00"), "only a UTC instant is labelled UTC")
    }

    @Test
    fun the_generated_build_info_is_real_or_says_unknown() {
        // Whatever this build is, the line is well-formed — a hash (optionally -dirty) and a minute, or unknown.
        assertTrue(
            Regex("""^TroubaStage ([0-9a-f]{4,40}(-dirty)?( · built \d{4}-\d{2}-\d{2} \d{2}:\d{2} UTC)?|unknown)$""").matches(appBuildLine),
            "malformed: $appBuildLine",
        )
    }

    @Test
    fun server_lines_say_checking_unreachable_or_the_version() {
        assertEquals("http://10.0.0.2:8080 · checking…", serverVersionLine("http://10.0.0.2:8080", ServerVersion.Checking))
        assertEquals("http://10.0.0.2:8080 · unreachable", serverVersionLine("http://10.0.0.2:8080", ServerVersion.Unreachable))
        assertEquals("https://band.example.org · 195ebe9a", serverVersionLine("https://band.example.org", ServerVersion.Known("195ebe9abc40b224d8b328dbdc928b5d434fb2b3")))
        assertEquals("https://band.example.org · docker", serverVersionLine("https://band.example.org", ServerVersion.Known("docker")))
    }
}

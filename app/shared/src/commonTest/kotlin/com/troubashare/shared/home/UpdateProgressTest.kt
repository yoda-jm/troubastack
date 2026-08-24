package com.troubashare.shared.home

import com.troubashare.shared.distribution.UpdateProgress
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A42 ① — the "honest bar" contract as pure logic: a determinate fraction only from a known total and
 * NEVER at/above 1f (a bar sitting at 100% reads as "hung"); an unknown total ⇒ indeterminate, never a
 * fabricated number (worse than an honest spinner); the install tail ⇒ indeterminate. Testable without
 * a device — the whole point per the spec.
 */
class UpdateProgressTest {

    @Test
    fun downloading_knownTotal_isDeterminate() {
        assertEquals(0.5f, inFlightStatus(UpdateProgress.Downloading(bytesRead = 500, contentLength = 1000)).fraction)
        assertEquals(0.1f, inFlightStatus(UpdateProgress.Downloading(100, 1000)).fraction)
    }

    @Test
    fun downloading_neverReaches100_beforeTheSwap() {
        // at or past the total, the bar caps just below full — the terminal state is UpToDate, not 1f.
        for (read in listOf(1000L, 2000L)) {
            val f = inFlightStatus(UpdateProgress.Downloading(read, 1000)).fraction
            assertTrue(f != null && f <= 0.99f, "download fraction must cap below full, was $f")
            assertTrue(f!! >= 0.98f)
        }
    }

    @Test
    fun downloading_unknownTotal_isIndeterminate_neverFabricated() {
        for (cl in listOf(0L, -1L)) {
            assertNull(
                inFlightStatus(UpdateProgress.Downloading(bytesRead = 123_456, contentLength = cl)).fraction,
                "no/unparseable Content-Length ($cl) ⇒ no fraction, ever",
            )
        }
    }

    @Test
    fun installing_isIndeterminate_labelled() {
        val s = inFlightStatus(UpdateProgress.Installing)
        assertNull(s.fraction)
        assertEquals("Installing…", s.label)
    }

    @Test
    fun label_showsBothSizes_whenKnown() {
        val s = inFlightStatus(UpdateProgress.Downloading(2_000_000, 5_000_000))
        assertTrue("2.0 MB" in s.label && "5.0 MB" in s.label, s.label)
    }

    @Test
    fun humanBytes_units() {
        assertEquals("512 B", humanBytes(512))
        assertEquals("12 KB", humanBytes(12_000))
        assertEquals("2.0 MB", humanBytes(2_000_000))
        assertEquals("45.6 MB", humanBytes(45_600_000))
    }
}

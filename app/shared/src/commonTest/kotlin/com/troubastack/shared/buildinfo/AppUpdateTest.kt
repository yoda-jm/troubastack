package com.troubastack.shared.buildinfo

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A88 §5 — the update decision, the once-a-day throttle, the dev-channel silence, the never-in-concert gate. */
class AppUpdateTest {
    private val rel = LatestRelease(
        commit = "b52113a0d26812808b2c7a22cba1c7cc4a09f007", publishedAt = "2026-10-12T19:40:11Z",
        apkUrl = "https://github.com/x/releases/download/latest/troubastage-debug.apk", apkSize = 15_753_501,
    )

    @Test
    fun same_commit_is_up_to_date_whatever_the_abbreviation() {
        assertEquals(UpdateVerdict.UpToDate, updateVerdict("b52113a", "2026-10-12T19:38:00Z", rel, null, manual = false))
        assertEquals(UpdateVerdict.UpToDate, updateVerdict("b52113a0", null, rel, null, manual = false))
    }

    @Test
    fun a_release_published_after_the_installed_build_is_available() {
        assertEquals(UpdateVerdict.Available(rel), updateVerdict("cad7478", "2026-10-12T03:29:00Z", rel, null, manual = false))
    }

    @Test
    fun a_build_newer_than_the_release_is_left_alone() {
        // a dev build made after the release (another commit): nothing to offer, even on a manual check
        assertEquals(UpdateVerdict.InstalledNewer, updateVerdict("e4508e4a-dirty", "2026-10-12T19:45:00Z", rel, null, manual = true))
    }

    @Test
    fun not_now_hides_exactly_that_release_until_a_newer_one() {
        assertEquals(UpdateVerdict.Dismissed(rel), updateVerdict("cad7478", "2026-10-12T03:29:00Z", rel, rel.commit, manual = false))
        // the person asks by hand → it is offered again
        assertEquals(UpdateVerdict.Available(rel), updateVerdict("cad7478", "2026-10-12T03:29:00Z", rel, rel.commit, manual = true))
        // a newer release → the old "Not now" no longer applies
        val newer = rel.copy(commit = "1fb37250aaaa", publishedAt = "2026-10-12T20:10:00Z")
        assertEquals(UpdateVerdict.Available(newer), updateVerdict("cad7478", "2026-10-12T03:29:00Z", newer, rel.commit, manual = false))
    }

    @Test
    fun an_unidentified_build_is_offered_the_release() {
        assertEquals(UpdateVerdict.Available(rel), updateVerdict("unknown", "2026-10-13T00:00:00Z", rel, null, manual = false))
    }

    @Test
    fun a_dev_build_never_checks_on_its_own() {
        assertFalse(shouldAutoCheck(nowMs = 10 * AUTO_CHECK_INTERVAL_MS, lastCheckMs = null, channel = "dev", autoChecks = null, inConcert = false, noteBusy = false))
        // unless the person switched automatic checks on
        assertTrue(shouldAutoCheck(10 * AUTO_CHECK_INTERVAL_MS, null, "dev", autoChecks = true, inConcert = false, noteBusy = false))
    }

    @Test
    fun a_ci_build_checks_at_most_once_a_day_and_never_in_a_concert() {
        val day = AUTO_CHECK_INTERVAL_MS
        assertTrue(shouldAutoCheck(5 * day, null, "ci", null, inConcert = false, noteBusy = false))
        assertFalse(shouldAutoCheck(5 * day, 5 * day - day / 2, "ci", null, false, false), "checked 12 h ago")
        assertTrue(shouldAutoCheck(5 * day, 4 * day, "ci", null, false, false), "checked a day ago")
        assertFalse(shouldAutoCheck(5 * day, null, "ci", null, inConcert = true, noteBusy = false), "never in a concert")
        assertFalse(shouldAutoCheck(5 * day, null, "ci", null, inConcert = false, noteBusy = true), "never while a note is drawn or sent")
        assertFalse(shouldAutoCheck(5 * day, null, "ci", autoChecks = false, inConcert = false, noteBusy = false), "switched off")
        assertTrue(shouldAutoCheck(5 * day, 9 * day, "ci", null, false, false), "a clock moved backwards does not block checks forever")
    }

    @Test
    fun iso_times_parse_to_utc_seconds() {
        assertEquals(0L, isoEpochSeconds("1970-01-01T00:00:00Z"))
        assertEquals(1_760_298_011L, isoEpochSeconds("2025-10-12T19:40:11Z"))
        assertEquals(isoEpochSeconds("2026-10-12T19:40:00Z"), isoEpochSeconds("2026-10-12T19:40Z"))
        assertEquals(null, isoEpochSeconds("2026-10-12 19:40"))
    }

    @Test
    fun the_about_line() {
        assertEquals("Latest: b52113a0 · built 2026-10-12 19:40 UTC", latestLine(UpdateVerdict.Available(rel), failed = false))
        assertEquals("Up to date", latestLine(UpdateVerdict.UpToDate, false))
        assertEquals("Couldn't check", latestLine(null, failed = true))
    }
}

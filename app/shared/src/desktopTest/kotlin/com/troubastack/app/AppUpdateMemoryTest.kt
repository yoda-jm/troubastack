package com.troubastack.app

import com.troubastack.shared.seams.PlainCodec
import com.troubastack.shared.seams.Storage
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A96 — what an update remembers across the process Android ends: "Updated ·" shows once; the marker names its build. */
class AppUpdateMemoryTest {
    private fun storage() = Storage(Files.createTempDirectory("a96").toFile(), PlainCodec)
    private val old = "TroubaStage 9c1d53e · built 2026-11-02 13:05 UTC"
    private val new = "TroubaStage 38a131a · built 2026-11-03 18:54 UTC"

    @Test
    fun updated_isShownOnce_thenNotOnTheNextLaunch() {
        // TEETH: drop the stored build's write and the second launch says "Updated ·" again.
        val s = storage()
        assertNull(updatedLineOnce(s, old, updatedSinceInstall = false), "a first install says nothing")
        assertNull(updatedLineOnce(s, old, updatedSinceInstall = false), "nor does the next launch of the same build")
        assertEquals("Updated · $new", updatedLineOnce(s, new, updatedSinceInstall = true))
        assertNull(updatedLineOnce(s, new, updatedSinceInstall = true), "once")
    }

    @Test
    fun theUpdateIntoThisBuild_saysUpdatedOnce_fromAndroidsTimes() {
        val s = storage() // nothing stored: the build that introduced the line
        assertEquals("Updated · $new", updatedLineOnce(s, new, updatedSinceInstall = true))
        assertNull(updatedLineOnce(s, new, updatedSinceInstall = true))
    }

    @Test
    fun theMarker_postsForItsBuild_only_andIsClearedOnEveryOutcome() {
        val s = storage()
        ReopenMarker(s).arm(3012)
        assertTrue(ReopenMarker(s).consume(3012), "match → post")
        assertFalse(ReopenMarker(s).consume(3012), "consumed: a second MY_PACKAGE_REPLACED posts nothing")

        ReopenMarker(s).arm(3012)
        assertFalse(ReopenMarker(s).consume(3020), "another versionCode → none")
        assertFalse(ReopenMarker(s).consume(3012), "and it is cleared by that replace too")

        assertFalse(ReopenMarker(storage()).consume(3012), "no marker → none")

        ReopenMarker(s).arm(3012)
        ReopenMarker(s).clear() // the session failed, or the person cancelled in Android's dialog
        assertFalse(ReopenMarker(s).consume(3012), "after a failure → cleared")
    }
}

package com.troubastack.shared.buildinfo

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A96 — the update's words for each phase, "Updated ·" once, and whether the reopening notification is this update's. */
class UpdatePhaseTest {
    @Test
    fun eachPhase_hasItsOwnLine() {
        // TEETH: map phase 3 to the download line (or drop the reopen clause) and this is red.
        assertEquals("Downloading the app update… 0 %", updatePhaseLine(UpdatePhase.Downloading(0f)))
        assertEquals("Downloading the app update… 42 %", updatePhaseLine(UpdatePhase.Downloading(0.42f)))
        assertEquals("Checking the download…", updatePhaseLine(UpdatePhase.Checking))
        assertEquals("Android will ask you to confirm the update.", updatePhaseLine(UpdatePhase.Confirming(reopenYourself = false)))
        assertEquals(
            "Android will ask you to confirm the update. Then open TroubaStage again.",
            updatePhaseLine(UpdatePhase.Confirming(reopenYourself = true)),
        )
        assertEquals("The update was not installed. Nothing was changed.", UPDATE_NOT_INSTALLED_LINE)
    }

    @Test
    fun updated_isSaidAfterAnUpdate_neverAfterAFreshInstall_orForTheSameBuild() {
        val running = "TroubaStage 38a131a · built 2026-11-03 18:54 UTC"
        assertEquals("Updated · $running", updatedLine("TroubaStage 9c1d53e · built 2026-11-02 13:05 UTC", running, updatedSinceInstall = true))
        assertEquals("Updated · $running", updatedLine("TroubaStage 9c1d53e · built 2026-11-02 13:05 UTC", running, updatedSinceInstall = false), "a stored, different build wins")
        assertNull(updatedLine(running, running, updatedSinceInstall = true), "the same build: nothing")
        assertNull(updatedLine(null, running, updatedSinceInstall = false), "a first install: nothing")
        // B2: the update INTO the build that introduced the line has nothing stored; Android's times tell it apart.
        assertEquals("Updated · $running", updatedLine(null, running, updatedSinceInstall = true))
    }

    @Test
    fun theReopenNotice_isForTheUpdateTheAppStarted_only() {
        assertTrue(shouldPostReopen(expectedVersionCode = 3012, runningVersionCode = 3012), "the marker's build is running: post")
        assertFalse(shouldPostReopen(expectedVersionCode = 3012, runningVersionCode = 3020), "another build (Chrome, adb after a cancel): none")
        assertFalse(shouldPostReopen(expectedVersionCode = null, runningVersionCode = 3012), "no marker: none")
    }

    @Test
    fun ourLine_comesBeforeAPromptAndroidWillShow_only() {
        assertEquals(NotifyAsk.NOT_NEEDED, notifyAsk(granted = true, askedForRelease = false, askedBefore = false, showRationale = false))
        assertEquals(NotifyAsk.ASK, notifyAsk(granted = false, askedForRelease = false, askedBefore = false, showRationale = false), "never asked")
        assertEquals(NotifyAsk.ASK, notifyAsk(granted = false, askedForRelease = false, askedBefore = true, showRationale = true), "refused once, a later release")
        assertEquals(NotifyAsk.SKIP, notifyAsk(granted = false, askedForRelease = true, askedBefore = true, showRationale = true), "at most once per release")
        // Refused for good: Android shows no prompt, so our line would lead nowhere.
        assertEquals(NotifyAsk.SKIP, notifyAsk(granted = false, askedForRelease = false, askedBefore = true, showRationale = false))
    }
}

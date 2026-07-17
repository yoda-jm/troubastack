package com.troubashare.shared.stage

import kotlinx.coroutines.launch
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** A2 — the immersive-chrome gesture split + auto-hide timeout (the load-bearing pure logic). */
class StageChromeTest {

    @Test
    fun tapThirds_edgesTurn_middleTogglesChrome() {
        val w = 900f
        assertEquals(TapAction.PREV, tapAction(10f, w, scrollMode = false))         // far left
        assertEquals(TapAction.PREV, tapAction(299f, w, scrollMode = false))        // just inside left third
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(450f, w, scrollMode = false)) // centre
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(301f, w, scrollMode = false)) // just inside middle
        assertEquals(TapAction.NEXT, tapAction(890f, w, scrollMode = false))        // far right
    }

    @Test
    fun scrollMode_anyTapTogglesChrome() {
        val w = 900f
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(10f, w, scrollMode = true))
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(450f, w, scrollMode = true))
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(890f, w, scrollMode = true))
    }

    @Test
    fun zeroWidth_doesNotTurn() {
        // Before layout the width can be 0 — never mis-fire a page turn; toggle is harmless.
        assertEquals(TapAction.TOGGLE_CHROME, tapAction(0f, 0f, scrollMode = false))
    }

    @Test
    fun autoHide_firesOnlyAfterTheDelay() = runTest {
        var hidden = false
        val job = launch { autoHideChrome(CHROME_AUTO_HIDE_MS) { hidden = true } }
        advanceTimeBy(CHROME_AUTO_HIDE_MS - 1); runCurrent()
        assertFalse(hidden, "must still be visible just before the timeout")
        advanceTimeBy(1); runCurrent()
        assertTrue(hidden, "must hide once the timeout elapses")
        job.join()
    }
}

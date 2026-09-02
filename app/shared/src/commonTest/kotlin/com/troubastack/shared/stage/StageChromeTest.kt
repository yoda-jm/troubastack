package com.troubastack.shared.stage

import kotlinx.coroutines.launch
import kotlinx.coroutines.test.advanceTimeBy
import kotlinx.coroutines.test.runCurrent
import kotlinx.coroutines.test.runTest
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** N3 — the immersive-chrome tap contract + auto-hide timeout (the load-bearing pure logic). */
class StageChromeTest {

    @Test
    fun everyTapTogglesChrome_neverTurns() {
        // N3 (supersedes A04 tap-thirds): a tap never navigates — page turns are swipe/‹ ›/pedals/keys.
        // tapAction is mode-independent by design; assert it toggles regardless of where/what mode.
        assertEquals(TapAction.TOGGLE_CHROME, tapAction())
        assertEquals(TapAction.TOGGLE_CHROME, tapAction()) // idempotent / no positional dependence
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

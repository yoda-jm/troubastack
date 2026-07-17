package com.troubashare.shared.stage

import kotlinx.coroutines.delay

/** How long the revealed Stage chrome stays up before auto-hiding (A2/Q2). */
const val CHROME_AUTO_HIDE_MS = 4000L

/** What a tap on the page area does (A2). Edge thirds turn pages (A04, verbatim); the middle third —
 *  inert before A2 — toggles the immersive chrome. */
enum class TapAction { PREV, NEXT, TOGGLE_CHROME }

/**
 * Resolve a page-area tap (A2). In page/width/two-up modes the left/right thirds turn pages (A04
 * acceptance kept exactly) and the **middle third toggles the chrome**. In SCROLL mode the column
 * owns vertical motion and there is no turn-by-tap, so ANY tap toggles the chrome. Pure so the split
 * is unit-tested (middle toggles, edges turn, no double-fire). [x] and [width] are in the same units.
 */
fun tapAction(x: Float, width: Float, scrollMode: Boolean): TapAction {
    if (scrollMode || width <= 0f) return TapAction.TOGGLE_CHROME
    val third = width / 3f
    return when {
        x < third -> TapAction.PREV
        x > third * 2f -> TapAction.NEXT
        else -> TapAction.TOGGLE_CHROME
    }
}

/**
 * Hide the chrome after [delayMs] (A2 auto-hide). Extracted so the timeout is testable with an
 * injected/virtual clock (runTest) rather than a wall-clock. Cancelling the calling coroutine (e.g.
 * the LaunchedEffect restarting when the user re-reveals or opens a drawer) cancels the pending hide.
 */
suspend fun autoHideChrome(delayMs: Long, hide: () -> Unit) {
    delay(delayMs)
    hide()
}

package com.troubashare.shared.stage

import kotlinx.coroutines.delay

/** How long the revealed Stage chrome stays up before auto-hiding (A2/Q2). */
const val CHROME_AUTO_HIDE_MS = 4000L

/** How long the N1 song-boundary cue (the title/position card ALONE, not full chrome) stays up after a
 *  cross-song advance, so continuous advance still announces "now in the next song". */
const val BOUNDARY_CUE_MS = 2000L

/** What a tap on the page area does. After N3 every tap [TOGGLE_CHROME]s; PREV/NEXT are page turns
 *  reached by swipe + ‹ › FABs + pedals/keys, never by tap. */
enum class TapAction { PREV, NEXT, TOGGLE_CHROME }

/**
 * Resolve a page-area tap (N3 — reverses A2/A04's edge-tap-turn). Living with A17 falsified the
 * "keep edge-turn for zero retraining" theory: VLL read an accidental edge-tap page turn as a
 * RENDERING GLITCH, and mid-performance a wrong page is far worse than an accidentally revealed
 * (self-hiding) chrome. In the immersive model taps mean "chrome" and swipes mean "navigate", so
 * **every tap, in every mode, toggles the chrome** — page turns come from swipe/‹ ›/pedals/keys only.
 * Kept as a pure, mode-independent function so the contract stays unit-tested (A04 tap-thirds
 * superseded per the 2026-07-17 N3 ruling).
 */
fun tapAction(): TapAction = TapAction.TOGGLE_CHROME

/**
 * Hide the chrome after [delayMs] (A2 auto-hide). Extracted so the timeout is testable with an
 * injected/virtual clock (runTest) rather than a wall-clock. Cancelling the calling coroutine (e.g.
 * the LaunchedEffect restarting when the user re-reveals or opens a drawer) cancels the pending hide.
 */
suspend fun autoHideChrome(delayMs: Long, hide: () -> Unit) {
    delay(delayMs)
    hide()
}

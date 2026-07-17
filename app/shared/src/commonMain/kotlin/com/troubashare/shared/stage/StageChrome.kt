package com.troubashare.shared.stage

import kotlinx.coroutines.delay

/** How long the revealed Stage chrome stays up before auto-hiding (A2/Q2). */
const val CHROME_AUTO_HIDE_MS = 4000L

/** How long the N1 song-boundary cue (the title/position card ALONE, not full chrome) stays up after a
 *  cross-song advance, so continuous advance still announces "now in the next song". */
const val BOUNDARY_CUE_MS = 2000L

/** How long the N7 end-of-bounds cue (a big center glyph) flashes when a turn is blocked at the
 *  concert edge, so a dead swipe reads as "you're at the start/end", not a broken turn. Shorter than
 *  the N1 card. Shares the N1 overlay layer (latest-wins). */
const val BLOCKED_TURN_CUE_MS = 900L

/** N4 — the direction-aware page-turn slide duration (page/width modes). Presentation only: the turn
 *  itself is unchanged; a new target mid-animation just wins (AnimatedContent keys off the page). */
const val PAGE_TURN_ANIM_MS = 260

/** N9 — the page-turn slide travels width/this (a SHORT shared-axis-X shift, not a full-width sweep);
 *  paired with a crossfade so a turn reads as intentional motion, not a pane sweeping across. */
const val SHARED_AXIS_SHIFT_DIVISOR = 5

/** N9 — how long to wait after the page settles before prefetching neighbours, so the prefetch decode
 *  never competes with the CURRENT page's own decode (which fires immediately on the turn). */
const val PREFETCH_SETTLE_MS = 180L

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

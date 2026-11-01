package com.troubastack.shared.stage

import kotlin.math.pow
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** WIN03 D3 — the wheel gesture rule and the cursor-hide conditions (pure, so the desktop and an Android mouse share them). */
class StageMouseTest {

    private val always: (PageTurn) -> Boolean = { true }

    /** Feed (timeMs, dx, dy) events through [wheelStep] and collect the turns. */
    private fun turns(events: List<Triple<Long, Float, Float>>, canTurn: (Long, PageTurn) -> Boolean = { _, _ -> true }): List<PageTurn> {
        var g = WheelGesture()
        val out = ArrayList<PageTurn>()
        for ((t, dx, dy) in events) {
            val step = wheelStep(g, dx, dy, t) { canTurn(t, it) }
            g = step.gesture
            step.turn?.let { out += it }
        }
        return out
    }

    private fun burst(fromMs: Long, count: Int, dy: Float, everyMs: Long = 16) =
        (0 until count).map { Triple(fromMs + it * everyMs, 0f, dy) }

    @Test
    fun oneNotch_turnsOnce() = assertEquals(listOf(PageTurn.NEXT), turns(listOf(Triple(0L, 0f, 1f))))

    @Test
    fun aLongBurst_turnsExactlyOnce() {
        // TEETH: a rule that turns on every event (or every Nth) reddens; so does one that never turns.
        assertEquals(listOf(PageTurn.NEXT), turns(burst(0, 60, 1f)))
    }

    @Test
    fun twoBursts_separatedByTheGap_turnTwice() {
        val first = burst(0, 10, 1f)
        val second = burst(first.last().first + WHEEL_GESTURE_GAP_MS + 1, 10, 1f)
        assertEquals(listOf(PageTurn.NEXT, PageTurn.NEXT), turns(first + second))
    }

    @Test
    fun aPauseShorterThanTheGap_isStillOneGesture() {
        val first = burst(0, 10, 1f)
        val second = burst(first.last().first + WHEEL_GESTURE_GAP_MS, 10, 1f) // exactly the gap: not over it
        assertEquals(listOf(PageTurn.NEXT), turns(first + second))
    }

    @Test
    fun aDecayingTail_turnsOnce() {
        // A touchpad-like run: a few large deltas, then ~1 s of decaying small ones with no gap. One page.
        val tail = (0 until 60).map { Triple(it * 16L, 0f, 12f * 0.92f.pow(it)) }
        assertEquals(listOf(PageTurn.NEXT), turns(tail))
    }

    @Test
    fun aReversalInsideABurst_turnsBack() {
        // Inertia never changes sign: the other direction is the performer's own new swipe.
        val events = burst(0, 10, 1f) + burst(170, 10, -1f)
        assertEquals(listOf(PageTurn.NEXT, PageTurn.PREV), turns(events))
    }

    @Test
    fun upAndLeft_arePrevious_rightIsNext_theLargerAxisDecides() {
        assertEquals(listOf(PageTurn.PREV), turns(listOf(Triple(0L, 0f, -2f))))
        assertEquals(listOf(PageTurn.NEXT), turns(listOf(Triple(0L, 3f, 1f))))
        assertEquals(listOf(PageTurn.PREV), turns(listOf(Triple(0L, -3f, 1f))))
    }

    @Test
    fun zeroDeltas_areIgnored_andDoNotExtendAGesture() {
        val events = listOf(Triple(0L, 0f, 1f), Triple(200L, 0f, 0f), Triple(400L, 0f, 1f))
        assertEquals(listOf(PageTurn.NEXT, PageTurn.NEXT), turns(events))
    }

    @Test
    fun aGestureThatCouldNotTurnAtItsStart_neverTurns_evenWhenItReachesTheEdge() {
        // Width mode: the burst starts mid-page (cannot turn) and scrolls to the edge; its tail must not turn.
        // A NEW gesture at the edge turns.
        val first = burst(0, 30, 1f)
        val second = burst(first.last().first + WHEEL_GESTURE_GAP_MS + 1, 5, 1f)
        val atEdgeFrom = 200L
        val turned = turns(first + second) { t, _ -> t >= atEdgeFrom }
        assertEquals(listOf(PageTurn.NEXT), turned, "only the new gesture at the edge turns")
    }

    @Test
    fun aGestureThatTurned_isMarkedTurnedToItsLastEvent_soItsTailIsSwallowed_andTheNextGestureStartsClear() {
        // TEETH: a tail that forgets it turned would scroll the page it turned to (width mode) — the caller swallows
        // only events whose gesture is marked turned.
        var g = WheelGesture()
        val marks = burst(0, 20, 1f).map { (t, dx, dy) -> wheelStep(g, dx, dy, t) { true }.also { g = it.gesture }.gesture.turned }
        assertEquals(List(20) { true }, marks, "every event of the turning gesture is marked")
        val scrolling = wheelStep(g, 0f, 1f, 1_000L) { false } // a new gesture that cannot turn (mid-page, width mode)
        assertEquals(false, scrolling.gesture.turned, "a new gesture starts unmarked")
        assertEquals(false, wheelStep(scrolling.gesture, 0f, 1f, 1_016L) { true }.gesture.turned, "and its tail stays unmarked")
    }

    @Test
    fun locked_neverTurns() = assertEquals(emptyList(), turns(burst(0, 10, 1f)) { _, _ -> false })

    @Test
    fun theCursorHides_onlyAfterTwoSecondsIdle_andNeverInNoteModeOverTheChromeOrUnderAnOverlay() {
        assertFalse(stageCursorHidden(CURSOR_HIDE_AFTER_MS - 1, noteMode = false, overChrome = false, overlayOpen = false))
        assertTrue(stageCursorHidden(CURSOR_HIDE_AFTER_MS, noteMode = false, overChrome = false, overlayOpen = false))
        assertFalse(stageCursorHidden(10_000, noteMode = true, overChrome = false, overlayOpen = false), "note mode: the crosshair stays")
        assertFalse(stageCursorHidden(10_000, noteMode = false, overChrome = true, overlayOpen = false), "over the chrome")
        assertFalse(stageCursorHidden(10_000, noteMode = false, overChrome = false, overlayOpen = true), "a dialog or the drawer")
    }
}

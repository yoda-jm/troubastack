package com.troubastack.desktop

import com.troubastack.shared.stage.PageTurn
import com.troubastack.shared.stage.WheelGesture
import com.troubastack.shared.stage.wheelStep
import java.awt.Canvas
import java.awt.event.MouseWheelEvent
import java.nio.file.Files
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/** WIN03 acceptance 6 — the debug wheel log: bounded, local, observe-only, and replayable into wheelStep. */
class WheelLogTest {
    private fun row(ms: Long, precise: Double, shift: Boolean = false, ctrl: Boolean = false) =
        WheelRow(ms, rotation = if (precise >= 1.0) 1 else 0, precise = precise, scrollType = MouseWheelEvent.WHEEL_UNIT_SCROLL, scrollAmount = 3, ctrl = ctrl, shift = shift)

    @Test
    fun keepsTheLastEvents_andTheCsvReadsBack() {
        // TEETH: a log that keeps the FIRST events loses the end of a touchpad's inertia tail, the part being measured.
        val log = WheelLog(capacity = 3)
        (1..5).forEach { log.record(row(1000L + it, 0.25 * it, ctrl = it == 5)) }
        val lines = log.csv().trim().lines()
        assertEquals(WheelRow.HEADER, lines.first())
        assertEquals(listOf(1003L, 1004L, 1005L), lines.drop(1).mapNotNull { WheelRow.parse(it) }.map { it.ms })
        assertEquals(row(1005L, 1.25, ctrl = true), WheelRow.parse(lines.last()), "a row reads back as it was recorded")
    }

    @Test
    fun save_overwritesThePreviousRun_andWritesNothingWithoutEvents() {
        val root = Files.createTempDirectory("wheel-log").toFile()
        try {
            WheelLog().save(root)
            assertFalse(root.resolve(WheelLog.FILE).exists(), "no wheel event, no file")
            WheelLog().apply { record(row(1, 1.0)); record(row(2, 1.0)) }.save(root)
            WheelLog().apply { record(row(3, 0.5)) }.save(root)
            assertEquals(listOf(WheelRow.HEADER, row(3, 0.5).csv()), root.resolve(WheelLog.FILE).readText().trim().lines(), "the last run only")
        } finally {
            root.deleteRecursively()
        }
    }

    @Test
    fun aRecordedTouchpadStream_replaysIntoWheelStep_throughComposesMapping() {
        // A two-finger swipe as a Windows touchpad sends it: small precise deltas every ~8 ms with a decaying inertia tail
        // for ~1 s, then a second swipe after a pause. Replayed as Stage sees it, it turns exactly once per swipe.
        val swipe1 = (0 until 120).map { i -> row(10_000L + i * 8, 0.6 * Math.pow(0.97, i.toDouble())) }
        val swipe2 = (0 until 60).map { i -> row(12_000L + i * 8, 0.6 * Math.pow(0.95, i.toDouble())) }
        val csv = (listOf(WheelRow.HEADER) + (swipe1 + swipe2).map { it.csv() }).joinToString("\n")
        var g = WheelGesture()
        val turns = csv.lines().mapNotNull { WheelRow.parse(it) }.mapNotNull { r ->
            val (dx, dy) = r.scrollDelta()
            wheelStep(g, dx, dy, r.ms) { true }.also { g = it.gesture }.turn
        }
        assertEquals(listOf(PageTurn.NEXT, PageTurn.NEXT), turns)
        assertEquals(0.3f to 0f, row(1, 0.3, shift = true).scrollDelta(), "Shift: the precise rotation is horizontal (Compose)")
    }

    @Test
    fun theListener_onlyObserves_theEventStillReachesItsComponentUnconsumed() {
        // TEETH: a listener that consumed the event would leave the component (Stage) without it.
        val log = WheelLog().apply { install() }
        val target = Canvas()
        var received: MouseWheelEvent? = null
        target.addMouseWheelListener { received = it }
        val e = MouseWheelEvent(target, MouseWheelEvent.MOUSE_WHEEL, 4242L, 0, 10, 10, 10, 10, 0, false, MouseWheelEvent.WHEEL_UNIT_SCROLL, 3, 1, 1.0)
        target.dispatchEvent(e)
        assertTrue(received != null && !received!!.isConsumed, "the component got the event, unconsumed")
        assertTrue(log.csv().lines().any { WheelRow.parse(it)?.ms == 4242L }, "and the log recorded it")
    }
}

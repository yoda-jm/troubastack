package com.troubastack.desktop

import java.awt.AWTEvent
import java.awt.Toolkit
import java.awt.event.InputEvent
import java.awt.event.MouseWheelEvent
import java.io.File

/**
 * WIN03 acceptance 6 — the debug wheel log: every wheel event the app received, the last [capacity], written to
 * `wheel-log.csv` in the data root when the app closes (overwriting the previous run's). VLL's Surface touchpad
 * streams become test vectors for wheelStep. Nothing leaves the machine.
 *
 * It only OBSERVES: an AWT event listener sees each event on its way and cannot consume or change it, so Stage's wheel
 * behaves exactly as without it. TEMPORARY: it goes away (or behind a switch) once the burst end is tuned on the
 * Surface — a WIN03 follow-up.
 */
class WheelLog(private val capacity: Int = 20_000) {
    private val rows = ArrayDeque<WheelRow>()

    fun record(row: WheelRow) = synchronized(rows) {
        if (rows.size == capacity) rows.removeFirst()
        rows.addLast(row)
    }

    fun csv(): String = synchronized(rows) { (listOf(WheelRow.HEADER) + rows.map { it.csv() }).joinToString("\n", postfix = "\n") }

    /** Observe every wheel event in the app, from now on. */
    fun install() {
        Toolkit.getDefaultToolkit().addAWTEventListener({ e -> if (e is MouseWheelEvent) record(WheelRow.of(e)) }, AWTEvent.MOUSE_WHEEL_EVENT_MASK)
    }

    /** Overwrite `wheel-log.csv` in [root] with this run's events. Nothing is written when no wheel event came. */
    fun save(root: File) {
        if (synchronized(rows) { rows.isEmpty() }) return
        root.mkdirs()
        File(root, FILE).writeText(csv())
    }

    /** [save] when the JVM exits normally: closing the window, including "Close TroubaStage?" → Close. (A second launch
     *  exits before the log is installed, so it never writes one.) */
    fun saveOnExit(root: File) {
        Runtime.getRuntime().addShutdownHook(Thread({ runCatching { save(root) } }, "wheel-log"))
    }

    companion object {
        const val FILE = "wheel-log.csv"
    }
}

/**
 * One wheel event as AWT gave it: its time ([MouseWheelEvent.getWhen]), the whole-notch rotation, the precise rotation
 * (a touchpad's fractions), the scroll type and amount, and Ctrl and Shift.
 */
data class WheelRow(
    val ms: Long,
    val rotation: Int,
    val precise: Double,
    val scrollType: Int,
    val scrollAmount: Int,
    val ctrl: Boolean,
    val shift: Boolean,
) {
    /**
     * What Stage receives for this event: Compose desktop's scrollDelta (dx, dy). Compose Multiplatform 1.9.0,
     * ui-desktop `ComposeSceneMediator.desktop.kt`, `ComposeScene.onMouseWheelEvent`: with Shift down the precise
     * rotation is the HORIZONTAL delta, otherwise the vertical one; the event time is `when`; the scroll type and amount
     * are not used. (On Windows a horizontal wheel or touchpad swipe reaches AWT as a Shift+wheel event.)
     */
    fun scrollDelta(): Pair<Float, Float> = if (shift) precise.toFloat() to 0f else 0f to precise.toFloat()

    fun csv(): String = "$ms,$rotation,$precise,$scrollType,$scrollAmount,${if (ctrl) 1 else 0},${if (shift) 1 else 0}"

    companion object {
        const val HEADER = "ms,rotation,precise,scrollType,scrollAmount,ctrl,shift"

        fun of(e: MouseWheelEvent) = WheelRow(
            ms = e.`when`, rotation = e.wheelRotation, precise = e.preciseWheelRotation,
            scrollType = e.scrollType, scrollAmount = e.scrollAmount,
            ctrl = e.modifiersEx and InputEvent.CTRL_DOWN_MASK != 0, shift = e.modifiersEx and InputEvent.SHIFT_DOWN_MASK != 0,
        )

        /** A row of the CSV back (a recorded stream as a test vector); null for the header or a malformed line. */
        fun parse(line: String): WheelRow? {
            val f = line.trim().split(',')
            if (f.size != 7) return null
            return runCatching {
                WheelRow(f[0].toLong(), f[1].toInt(), f[2].toDouble(), f[3].toInt(), f[4].toInt(), f[5] == "1", f[6] == "1")
            }.getOrNull()
        }
    }
}

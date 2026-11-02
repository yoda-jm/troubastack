package com.troubastack.shared.stage

import androidx.compose.runtime.Composable
import androidx.compose.runtime.staticCompositionLocalOf
import androidx.compose.ui.input.pointer.PointerIcon
import androidx.compose.ui.input.pointer.PointerKeyboardModifiers
import androidx.compose.ui.input.pointer.isCtrlPressed
import androidx.compose.ui.input.pointer.isMetaPressed
import kotlin.math.abs

/** WIN03 D3 — a wheel gesture has ended after this long without a wheel event. */
const val WHEEL_GESTURE_GAP_MS = 250L

/** One wheel gesture in progress: when its last event came, its direction (+1 next, -1 previous), and whether it has
 *  turned a page. The empty value means no gesture. */
data class WheelGesture(val lastAtMs: Long = 0L, val direction: Int = 0, val turned: Boolean = false)

/** The gesture after one wheel event, and the page turn it asks for (null: none). */
data class WheelStep(val gesture: WheelGesture, val turn: PageTurn?)

/**
 * WIN03 D3 — one wheel gesture turns at most one page (or one spread). The pure rule behind the mouse wheel and the
 * touchpad in Stage. The property (Fable): one swipe, inertia included, turns exactly one page; two deliberate swipes
 * turn two.
 *
 * A gesture is a run of wheel events: a free-spinning wheel or a touchpad sends dozens, and a touchpad may keep
 * sending decaying deltas after the fingers lift (inertia). The FIRST event of a gesture may turn; the rest never do.
 * A new gesture starts after [WHEEL_GESTURE_GAP_MS] without a wheel event, or on a reversal (inertia never changes
 * sign, so the other direction is the performer's own new swipe). Whether Windows sends inertia to this window at all
 * is to be measured on VLL's Surface (the debug wheel log); its recorded streams become test vectors, and this stays
 * the plain rule while it passes them.
 *
 * A gesture that turned keeps [WheelGesture.turned] for the rest of its events: the caller swallows that tail, so it
 * never scrolls the page it turned to.
 *
 * [dx]/[dy]: the scroll delta, down/right positive. The larger axis decides; down or right is NEXT. [canTurn] says
 * whether a new gesture in that direction may turn here (page and two-up modes: unless swipes are locked; width
 * mode: only at the edge in that direction; scroll mode and note mode: never). It is asked once per gesture, at its
 * first event, so the tail of a gesture that scrolled up to the edge never turns.
 */
fun wheelStep(g: WheelGesture, dx: Float, dy: Float, nowMs: Long, canTurn: (PageTurn) -> Boolean): WheelStep {
    val v = if (abs(dy) >= abs(dx)) dy else dx
    if (v == 0f || v.isNaN()) return WheelStep(g, null)
    val direction = if (v > 0f) 1 else -1
    val isNew = g.direction == 0 || nowMs - g.lastAtMs > WHEEL_GESTURE_GAP_MS || direction != g.direction
    if (!isNew) return WheelStep(WheelGesture(nowMs, direction, g.turned), null)
    val turn = (if (direction > 0) PageTurn.NEXT else PageTurn.PREV).takeIf(canTurn)
    return WheelStep(WheelGesture(nowMs, direction, turned = turn != null), turn)
}

/** WIN03 D3 — the pointer hides over the page after this long without moving. */
const val CURSOR_HIDE_AFTER_MS = 2000L

/**
 * WIN03 D3 — whether the pointer is hidden over the Stage page: after [CURSOR_HIDE_AFTER_MS] without movement, but
 * never in note mode (it is the pen's crosshair there), never over the chrome, and never while a dialog or the
 * drawer is open. Any movement shows it again (the caller resets [idleMs]).
 */
fun stageCursorHidden(idleMs: Long, noteMode: Boolean, overChrome: Boolean, overlayOpen: Boolean): Boolean =
    idleMs >= CURSOR_HIDE_AFTER_MS && !noteMode && !overChrome && !overlayOpen

/** WIN03 D3 — the pointer icons Stage uses where there is a mouse cursor: [hidden] over a still page, [pen] in note
 *  mode. null (Android, iOS): Stage never sets a pointer icon. */
class StagePointerIcons(val hidden: PointerIcon, val pen: PointerIcon = PointerIcon.Crosshair)

val LocalStagePointerIcons = staticCompositionLocalOf<StagePointerIcons?> { null }

/** DESK01 D3 — the modifier that turns the wheel into note mode's zoom: Ctrl (Windows, Linux, and Android's default,
 *  unchanged), Cmd on macOS, where Ctrl+scroll is the system's own accessibility zoom. */
enum class CommandModifier {
    CTRL, META;

    fun isHeld(keys: PointerKeyboardModifiers): Boolean = when (this) {
        CTRL -> keys.isCtrlPressed
        META -> keys.isMetaPressed
    }
}

val LocalCommandModifier = staticCompositionLocalOf { CommandModifier.CTRL }

/** WIN03 D3 — shows a glyph-only Stage button's label on hover. null (Android, iOS): no tooltips. */
val LocalGlyphTooltip = staticCompositionLocalOf<(@Composable (label: String, content: @Composable () -> Unit) -> Unit)?> { null }

/** WIN03 D5 — what the performer points with, for the few strings that name it ("Touch will draw" / "The mouse will
 *  draw"). */
enum class PointerKind { TOUCH, MOUSE }

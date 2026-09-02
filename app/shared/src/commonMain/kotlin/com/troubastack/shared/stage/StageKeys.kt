// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key

/** A hardware page-turn direction (A09). */
enum class PageTurn { NEXT, PREV }

/**
 * Map a hardware key to a page turn (A09) — Bluetooth pedals present as keyboards sending
 * PageUp/Down or arrows; Space is common; volume keys are the phone stand-in. Fixed map, no settings
 * (v1). Pure + shared so it's unit-tested off-device; the event capture is platform glue in the
 * entrypoints (Compose onPreviewKeyEvent for keyboards on both platforms; Android volume keys via the
 * Activity's onKeyDown). Unmapped keys → null (let the event through). Navigation itself stays clamped
 * (no wraparound) in the ViewModel.
 */
fun stageKeyAction(key: Key): PageTurn? = when (key) {
    Key.PageDown, Key.DirectionRight, Key.DirectionDown, Key.Spacebar, Key.VolumeDown -> PageTurn.NEXT
    Key.PageUp, Key.DirectionLeft, Key.DirectionUp, Key.VolumeUp -> PageTurn.PREV
    else -> null
}

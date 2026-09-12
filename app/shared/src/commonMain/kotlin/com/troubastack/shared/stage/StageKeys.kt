// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key

/** A hardware page-turn direction (A09). */
enum class PageTurn { NEXT, PREV }

/**
 * A72 ⟨D7⟩ — learned pedal bindings: each action → the native key codes taught for it. Action→codes (not
 * code→action) so adding a third action later needs no migration. Device-local, never synced (⟨D5⟩): a code
 * describes a piece of hardware in front of ONE device, and means nothing on another platform.
 */
typealias PedalBindings = Map<PageTurn, Set<Long>>

/**
 * Map a hardware key to a page turn (A09) — Bluetooth pedals present as keyboards sending
 * PageUp/Down or arrows; Space is common; volume keys are the phone stand-in. The eight built-ins are
 * fixed; [learned] (A72) ADDS bindings for other codes without ever replacing a built-in (⟨D2⟩), so a
 * working two-pedal unit cannot break because a new button was taught. Pure + shared so it's unit-tested
 * off-device; the event capture is platform glue in the entrypoints (Compose onPreviewKeyEvent for keyboards
 * on both platforms; Android volume keys via the Activity's onKeyDown). Unmapped keys → null (let the event
 * through). Navigation itself stays clamped (no wraparound) in the ViewModel.
 */
fun stageKeyAction(key: Key, learned: PedalBindings = emptyMap()): PageTurn? {
    // ⟨D2⟩: the eight built-ins ALWAYS win — a learned button never replaces a default. (This is also why the
    // pre-A72 StageKeysTest passes unchanged: the default arg is empty, so this is the old fixed map.)
    val builtin = when (key) {
        Key.PageDown, Key.DirectionRight, Key.DirectionDown, Key.Spacebar, Key.VolumeDown -> PageTurn.NEXT
        Key.PageUp, Key.DirectionLeft, Key.DirectionUp, Key.VolumeUp -> PageTurn.PREV
        else -> null
    }
    if (builtin != null) return builtin
    // Then a learned code. ⟨D3⟩ guarantees a code lives under at most one action, so first-match is total.
    val code = key.keyCode
    return PageTurn.entries.firstOrNull { code in learned[it].orEmpty() }
}

/** A72 ⟨D4⟩ — Back and Home may NEVER be learned: binding Back would trap the user inside the Stage with no
 *  way out, and Home is the OS's. Everything else (including the volume keys the app already claims) is
 *  offered; the panel still SHOWS a refused press with its reason (⟨D1⟩). */
fun isLearnableKey(key: Key): Boolean = key != Key.Back && key != Key.Home

/** The outcome of teaching one button. [takenFrom] is the action the code used to drive, if any — so the UI
 *  can say "this button was Previous" (⟨D3⟩). */
data class PedalLearnResult(val bindings: PedalBindings, val takenFrom: PageTurn?)

/**
 * A72 ⟨D3⟩ — teach [code] to [action]. Last learned wins VISIBLY: the code is removed from every OTHER
 * action first, so one key never drives two (which would let evaluation order decide behaviour invisibly).
 * Pure; refusal (⟨D4⟩) is the caller's ([isLearnableKey]) — this assumes a learnable code.
 */
fun learnPedalBinding(bindings: PedalBindings, action: PageTurn, code: Long): PedalLearnResult {
    var takenFrom: PageTurn? = null
    val next = PageTurn.entries.associateWith { a ->
        val set = bindings[a].orEmpty()
        when {
            a == action -> set + code
            code in set -> { takenFrom = a; set - code }
            else -> set
        }
    }.filterValues { it.isNotEmpty() }
    return PedalLearnResult(next, takenFrom)
}

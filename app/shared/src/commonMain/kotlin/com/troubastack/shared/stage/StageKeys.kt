// Generated proto types come from gen/ — single source of truth is proto/ (I1).
package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key

/** A hardware page-turn direction (A09). */
enum class PageTurn { NEXT, PREV }

/**
 * A72 ⟨D7⟩ / A76 ⟨D2⟩ — learned pedal bindings: each action → the set of KIND-TAGGED tokens taught for it.
 *
 * A76 makes the pedal able to speak two transports, so the token carries its kind:
 *  - a Bluetooth keyboard/HID key → `"KEY:<keyCode>"` ([keyToken])
 *  - a BLE-MIDI message           → `"MIDI:<status>,<data1>"` ([midiToken]) — status incl. channel, data1 =
 *    program / cc / note.
 *
 * ONE store, tagged, not two (Fable A76 ⟨D2⟩): a second map would be two lifetimes for one concept — "Forget
 * learned buttons" would become two operations that can diverge. Action→tokens (not token→action) so a third
 * action later needs no migration. Device-local, never synced (A72 ⟨D5⟩): a token describes hardware in front
 * of ONE device.
 */
typealias PedalBindings = Map<PageTurn, Set<String>>

/** The tagged token for a keyboard key code. */
fun keyToken(keyCode: Long): String = "KEY:$keyCode"

/**
 * The tagged token for a MIDI message. A76 ⟨D3⟩: keyed on the message TYPE (`status and 0xF0`) + `data1`,
 * deliberately dropping the **channel** (the low status nibble). VLL's pedal steps "registers" by changing
 * the MIDI channel it transmits on, so a full-status match would silently stop working after a register step
 * — the exact failure ⟨D3⟩ forbids. Keying on the stable part (type + number, channel-agnostic) makes a
 * learned switch turn pages in every register. The value byte is also dropped (a press and its release share
 * type+data1; only presses reach here). Both learn and match go through this, so they always agree.
 */
fun midiToken(status: Int, data1: Int): String = "MIDI:${status and 0xF0},$data1"

/**
 * Map a hardware key to a page turn (A09) — Bluetooth pedals present as keyboards sending PageUp/Down or
 * arrows; Space is common; volume keys are the phone stand-in. The eight built-ins are fixed; [learned] ADDS
 * bindings for other keys without ever replacing a built-in (A72 ⟨D2⟩), so a working two-pedal unit cannot
 * break because a new button was taught. Pure + shared so it's unit-tested off-device; the event capture is
 * platform glue in the entrypoints. Unmapped keys → null (let the event through).
 */
fun stageKeyAction(key: Key, learned: PedalBindings = emptyMap()): PageTurn? {
    val builtin = when (key) {
        Key.PageDown, Key.DirectionRight, Key.DirectionDown, Key.Spacebar, Key.VolumeDown -> PageTurn.NEXT
        Key.PageUp, Key.DirectionLeft, Key.DirectionUp, Key.VolumeUp -> PageTurn.PREV
        else -> null
    }
    if (builtin != null) return builtin
    val tok = keyToken(key.keyCode)
    return PageTurn.entries.firstOrNull { tok in learned[it].orEmpty() }
}

/**
 * A76 — map a learned BLE-MIDI message (status, data1) to a page turn. There are NO built-in MIDI messages
 * (unlike keys): a pedal's MIDI vocabulary is arbitrary, so only what was taught matches. Unmatched → null.
 */
fun stageMidiAction(status: Int, data1: Int, learned: PedalBindings = emptyMap()): PageTurn? =
    tokenAction(midiToken(status, data1), learned)

/** A76 — match a raw learned token (`KEY:` or `MIDI:`) against the bindings. The Stage's MIDI path uses this
 *  directly with the token delivered by the pedal; [stageKeyAction]/[stageMidiAction] are typed conveniences. */
fun tokenAction(token: String, learned: PedalBindings): PageTurn? =
    PageTurn.entries.firstOrNull { token in learned[it].orEmpty() }

/** A72 ⟨D4⟩ — Back and Home may NEVER be learned (binding Back would trap the user in the Stage; Home is the
 *  OS's). Everything else is offered; the panel still SHOWS a refused press with its reason (⟨D1⟩). MIDI
 *  messages have no such trap, so all are learnable. */
fun isLearnableKey(key: Key): Boolean = key != Key.Back && key != Key.Home

/** The outcome of teaching one token. [takenFrom] is the action the token used to drive, if any — so the UI
 *  can say "this button was Previous" (A72 ⟨D3⟩). */
data class PedalLearnResult(val bindings: PedalBindings, val takenFrom: PageTurn?)

/**
 * A72 ⟨D3⟩ — teach [token] (a KEY: or MIDI: tag) to [action]. Last learned wins VISIBLY: the token is removed
 * from every OTHER action first, so one control never drives two (which would let evaluation order decide
 * behaviour invisibly). Pure; refusal (⟨D4⟩) is the caller's — this assumes a learnable token.
 */
fun learnPedalBinding(bindings: PedalBindings, action: PageTurn, token: String): PedalLearnResult {
    var takenFrom: PageTurn? = null
    val next = PageTurn.entries.associateWith { a ->
        val set = bindings[a].orEmpty()
        when {
            a == action -> set + token
            token in set -> { takenFrom = a; set - token }
            else -> set
        }
    }.filterValues { it.isNotEmpty() }
    return PedalLearnResult(next, takenFrom)
}

/** A72 ⟨D5⟩ — serialise device-local bindings for `storage.putSecret`. Tokens are joined by `|` (a MIDI token
 *  contains a comma, so the list separator must not be one); actions by `;`. E.g. `NEXT=KEY:1001|MIDI:192,3`. */
fun encodePedalBindings(bindings: PedalBindings): String =
    PageTurn.entries.mapNotNull { a ->
        bindings[a]?.takeIf { it.isNotEmpty() }?.let { "${a.name}=${it.sorted().joinToString("|")}" }
    }.joinToString(";")

/** Inverse of [encodePedalBindings]; tolerant of null/blank/garbage (returns what it can parse). */
fun parsePedalBindings(s: String?): PedalBindings {
    if (s.isNullOrBlank()) return emptyMap()
    return s.split(";").mapNotNull { part ->
        val eq = part.indexOf('=')
        if (eq <= 0) return@mapNotNull null
        val action = PageTurn.entries.firstOrNull { it.name == part.substring(0, eq) } ?: return@mapNotNull null
        val set = part.substring(eq + 1).split("|").map { it.trim() }.filter { it.isNotEmpty() }.toSet()
        if (set.isEmpty()) null else action to set
    }.toMap()
}

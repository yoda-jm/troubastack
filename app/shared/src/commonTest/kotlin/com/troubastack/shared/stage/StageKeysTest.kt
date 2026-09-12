package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A09 — the hardware key → page-turn map (pedals/keyboards/volume). */
class StageKeysTest {

    @Test
    fun nextKeys() {
        for (k in listOf(Key.PageDown, Key.DirectionRight, Key.DirectionDown, Key.Spacebar, Key.VolumeDown)) {
            assertEquals(PageTurn.NEXT, stageKeyAction(k), "expected NEXT for $k")
        }
    }

    @Test
    fun prevKeys() {
        for (k in listOf(Key.PageUp, Key.DirectionLeft, Key.DirectionUp, Key.VolumeUp)) {
            assertEquals(PageTurn.PREV, stageKeyAction(k), "expected PREV for $k")
        }
    }

    @Test
    fun unmappedKeys_areNull() {
        for (k in listOf(Key.A, Key.Enter, Key.Escape, Key.Back, Key.MediaPlay)) {
            assertNull(stageKeyAction(k), "expected null for $k")
        }
    }

    // ── A72: learned pedal bindings ────────────────────────────────────────────────────────────────

    @Test
    fun learnedCode_mapsToItsAction() {
        val learned = mapOf(PageTurn.NEXT to setOf(1001L))
        assertEquals(PageTurn.NEXT, stageKeyAction(Key(1001L), learned))
    }

    @Test
    fun defaults_alwaysWin_evenIfLearnedTriesToReplaceThem() {
        // ⟨D2⟩: a built-in is never replaced by a learned binding — a working two-pedal unit can't break.
        val learned = mapOf(PageTurn.PREV to setOf(Key.PageDown.keyCode))
        assertEquals(PageTurn.NEXT, stageKeyAction(Key.PageDown, learned))
    }

    @Test
    fun unknownCode_isNull_evenWithLearnedPresent() {
        assertNull(stageKeyAction(Key(9999L), mapOf(PageTurn.NEXT to setOf(1001L))))
    }

    @Test
    fun learn_lastLearnedWins_removesTheOldBinding() {
        // ⟨D3⟩: teaching a code already on NEXT to PREV moves it — one key never drives two.
        val r = learnPedalBinding(mapOf(PageTurn.NEXT to setOf(1001L)), PageTurn.PREV, 1001L)
        assertEquals(mapOf(PageTurn.PREV to setOf(1001L)), r.bindings)
        assertEquals(PageTurn.NEXT, r.takenFrom)
        assertEquals(PageTurn.PREV, stageKeyAction(Key(1001L), r.bindings))
    }

    @Test
    fun learn_sameAction_addsWithoutTakenFrom() {
        val r = learnPedalBinding(mapOf(PageTurn.NEXT to setOf(1001L)), PageTurn.NEXT, 1002L)
        assertEquals(setOf(1001L, 1002L), r.bindings[PageTurn.NEXT])
        assertNull(r.takenFrom)
    }

    @Test
    fun learnable_refusesBackAndHome_allowsTheRest() {
        assertFalse(isLearnableKey(Key.Back))
        assertFalse(isLearnableKey(Key.Home))
        assertTrue(isLearnableKey(Key.VolumeUp)) // ⟨D4⟩: volume stays learnable
        assertTrue(isLearnableKey(Key(1001L)))
    }
}

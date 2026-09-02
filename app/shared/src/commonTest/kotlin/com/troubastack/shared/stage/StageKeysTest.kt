package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertNull

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
}

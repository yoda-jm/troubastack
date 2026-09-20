package com.troubastack.shared.stage

import androidx.compose.ui.input.key.Key
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/** A09 / A72 / A76 — the hardware page-turn map (keys + learned KEY/MIDI tokens) and the learn/persist logic. */
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

    // ── A72: learned keyboard tokens ────────────────────────────────────────────────────────────────

    @Test
    fun learnedKey_mapsToItsAction() {
        val learned = mapOf(PageTurn.NEXT to setOf(keyToken(1001L)))
        assertEquals(PageTurn.NEXT, stageKeyAction(Key(1001L), learned))
    }

    @Test
    fun defaults_alwaysWin_evenIfLearnedTriesToReplaceThem() {
        // A72 ⟨D2⟩: a built-in is never replaced by a learned binding — a working two-pedal unit can't break.
        val learned = mapOf(PageTurn.PREV to setOf(keyToken(Key.PageDown.keyCode)))
        assertEquals(PageTurn.NEXT, stageKeyAction(Key.PageDown, learned))
    }

    @Test
    fun unknownKey_isNull_evenWithLearnedPresent() {
        assertNull(stageKeyAction(Key(9999L), mapOf(PageTurn.NEXT to setOf(keyToken(1001L)))))
    }

    // ── A76: learned MIDI tokens ────────────────────────────────────────────────────────────────────

    @Test
    fun learnedMidi_mapsToItsAction() {
        // Program Change 3 on channel 1 (status 0xC0=192) taught to NEXT.
        val learned = mapOf(PageTurn.NEXT to setOf(midiToken(192, 3)))
        assertEquals(PageTurn.NEXT, stageMidiAction(192, 3, learned))
        assertNull(stageMidiAction(192, 4, learned), "a different program must not match")
        assertNull(stageMidiAction(176, 3, learned), "a different message TYPE must not match")
    }

    @Test
    fun learnedMidi_isChannelTolerant_soRegisterStepsDontBreakIt() {
        // A76 ⟨D3⟩: stepping the pedal's register changes the MIDI CHANNEL (status low nibble). A switch learned
        // on channel 1 must still turn pages on channels 2..16 — else the binding silently dies mid-set.
        val learned = mapOf(PageTurn.NEXT to setOf(midiToken(0xC0, 3))) // PC 3, channel 1
        assertEquals(PageTurn.NEXT, stageMidiAction(0xC0, 3, learned))  // same channel
        assertEquals(PageTurn.NEXT, stageMidiAction(0xC1, 3, learned))  // channel 2 (register stepped)
        assertEquals(PageTurn.NEXT, stageMidiAction(0xCF, 3, learned))  // channel 16
        assertNull(stageMidiAction(0xB0, 3, learned), "CC ≠ PC even on the same number")
        assertNull(stageMidiAction(0xC0, 4, learned), "a different program still must not match")
    }

    @Test
    fun midi_andKey_liveInTheOneStore() {
        // A76 ⟨D2⟩: one store, tagged. A key and a MIDI message on the same action both resolve.
        val learned = mapOf(PageTurn.NEXT to setOf(keyToken(1001L), midiToken(192, 3)))
        assertEquals(PageTurn.NEXT, stageKeyAction(Key(1001L), learned))
        assertEquals(PageTurn.NEXT, stageMidiAction(192, 3, learned))
    }

    // ── learn (A72 ⟨D3⟩), token-agnostic ────────────────────────────────────────────────────────────

    @Test
    fun learn_lastLearnedWins_movesTheToken() {
        val r = learnPedalBinding(mapOf(PageTurn.NEXT to setOf(midiToken(192, 3))), PageTurn.PREV, midiToken(192, 3))
        assertEquals(mapOf(PageTurn.PREV to setOf(midiToken(192, 3))), r.bindings)
        assertEquals(PageTurn.NEXT, r.takenFrom)
        assertEquals(PageTurn.PREV, stageMidiAction(192, 3, r.bindings))
    }

    @Test
    fun learn_sameAction_addsWithoutTakenFrom() {
        val r = learnPedalBinding(mapOf(PageTurn.NEXT to setOf(keyToken(1001L))), PageTurn.NEXT, midiToken(176, 64))
        assertEquals(setOf(keyToken(1001L), midiToken(176, 64)), r.bindings[PageTurn.NEXT])
        assertNull(r.takenFrom)
    }

    @Test
    fun learnable_refusesBackAndHome_allowsTheRest() {
        assertFalse(isLearnableKey(Key.Back))
        assertFalse(isLearnableKey(Key.Home))
        assertTrue(isLearnableKey(Key.VolumeUp))
        assertTrue(isLearnableKey(Key(1001L)))
    }

    @Test
    fun bindings_roundTripThroughStorage_keyAndMidi() {
        val b = mapOf(
            PageTurn.NEXT to setOf(keyToken(1001L), midiToken(192, 3)),
            PageTurn.PREV to setOf(midiToken(176, 64)),
        )
        assertEquals(b, parsePedalBindings(encodePedalBindings(b)))
        assertEquals(emptyMap(), parsePedalBindings(null))
        assertEquals(emptyMap(), parsePedalBindings(""))
        assertEquals(emptyMap(), parsePedalBindings("garbage;=;NEXT=")) // tolerant of junk
    }
}

package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/** A76 — the pure MIDI-frame → presses parser. */
class StageMidiTest {
    private fun bytes(vararg v: Int) = ByteArray(v.size) { v[it].toByte() }

    @Test
    fun noteOn_isAPress_noteOffAndZeroVelocityAreNot() {
        assertEquals(listOf(MidiPress(0x90, 60)), parseMidiPresses(bytes(0x90, 60, 100)))
        assertEquals(emptyList(), parseMidiPresses(bytes(0x90, 60, 0)))   // velocity 0 = release
        assertEquals(emptyList(), parseMidiPresses(bytes(0x80, 60, 64)))  // Note Off
    }

    @Test
    fun controlChange_pressOnNonZero_releaseOnZero() {
        assertEquals(listOf(MidiPress(0xB0, 64)), parseMidiPresses(bytes(0xB0, 64, 127)))
        assertEquals(emptyList(), parseMidiPresses(bytes(0xB0, 64, 0))) // momentary switch release
    }

    @Test
    fun programChange_isAlwaysAPress() {
        assertEquals(listOf(MidiPress(0xC0, 3)), parseMidiPresses(bytes(0xC0, 3)))
    }

    @Test
    fun runningStatus_yieldsMultiplePresses() {
        // Note On status once, then two note/velocity pairs.
        assertEquals(
            listOf(MidiPress(0x90, 60), MidiPress(0x90, 62)),
            parseMidiPresses(bytes(0x90, 60, 100, 62, 100)),
        )
    }

    @Test
    fun channelIsKeptInStatus() {
        // ch10 Note On (0x99) → status 0x99, distinct from ch1 (0x90).
        assertEquals(listOf(MidiPress(0x99, 36)), parseMidiPresses(bytes(0x99, 36, 100)))
    }

    @Test
    fun systemRealtimeInterleaved_isIgnored() {
        // 0xF8 (timing clock) between the status and its data must not corrupt the parse.
        assertEquals(listOf(MidiPress(0xC0, 3)), parseMidiPresses(bytes(0xF8, 0xC0, 3, 0xFE)))
    }

    @Test
    fun truncatedTail_doesNotThrow_andParsesWhatItCan() {
        val r = parseMidiPresses(bytes(0xC0, 3, 0x90, 60)) // a full PC then a truncated Note On
        assertTrue(r.contains(MidiPress(0xC0, 3)))
        assertEquals(1, r.size)
    }

    @Test
    fun offsetAndLen_areHonoured() {
        val buf = bytes(0xFF, 0xC0, 3, 0xFF)
        assertEquals(listOf(MidiPress(0xC0, 3)), parseMidiPresses(buf, off = 1, len = 2))
    }
}

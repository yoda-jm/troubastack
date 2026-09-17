package com.troubastack.shared.stage

/** A76 ⟨D1⟩ — the BLE-MIDI link state the Learn panel must show, so "nothing received yet" is a diagnostic
 *  (a disconnected pedal and a mute one look identical without it). Shared so the panel needn't know Android. */
enum class PedalLinkState { NOT_CONNECTED, SCANNING, CONNECTED }

/** A76 — one delivered MIDI press, handed to the UI. [seq] rises per press so a Compose effect can tell a NEW
 *  press from a recomposition; [token] is the [midiToken] to learn; [label] is the human line ("MIDI: PC 3"). */
data class MidiSignal(val seq: Long, val token: String, val label: String)

/** A76 — a MIDI "press" worth binding: the (status, data1) that identifies WHICH control fired. `status`
 *  includes the channel (low nibble); `data1` is the note / cc / program number. The value byte is dropped
 *  on purpose — a press and its release share (status, data1), and only presses reach here (see below). */
data class MidiPress(val status: Int, val data1: Int)

/** A76 ⟨D1⟩ — a human line for a MIDI press, for the panel diagnostic ("MIDI: PC 3 (ch1)"). */
fun midiLabel(status: Int, data1: Int): String {
    val ch = (status and 0x0F) + 1
    return when (status and 0xF0) {
        0x90 -> "MIDI: Note $data1 (ch$ch)"
        0xB0 -> "MIDI: CC $data1 (ch$ch)"
        0xC0 -> "MIDI: PC $data1 (ch$ch)"
        else -> "MIDI: $status,$data1"
    }
}

/**
 * A76 — parse the channel-voice MIDI in [bytes] into the PRESS events worth binding, pure + testable
 * off-device (the A72/A75 pattern; BLE plumbing stays in the host). Android's BLE-MIDI device delivers a
 * de-framed standard MIDI byte stream to a MidiReceiver; this turns one such frame into presses.
 *
 * A footswitch emits **Note On**, **Control Change**, or **Program Change**. We forward only PRESSES —
 * Note On velocity>0, CC value>0, Program Change always — so a press and its release do not both bind, and
 * the releases (Note Off, Note On velocity 0, CC value 0) are dropped. **Running status** is honoured.
 * System real-time bytes (0xF8..0xFF) are skipped; system-common resets running status; a truncated tail is
 * ignored rather than throwing (a value at every edge, like the rest of the stage core).
 */
fun parseMidiPresses(bytes: ByteArray, off: Int = 0, len: Int = bytes.size): List<MidiPress> {
    val out = ArrayList<MidiPress>()
    val end = (off + len).coerceAtMost(bytes.size)
    var i = off.coerceAtLeast(0)
    var status = 0 // running status (0 = none)
    while (i < end) {
        val b = bytes[i].toInt() and 0xFF
        if (b >= 0xF8) { i++; continue }               // system real-time — no data
        if (b >= 0x80) {                               // a status byte
            if (b in 0xF0..0xF7) { status = 0; i++; continue } // system common — not a footswitch press
            status = b; i++                            // channel-voice status; data follows
        } else if (status == 0) { i++; continue }      // stray data byte, no running status → resync
        when (status and 0xF0) {
            0x90 -> { // Note On
                if (i + 1 >= end) return out
                val note = bytes[i].toInt() and 0x7F; val vel = bytes[i + 1].toInt() and 0x7F; i += 2
                if (vel > 0) out.add(MidiPress(status, note))
            }
            0xB0 -> { // Control Change
                if (i + 1 >= end) return out
                val cc = bytes[i].toInt() and 0x7F; val v = bytes[i + 1].toInt() and 0x7F; i += 2
                if (v > 0) out.add(MidiPress(status, cc))
            }
            0xC0 -> { // Program Change — 1 data byte, always a press
                if (i >= end) return out
                out.add(MidiPress(status, bytes[i].toInt() and 0x7F)); i += 1
            }
            0x80 -> { if (i + 1 >= end) return out; i += 2 } // Note Off — release, drop
            0xA0, 0xE0 -> { if (i + 1 >= end) return out; i += 2 } // poly aftertouch / pitch bend — 2 data
            0xD0 -> { if (i >= end) return out; i += 1 }          // channel pressure — 1 data
            else -> i++
        }
    }
    return out
}

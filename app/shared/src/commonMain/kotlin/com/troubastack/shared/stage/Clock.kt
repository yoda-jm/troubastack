package com.troubastack.shared.stage

/** T147 — how the bottom-right Stage clock is drawn. Analog is the DEFAULT (VLL). */
enum class ClockStyle { ANALOG, DIGITAL }

/**
 * T147 — the three hand angles (degrees clockwise from 12 o'clock) for an analog clock face at local
 * time [h]:[m]:[s]. Pure + testable; the Compose Canvas turns these into strokes. Hands move smoothly:
 * the hour hand advances with the minutes and the minute hand with the seconds, so the face never reads
 * a whole-number lie (e.g. 6:30 puts the hour hand halfway to 7). [h] is 0-23; only h % 12 matters.
 */
fun clockHandAngles(h: Int, m: Int, s: Int): Triple<Float, Float, Float> {
    val hour = ((h % 12) * 30f) + (m * 0.5f) + (s * (0.5f / 60f))
    val minute = (m * 6f) + (s * 0.1f)
    val second = s * 6f
    return Triple(hour % 360f, minute % 360f, second % 360f)
}

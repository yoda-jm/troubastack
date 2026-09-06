package com.troubastack.shared.stage

/** T147/T157 — how the bottom-right Stage clock is drawn. Analog is the DEFAULT (VLL); BOTH stacks the
 *  digital time UNDER the analog face (T157). */
enum class ClockStyle {
    ANALOG, DIGITAL, BOTH;

    companion object {
        /** T157 — restore a persisted style by NAME over the entries; null/unknown ⇒ the ANALOG default.
         *  (The old two-way `== "DIGITAL"` compare dropped a stored BOTH back to ANALOG; matching by name
         *  also degrades a newer build's unknown value to the default instead of to a wrong face.) */
        fun parse(raw: String?): ClockStyle = entries.firstOrNull { it.name == raw } ?: ANALOG
    }
}

/** T157 — the analog face is shown for ANALOG and BOTH. */
fun clockShowsAnalog(style: ClockStyle): Boolean = style == ClockStyle.ANALOG || style == ClockStyle.BOTH

/** T157 — the digital line is shown for DIGITAL and BOTH, but only when the host supplied a formatted time
 *  ([clockTextPresent]); on a formatter-less host (iOS/tests) BOTH still shows the analog face alone. */
fun clockShowsDigital(style: ClockStyle, clockTextPresent: Boolean): Boolean =
    (style == ClockStyle.DIGITAL || style == ClockStyle.BOTH) && clockTextPresent

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

package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * A34 — the properties the shared vectors can't express: the *sequence* (that a beat is VISIBLE — 8
 * on→off transitions, downbeats emphasised), no accumulated drift, and the tempo guard. This is the
 * regression the A11 count-in lacked: it only tested the interval number, never that anything blinks,
 * so an "always on" render shipped. A flip to "always lit" here must redden this file.
 */
class BeatPhaseTest {

    // A67: the count-in length is metre-aware; 4/4 (the default) is two bars = 8 units. Was COUNT_IN_BEATS.
    private val countIn = countInUnits(DEFAULT_GROUPS)

    @Test
    fun stageBeat_defaultsToCountIn_notInfinite() {
        // A40: the default must be the two-bar count-in, not ∞. A default sitting wrong is invisible
        // without this assertion — flipping `continuous` back to true reddens exactly this.
        assertFalse(StageBeat().continuous, "the metronome must count in and stop by default; ∞ is opt-in")
    }

    @Test
    fun toggle_usesTheCountInBeatCount() {
        // A40 (Fable): the flag default alone would pass even if toggle() ignored it — pin the WIRING
        // too. A default tap arms a count-in; with ∞ on it arms the continuous count.
        val b = StageBeat()
        b.toggle(120)
        assertEquals(countIn, b.beats, "a default toggle must arm a count-in, not an endless run")
        b.stop(); b.continuous = true; b.toggle(120)
        assertEquals(CONTINUOUS_BEATS, b.beats, "with the opt-in on, toggle must arm the continuous count")
    }

    @Test
    fun intervalMs_isDouble_noTruncation() {
        // the original bug: 60000/90 truncated to 666 instead of 666.67 (drifts a bar every ~40 s).
        assertEquals(666.6666, intervalMs(90), 1e-3)
        assertEquals(500.0, intervalMs(120), 0.0)
    }

    @Test
    fun tempoRange_guardsAbsurdRates() {
        assertNull(tempoIntervalMs(0))
        assertNull(tempoIntervalMs(19))
        assertNull(tempoIntervalMs(301))
        assertNull(tempoIntervalMs(-1))
        assertEquals(3000.0, tempoIntervalMs(20))   // inclusive low
        assertEquals(200.0, tempoIntervalMs(300))   // inclusive high
    }

    @Test
    fun eightBeats_isTwoBarsOf4_downbeatsAt0and4() {
        assertEquals(8, countIn)
        assertEquals(
            listOf(true, false, false, false, true, false, false, false),
            (0 until countIn).map { it % unitsPerBar(DEFAULT_GROUPS) == 0 },
        )
    }

    /** THE criterion that matters: a 120-bpm count-in produces exactly 8 lit→unlit transitions. */
    @Test
    fun countIn_120bpm_hasExactly8VisibleOnsets() {
        val interval = 500.0
        var prevLit = false
        var onsets = 0
        val litBeats = mutableSetOf<Int>()
        var t = 0.0
        val end = countIn * interval + 50
        while (t <= end) {
            val p = beatPhase(t, interval, countIn)
            if (p.lit && !prevLit) { onsets++; litBeats.add(p.beatIndex) }
            prevLit = p.lit
            t += 1.0
        }
        assertEquals(8, onsets, "a count-in must produce exactly 8 visible on→off events (not 'always on')")
        assertEquals((0 until 8).toSet(), litBeats, "every one of the 8 beats lights once")
    }

    @Test
    fun litWindow_isATransient_atMost35pctOfInterval() {
        for (bpm in listOf(40, 60, 90, 120, 180, 300)) {
            val i = intervalMs(bpm)
            assertTrue(litWindowMs(i) <= i * 0.35 + 1e-9, "lit window must stay ≤ 35% of the interval @ $bpm bpm")
        }
    }

    @Test
    fun noDrift_beatIndexIsComputedNotAccumulated() {
        val interval = 500.0
        // beat 200's onset is start + 200×interval, to the ms — computed, so no accumulated error.
        assertEquals(199, beatPhase(200 * interval - 1, interval, CONTINUOUS_BEATS).beatIndex)
        assertEquals(200, beatPhase(200 * interval, interval, CONTINUOUS_BEATS).beatIndex)
        assertEquals(200, beatPhase(200 * interval + 5, interval, CONTINUOUS_BEATS).beatIndex) // within ±5 ms
        // the 90-bpm truncation guard: a 666-ms truncation would put elapsed 1333 on beat 2.
        assertEquals(1, beatPhase(1333.0, intervalMs(90), countIn).beatIndex)
    }

    @Test
    fun countInEnds_noLitPastTheLastBeat() {
        assertEquals(8, beatPhase(countIn * 500.0, 500.0, countIn).beatIndex)
        assertFalse(beatPhase(countIn * 500.0, 500.0, countIn).lit)
        assertFalse(beatPhase((countIn + 3) * 500.0, 500.0, countIn).emphasis)
    }

    /** The edge-frame envelope: full at the beat, eased to nothing before the next, dark between. */
    @Test
    fun beatFrame_pulsesThenGoesDark() {
        val interval = 500.0
        val atBeat = beatFrame(0.0, interval, countIn)!!
        assertTrue(atBeat.env > 0.99f, "full envelope at the beat")
        assertEquals(0, atBeat.tier, "unit 0 is the bar (tier 0)")
        // decay(500) = min(220, 375) = 220 → dark once msSinceBeat ≥ 220
        assertNull(beatFrame(220.0, interval, countIn), "dark between pulses")
        assertNull(beatFrame(499.0, interval, countIn), "still dark just before the next beat")
        assertEquals(1, beatFrame(500.0, interval, countIn)!!.tier, "unit 1 is a felt pulse (tier 1) in 4/4")
        assertNull(beatFrame(-1.0, interval, countIn), "dark before start")
        assertNull(beatFrame(countIn * interval, interval, countIn), "dark after the count")
    }
}

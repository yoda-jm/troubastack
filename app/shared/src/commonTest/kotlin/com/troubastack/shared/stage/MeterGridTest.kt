package com.troubastack.shared.stage

import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * A35 — the metric grid the shared vectors can't fully pin: the no-regression case for every pre-T86
 * bundle (unset metre ⇒ 4/4, no grey ever), the 3/4 and 6/8 tier layouts VLL will see on the stage,
 * the tier-2 strobe mute at 130 ms/unit, and the lenient parser that must never throw on a typo.
 */
class MeterGridTest {

    /** The tiers of a full bar, unit 0..unitsPerBar-1, for a metre string. */
    private fun barTiers(meter: String): List<Int> {
        val g = meterGroups(meter)
        return (0 until unitsPerBar(g)).map { tierOf(it, g) }
    }

    @Test
    fun noMeter_beatsExactlyAsBefore_eightUnitCountIn_amberEvery4_noGreyEver() {
        // The whole no-regression promise: anything baked before T86 (meter "") keeps A34's behaviour.
        val g = meterGroups("")
        assertEquals(DEFAULT_GROUPS, g, "an unset metre resolves to 4/4")
        assertEquals(8, countInUnits(g), "the count-in is still two bars of four = 8 units")
        val interval = intervalMs(120)
        val tiers = (0 until countInUnits(g)).map { beatPhase(it * interval, interval, countInUnits(g), g).tier }
        assertEquals(listOf(0, 1, 1, 1, 0, 1, 1, 1), tiers, "amber on 1 & 5, felt-pulse elsewhere")
        assertFalse(tiers.any { it == 2 }, "a 4/4 bundle must NEVER paint a grey (tier-2) unit")
    }

    @Test
    fun threeFour_countsSix_amberOnOne() {
        val g = meterGroups("3/4")
        assertEquals(listOf(1, 1, 1), g)
        assertEquals(6, countInUnits(g), "3/4 counts in 6 units (two bars)")
        assertEquals(listOf(0, 1, 1), barTiers("3/4"), "amber on 1, felt pulse on 2 & 3, no grey")
        assertFalse(barTiers("3/4").any { it == 2 })
    }

    @Test
    fun sixEight_countsTwelve_amberOn1_aquaOn4_greyOn2356() {
        val g = meterGroups("6/8")
        assertEquals(listOf(3, 3), g)
        assertEquals(12, countInUnits(g), "6/8 counts in 12 units = 4 felt pulses over two bars")
        // 1-based: amber on 1, aqua on 4, grey on 2/3/5/6.
        assertEquals(listOf(0, 2, 2, 1, 2, 2), barTiers("6/8"))
    }

    @Test
    fun twelveEight_fourFeltPulses() {
        assertEquals(listOf(3, 3, 3, 3), meterGroups("12/8"))
        assertEquals(listOf(0, 2, 2, 1, 2, 2, 1, 2, 2, 1, 2, 2), barTiers("12/8"), "amber 1, aqua on each dotted-quarter")
    }

    @Test
    fun additive_isLiteralGroups_notReduced() {
        assertEquals(listOf(3, 4), meterGroups("3+4/8"), "additive metres keep the groups the player wrote")
        assertEquals(listOf(0, 2, 2, 1, 2, 2, 2), barTiers("3+4/8"), "amber on 1, aqua where the 4-group starts")
        assertEquals(listOf(3, 2), meterGroups("3+2/4"))
    }

    @Test
    fun oddNumerator_staysAllOnes_notInferredGrouping() {
        // Spec: a plain odd numerator is NOT auto-grouped (5/4 is five ones; 3+2/4 is how you ask for 3+2).
        assertEquals(listOf(1, 1, 1, 1, 1), meterGroups("5/4"))
        assertEquals(listOf(1, 1, 1, 1, 1, 1, 1), meterGroups("7/8"))
    }

    @Test
    fun parser_isLenient_malformedFallsBackTo4_4() {
        for (bad in listOf("", "  ", "4", "4/", "/4", "4/4/4", "x/4", "4/x", "4/3", "0/4", "40/4", "3+/8", "3++4/8", "33+1/8", "20+20+20+20/8")) {
            assertEquals(DEFAULT_GROUPS, meterGroups(bad), "malformed \"$bad\" must fall back to 4/4, never throw")
        }
        assertEquals(DEFAULT_GROUPS, meterGroups(null))
    }

    @Test
    fun unitInterval_uniformDividesThePulse_irregularCountsUnits() {
        // 4/4 @120: a quarter is the unit (500 ms). 6/8 @120: the eighth is the unit (500/3).
        assertEquals(500.0, unitIntervalMs(120, meterGroups("4/4")), 1e-9)
        assertEquals(500.0 / 3.0, unitIntervalMs(120, meterGroups("6/8")), 1e-9)
        // Irregular groups have no single pulse length → tempo counts UNITS directly (60000/bpm).
        assertEquals(500.0, unitIntervalMs(120, meterGroups("3+4/8")), 1e-9)
    }

    @Test
    fun tempoUnit_namesTheBeatNote() {
        assertEquals(TempoUnit.QUARTER, tempoUnit(meterGroups("4/4")))
        assertEquals(TempoUnit.QUARTER, tempoUnit(meterGroups("3/4")))
        assertEquals(TempoUnit.DOTTED_QUARTER, tempoUnit(meterGroups("6/8")))
        assertEquals(TempoUnit.DOTTED_QUARTER, tempoUnit(meterGroups("12/8")))
        assertEquals(TempoUnit.EIGHTH, tempoUnit(meterGroups("3+4/8")))
    }

    @Test
    fun tier2_mutedBelow_litAbove_the130msThreshold() {
        // 6/8 unit 1 is a free subdivision (tier 2). At its onset it lights ABOVE 130 ms/unit and is
        // muted BELOW — both sides asserted, so neither the strobe-floor nor its inverse can regress.
        val g = meterGroups("6/8")
        val below = beatPhase(129.0, 129.0, 12, g) // unit 1, interval 129 ms
        assertEquals(2, below.tier)
        assertFalse(below.lit, "a tier-2 unit is muted below the 130 ms strobe floor")
        val above = beatPhase(131.0, 131.0, 12, g) // unit 1, interval 131 ms
        assertEquals(2, above.tier)
        assertTrue(above.lit, "a tier-2 unit lights above the 130 ms floor")
        // The bar and felt pulses always light, even below the floor.
        assertTrue(beatPhase(0.0, 100.0, 12, g).lit, "the bar (tier 0) is never muted")
        assertTrue(beatPhase(300.0, 100.0, 12, g).lit, "a felt pulse (tier 1) is never muted")
    }
}

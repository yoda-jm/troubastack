package com.troubastack.shared.stage

import com.troubastack.shared.stage.SchemeCycleDirection.DOWN
import com.troubastack.shared.stage.SchemeCycleDirection.UP
import com.troubastack.shared.stage.StageColorMode.AMBER
import com.troubastack.shared.stage.StageColorMode.NIGHT
import com.troubastack.shared.stage.StageColorMode.NORMAL
import com.troubastack.shared.stage.StageColorMode.WARM
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * A10 persistence + A37 ping-pong cycle (pure; the ColorFilter is draw-time). The cycle is the safety
 * feature: a mistimed on-stage tap must never step from a dark scheme (Night/Amber) straight to a
 * full-white Normal in a pit blackout. Every one of Fable's Ruling-1b rules is a row here.
 */
class StageColorModeTest {

    /** Each scheme in each direction — the exhaustive step table (Ruling 1b table-test). */
    @Test
    fun step_table_eachSchemeEachDirection() {
        // UP walks toward the darkest (Amber); endpoints flip and step (never a no-op, Ruling 3).
        assertEquals(SchemeStep(WARM, UP), stageSchemeStep(NORMAL, UP))
        assertEquals(SchemeStep(NIGHT, UP), stageSchemeStep(WARM, UP))
        assertEquals(SchemeStep(AMBER, UP), stageSchemeStep(NIGHT, UP))
        assertEquals(SchemeStep(NIGHT, DOWN), stageSchemeStep(AMBER, UP)) // top endpoint flips
        assertEquals(SchemeStep(NIGHT, DOWN), stageSchemeStep(AMBER, DOWN))
        assertEquals(SchemeStep(WARM, DOWN), stageSchemeStep(NIGHT, DOWN))
        assertEquals(SchemeStep(NORMAL, DOWN), stageSchemeStep(WARM, DOWN))
        assertEquals(SchemeStep(WARM, UP), stageSchemeStep(NORMAL, DOWN)) // bottom endpoint flips
    }

    /** THE safety invariant: from a dark scheme, no direction ever steps to Normal. */
    @Test
    fun step_neverDarkToWhite() {
        for (dark in listOf(NIGHT, AMBER)) {
            for (dir in SchemeCycleDirection.entries) {
                assertTrue(
                    stageSchemeStep(dark, dir).mode != NORMAL,
                    "$dark stepping $dir must NOT jump to NORMAL (pit-blackout flood)",
                )
            }
        }
    }

    /** A full walk from a cold start ping-pongs and, across many taps, never lands dark→white. */
    @Test
    fun walk_pingPongs_andNeverFloods() {
        var mode = NORMAL
        var dir = SchemeCycleDirection.INITIAL
        val seen = mutableListOf(mode)
        repeat(8) {
            val prev = mode
            val step = stageSchemeStep(mode, dir); mode = step.mode; dir = step.direction
            if (prev.isDark) assertTrue(mode != NORMAL, "never dark→white mid-walk")
            seen += mode
        }
        // Normal→Warm→Night→Amber→Night→Warm→Normal→Warm→Night
        assertEquals(listOf(NORMAL, WARM, NIGHT, AMBER, NIGHT, WARM, NORMAL, WARM, NIGHT), seen)
    }

    /** Direct selection (Parameters) resets the direction to UP — a fresh walk, not a continuation. */
    @Test
    fun directSelect_resetsDirectionToUp() {
        for (m in StageColorMode.entries) {
            assertEquals(SchemeStep(m, UP), stageSchemeSelect(m))
        }
        // Fable's scenario: Amber, walk down to Night, pick Warm in Parameters → next tap goes to NIGHT.
        val picked = stageSchemeSelect(WARM)
        assertEquals(WARM, picked.mode)
        assertEquals(SchemeStep(NIGHT, UP), stageSchemeStep(picked.mode, picked.direction))
    }

    @Test
    fun coldStartDirection_isUp() {
        assertEquals(UP, SchemeCycleDirection.INITIAL)
    }

    @Test
    fun persistenceRoundTrip() {
        for (m in StageColorMode.entries) {
            assertEquals(m, StageColorMode.parse(m.name))
        }
    }

    @Test
    fun parseUnknownOrNull_defaultsNormal() {
        assertEquals(NORMAL, StageColorMode.parse(null))
        assertEquals(NORMAL, StageColorMode.parse(""))
        assertEquals(NORMAL, StageColorMode.parse("sepia"))
        assertEquals(NORMAL, StageColorMode.parse("DAY")) // an old A10-era value that never existed
    }

    @Test
    fun labels_areStableAndDistinct() {
        val labels = StageColorMode.entries.map { it.label() }
        assertEquals(listOf("Normal", "Warm", "Night", "Amber"), labels)
    }
}
